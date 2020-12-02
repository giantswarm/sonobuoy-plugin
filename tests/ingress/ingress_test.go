package ingress

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/Azure/go-autorest/autorest/to"
	"github.com/giantswarm/apptest"
	"github.com/giantswarm/backoff"
	"github.com/giantswarm/microerror"
	"github.com/giantswarm/micrologger"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/api/networking/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
	capzv1alpha3 "sigs.k8s.io/cluster-api-provider-azure/api/v1alpha3"
	expcapzv1alpha3 "sigs.k8s.io/cluster-api-provider-azure/exp/api/v1alpha3"
	capiv1alpha3 "sigs.k8s.io/cluster-api/api/v1alpha3"
	expcapiv1alpha3 "sigs.k8s.io/cluster-api/exp/api/v1alpha3"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/giantswarm/azure-sonobuoy/v5/tests/ctrlclient"
)

const (
	helloWorldAppName = "hello-world-app"
	helloWorldAppPort = 80
)

const Values = `resource:
  default:
    name: '%s'
`

func Test_CPTCConnectivity(t *testing.T) {
	var err error

	ctx := context.Background()

	cpCtrlClient, err := ctrlclient.CreateCPCtrlClient(ctx)
	if err != nil {
		t.Fatalf("error creating CP k8s client: %v", err)
	}

	tcCtrlClient, err := ctrlclient.CreateTCCtrlClient(ctx)
	if err != nil {
		t.Fatalf("error creating TC k8s client: %v", err)
	}

	logger, err := micrologger.New(micrologger.Config{})
	if err != nil {
		t.Fatal(err)
	}

	clusterID, exists := os.LookupEnv("CLUSTER_ID")
	if !exists {
		t.Fatal("missing CLUSTER_ID environment variable")
	}

	cpKubeConfig, err := ctrlclient.GetCPKubeConfig(ctx)
	if err != nil {
		t.Fatal(err)
	}

	var appTest apptest.Interface
	{
		runtimeScheme := runtime.NewScheme()
		appSchemeBuilder := runtime.SchemeBuilder{
			capiv1alpha3.AddToScheme,
			capzv1alpha3.AddToScheme,
			expcapiv1alpha3.AddToScheme,
			expcapzv1alpha3.AddToScheme,
			corev1.AddToScheme,
		}
		err := appSchemeBuilder.AddToScheme(runtimeScheme)
		if err != nil {
			t.Fatal(err)
		}
		appTest, err = apptest.New(apptest.Config{
			KubeConfig: string(cpKubeConfig),
			Logger:     logger,
			Scheme:     runtimeScheme,
		})
		if err != nil {
			t.Fatal(err)
		}
	}

	apps := []apptest.App{
		{
			CatalogName:   "control-plane-catalog",
			Name:          "nginx-ingress-controller-app",
			Namespace:     clusterID,
			Version:       "1.11.0",
			WaitForDeploy: true,
		},
		{
			CatalogName:   "giantswarm-playground-catalog",
			Name:          helloWorldAppName,
			Namespace:     clusterID,
			ValuesYAML:    fmt.Sprintf(Values, helloWorldAppName),
			Version:       "0.0.1",
			WaitForDeploy: true,
		},
	}
	err = appTest.InstallApps(ctx, apps)
	if err != nil {
		t.Fatal(err)
	}

	httpEndpoint, err := exposeAppUsingIngress(ctx, cpCtrlClient, tcCtrlClient, clusterID)
	if err != nil {
		t.Fatal(err)
	}

	pod, err := createPodThatSendsHttpRequestToEndpoint(ctx, cpCtrlClient, httpEndpoint)
	if err != nil {
		t.Fatal(err)
	}

	o := func() error {
		objectKey, err := client.ObjectKeyFromObject(pod)
		if err != nil {
			return microerror.Mask(err)
		}
		scheduledPod := &corev1.Pod{}
		err = tcCtrlClient.Get(ctx, objectKey, scheduledPod)
		if err != nil {
			return microerror.Mask(err)
		}

		if scheduledPod.Status.Phase != corev1.PodSucceeded {
			return microerror.Maskf(executionFailedError, "container didn't finish yet, pod state is %#q", scheduledPod.Status.Phase)
		}

		if scheduledPod.Status.ContainerStatuses[0].State.Terminated.ExitCode != 0 {
			return microerror.Maskf(executionFailedError, "expected container exit code is 0, got %d", scheduledPod.Status.ContainerStatuses[0].LastTerminationState.Terminated.ExitCode)
		}

		return nil
	}
	b := backoff.NewExponential(backoff.ShortMaxWait, backoff.ShortMaxInterval)
	n := backoff.NewNotifier(logger, ctx)
	err = backoff.RetryNotify(o, b, n)
	if err != nil {
		_ = tcCtrlClient.Delete(ctx, pod)
		t.Fatalf("couldn't get successful HTTP response from hello world app: %v", err)
	}
	_ = tcCtrlClient.Delete(ctx, pod)
}

func createPodThatSendsHttpRequestToEndpoint(ctx context.Context, ctrlClient client.Client, httpEndpoint string) (*corev1.Pod, error) {
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "e2e",
			Namespace: metav1.NamespaceDefault,
		},
		Spec: corev1.PodSpec{
			RestartPolicy: corev1.RestartPolicyNever,
			Containers:    []corev1.Container{{Name: "connectivity", Image: "busybox", Command: []string{"wget"}, Args: []string{httpEndpoint, "--timeout", "5", "-O", "-"}}},
		},
	}
	err := ctrlClient.Create(ctx, pod)
	if err != nil {
		return nil, err
	}

	return pod, nil
}

func exposeAppUsingIngress(ctx context.Context, cpCtrlClient, tcCtrlClient client.Client, clusterID string) (string, error) {
	clusterList := &capiv1alpha3.ClusterList{}
	err := cpCtrlClient.List(ctx, clusterList, client.MatchingLabels{capiv1alpha3.ClusterLabelName: clusterID})
	if err != nil {
		return "", err
	}

	host := strings.Replace(clusterList.Items[0].Spec.ControlPlaneEndpoint.Host, "api", helloWorldAppName, 1)

	pathTypePrefix := v1beta1.PathTypePrefix
	ingressRule := &v1beta1.Ingress{
		ObjectMeta: metav1.ObjectMeta{},
		Spec: v1beta1.IngressSpec{
			IngressClassName: to.StringPtr("nginx"),
			Rules: []v1beta1.IngressRule{
				{
					Host: host,
					IngressRuleValue: v1beta1.IngressRuleValue{
						HTTP: &v1beta1.HTTPIngressRuleValue{
							Paths: []v1beta1.HTTPIngressPath{
								{
									Path:     "/",
									PathType: &pathTypePrefix,
									Backend: v1beta1.IngressBackend{
										ServiceName: fmt.Sprintf("%s-service", helloWorldAppName),
										ServicePort: intstr.IntOrString{
											Type:   0,
											IntVal: helloWorldAppPort,
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}
	err = tcCtrlClient.Create(ctx, ingressRule)
	if err != nil {
		return "", err
	}

	return host, nil
}
