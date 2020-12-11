package ingress

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/giantswarm/apptest"
	"github.com/giantswarm/backoff"
	"github.com/giantswarm/microerror"
	"github.com/giantswarm/micrologger"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	capiv1alpha3 "sigs.k8s.io/cluster-api/api/v1alpha3"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/giantswarm/sonobuoy-plugin/v5/tests/ctrlclient"
)

const (
	helloWorldAppName  = "loadtest-app"
	NginxIngressValues = "baseDomain: %s"
	HelloWorldValues   = `
replicaCount: 1
ingress:
  enabled: true
  annotations:
    kubernetes.io/ingress.class: nginx
  paths:
    - "/"
  hosts:
    - %s
`
)

func Test_Ingress(t *testing.T) {
	var err error

	ctx := context.Background()

	cpCtrlClient, err := ctrlclient.CreateCPCtrlClient(ctx)
	if err != nil {
		t.Fatalf("error creating CP k8s client: %v", err)
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

	tcKubeConfig, err := ctrlclient.GetTCKubeConfig(ctx)
	if err != nil {
		t.Fatal(err)
	}

	clusterList := &capiv1alpha3.ClusterList{}
	err = cpCtrlClient.List(ctx, clusterList, client.MatchingLabels{capiv1alpha3.ClusterLabelName: clusterID})
	if err != nil {
		t.Fatal(err)
	}

	baseDomain := strings.TrimPrefix(clusterList.Items[0].Spec.ControlPlaneEndpoint.Host, "api.")
	appEndpoint := fmt.Sprintf("%s.%s", helloWorldAppName, baseDomain)

	var appTest apptest.Interface
	{
		appTest, err = apptest.New(apptest.Config{
			KubeConfig: string(cpKubeConfig),
			Logger:     logger,
			Scheme:     ctrlclient.Scheme,
		})
		if err != nil {
			t.Fatal(err)
		}
	}

	apps := []apptest.App{
		{
			AppCRNamespace:     clusterID,
			AppOperatorVersion: "1.0.0",
			CatalogName:        "giantswarm",
			KubeConfig:         string(tcKubeConfig),
			Name:               "nginx-ingress-controller-app",
			Namespace:          "kube-system",
			ValuesYAML:         fmt.Sprintf(NginxIngressValues, baseDomain),
			Version:            "1.11.0",
			WaitForDeploy:      true,
		},
		{
			AppCRNamespace:     clusterID,
			AppOperatorVersion: "1.0.0",
			CatalogName:        "default",
			KubeConfig:         string(tcKubeConfig),
			Name:               helloWorldAppName,
			Namespace:          "default",
			ValuesYAML:         fmt.Sprintf(HelloWorldValues, appEndpoint),
			Version:            "0.3.0",
			WaitForDeploy:      true,
		},
	}
	err = appTest.InstallApps(ctx, apps)
	if err != nil {
		t.Fatal(err)
	}

	pod, err := createPodThatSendsHttpRequestToEndpoint(ctx, cpCtrlClient, clusterID, appEndpoint)
	if err != nil {
		t.Fatal(err)
	}

	o := func() error {
		objectKey, err := client.ObjectKeyFromObject(pod)
		if err != nil {
			return microerror.Mask(err)
		}
		scheduledPod := &corev1.Pod{}
		err = cpCtrlClient.Get(ctx, objectKey, scheduledPod)
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
		_ = cpCtrlClient.Delete(ctx, pod)
		_ = appTest.CleanUp(ctx, apps)
		t.Fatalf("couldn't get successful HTTP response from hello world app: %v", err)
	}
	_ = cpCtrlClient.Delete(ctx, pod)
	_ = appTest.CleanUp(ctx, apps)
}

func createPodThatSendsHttpRequestToEndpoint(ctx context.Context, ctrlClient client.Client, namespace, httpEndpoint string) (*corev1.Pod, error) {
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "e2e-ingress",
			Namespace: namespace,
		},
		Spec: corev1.PodSpec{
			RestartPolicy: corev1.RestartPolicyNever,
			Containers:    []corev1.Container{{Name: "test", Image: "busybox", Command: []string{"wget"}, Args: []string{httpEndpoint, "--timeout", "5", "-O", "-"}}},
		},
	}
	err := ctrlClient.Create(ctx, pod)
	if err != nil {
		return nil, err
	}

	return pod, nil
}
