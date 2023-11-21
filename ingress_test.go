package sonobuoy_plugin

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"

	appv1alpha1 "github.com/giantswarm/apiextensions-application/api/v1alpha1"
	"github.com/giantswarm/backoff"
	"github.com/giantswarm/microerror"
	"github.com/giantswarm/micrologger"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/kyverno/kyverno/api/kyverno/v2alpha1"
	"github.com/kyverno/kyverno/api/kyverno/v2beta1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	capiv1alpha3 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/giantswarm/sonobuoy-plugin/v5/pkg/apputil"
	"github.com/giantswarm/sonobuoy-plugin/v5/pkg/ctrlclient"
)

const (
	helloWorldAppName  = "loadtest-app"
	IngressNginxValues = `baseDomain: %s
controller:
  admissionWebhooks:
    enabled: false
`
	HelloWorldValues = `
replicaCount: 1
ingress:
  enabled: true
  paths:
    - "/"
  hosts:
    - %s
`
)

// Test_Ingress creates a Pod in the tenant cluster namespace in the MC cluster
// that tries to send an HTTP request to a "hello world" app
// (https://github.com/giantswarm/loadtest-app) running in the tenant cluster.
// The app is installed in the WC together with the Ingress NGINX Controller
// app (https://github.com/giantswarm/ingress-nginx-app), so that it
// can receive traffic from outside the cluster.
func Test_Ingress(t *testing.T) {
	t.Parallel()

	var err error

	ctx := context.Background()

	cpCtrlClient, err := ctrlclient.CreateCPCtrlClient()
	if err != nil {
		t.Fatalf("error creating CP k8s client: %v", err)
	}

	regularLogger, err := micrologger.New(micrologger.Config{})
	if err != nil {
		t.Fatal(err)
	}

	logger := NewTestLogger(regularLogger, t)

	clusterID, exists := os.LookupEnv("CLUSTER_ID")
	if !exists {
		t.Fatal("missing CLUSTER_ID environment variable")
	}

	logger.Debugf(ctx, "Testing that we can send http requests to a deployed app exposed via Ingress")

	clusterList := &capiv1alpha3.ClusterList{}
	err = cpCtrlClient.List(ctx, clusterList, client.MatchingLabels{capiv1alpha3.ClusterNameLabel: clusterID})
	if err != nil {
		t.Fatal(err)
	}

	baseDomain := strings.TrimPrefix(clusterList.Items[0].Spec.ControlPlaneEndpoint.Host, "api.")
	appEndpoint := fmt.Sprintf("%s.%s", helloWorldAppName, baseDomain)

	// install apps
	var ingress *appv1alpha1.App
	var ingressConfig *corev1.ConfigMap
	{
		ingressAppConfig := apputil.AppConfig{Name: "ingress-nginx", Namespace: "kube-system", Catalog: "giantswarm", ValuesYAML: fmt.Sprintf(IngressNginxValues, baseDomain)}

		ingress, err = apputil.GetApp(clusterID, ingressAppConfig)
		if err != nil {
			t.Fatal(err)
		}

		ingressConfig, err = apputil.CreateAppConfigCM(ctx, logger, cpCtrlClient, clusterID, ingressAppConfig)
		if err != nil {
			t.Fatal(err)
		}

		err = apputil.InstallAndWait(ctx, logger, cpCtrlClient, ingress)
		if err != nil {
			t.Fatal(err)
		}
	}

	var helloworld *appv1alpha1.App
	var helloworldConfig *corev1.ConfigMap
	{
		helloworldAppCfg := apputil.AppConfig{Name: helloWorldAppName, Namespace: "default", Catalog: "default", ValuesYAML: fmt.Sprintf(HelloWorldValues, appEndpoint)}

		helloworld, err = apputil.GetApp(clusterID, helloworldAppCfg)
		if err != nil {
			t.Fatal(err)
		}

		helloworldConfig, err = apputil.CreateAppConfigCM(ctx, logger, cpCtrlClient, clusterID, helloworldAppCfg)
		if err != nil {
			t.Fatal(err)
		}

		err = apputil.InstallAndWait(ctx, logger, cpCtrlClient, helloworld)
		if err != nil {
			t.Fatal(err)
		}
	}

	pod, cm, polex, err := createPodThatSendsHttpRequestToEndpoint(ctx, cpCtrlClient, clusterID, appEndpoint)
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		_ = cpCtrlClient.Delete(ctx, pod)
		_ = cpCtrlClient.Delete(ctx, cm)
		_ = cpCtrlClient.Delete(ctx, polex)
		_ = cpCtrlClient.Delete(ctx, ingress)
		_ = cpCtrlClient.Delete(ctx, ingressConfig)
		_ = cpCtrlClient.Delete(ctx, helloworld)
		_ = cpCtrlClient.Delete(ctx, helloworldConfig)
	})

	o := func() error {
		objectKey := client.ObjectKeyFromObject(pod)
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
		t.Fatalf("couldn't get successful HTTP response from hello world app: %v", err)
	}
}

func createPodThatSendsHttpRequestToEndpoint(ctx context.Context, ctrlClient client.Client, namespace, httpEndpoint string) (*corev1.Pod, *corev1.ConfigMap, *v2alpha1.PolicyException, error) {
	script := `
#!/bin/sh

attempts=5
while [ $attempts -gt 0 ]
do
	if wget --timeout 5 -O- %s
	then
		echo "Success"
		exit 0
    fi
    attempts=$((attempts-1))
	sleep 5
done
`
	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "e2e-ingress",
			Namespace: namespace,
		},
		Data: map[string]string{"script.sh": fmt.Sprintf(script, httpEndpoint)},
	}

	err := ctrlClient.Create(ctx, cm)
	if err != nil {
		return nil, nil, nil, err
	}

	polex := &v2alpha1.PolicyException{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "ingress-test",
			Namespace: "giantswarm",
		},
		Spec: v2alpha1.PolicyExceptionSpec{
			Match: v2beta1.MatchResources{
				Any: kyvernov1.ResourceFilters{
					{
						ResourceDescription: kyvernov1.ResourceDescription{
							Kinds:      []string{"Pod"},
							Names:      []string{"e2e-ingress"},
							Namespaces: []string{namespace},
						},
					},
				},
			},
			Exceptions: []v2alpha1.Exception{
				{
					PolicyName: "disallow-capabilities-strict",
					RuleNames:  []string{"require-drop-all"},
				},
				{
					PolicyName: "disallow-privilege-escalation",
					RuleNames:  []string{"privilege-escalation"},
				},
				{
					PolicyName: "require-run-as-nonroot",
					RuleNames:  []string{"run-as-non-root"},
				},
				{
					PolicyName: "restrict-seccomp-strict",
					RuleNames:  []string{"check-seccomp-strict"},
				},
			},
		},
	}

	err = ctrlClient.Create(ctx, polex)
	if err != nil {
		return nil, nil, nil, err
	}

	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "e2e-ingress",
			Namespace: namespace,
		},
		Spec: corev1.PodSpec{
			RestartPolicy: corev1.RestartPolicyNever,
			Containers: []corev1.Container{
				{
					Name:    "test",
					Image:   "quay.io/giantswarm/busybox:1.34.1",
					Command: []string{"/bin/sh"},
					Args:    []string{"/script.sh"},
					VolumeMounts: []corev1.VolumeMount{
						{
							Name:      "script",
							MountPath: "/script.sh",
							SubPath:   "script.sh",
						},
					},
				},
			},
			Volumes: []corev1.Volume{
				{
					Name: "script",
					VolumeSource: corev1.VolumeSource{
						ConfigMap: &corev1.ConfigMapVolumeSource{
							LocalObjectReference: corev1.LocalObjectReference{
								Name: "e2e-ingress",
							},
						},
					},
				},
			},
		},
	}
	err = ctrlClient.Create(ctx, pod)
	if err != nil {
		return nil, nil, nil, err
	}

	return pod, cm, polex, nil
}
