package sonobuoy_plugin

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
	capi "sigs.k8s.io/cluster-api/api/v1alpha4"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/giantswarm/sonobuoy-plugin/v5/pkg/ctrlclient"
)

const (
	helloWorldAppName  = "loadtest-app"
	NginxIngressValues = `baseDomain: %s
controller:
  admissionWebhooks:
    enabled: false
`
	HelloWorldValues = `
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

// Test_Ingress creates a Pod in the tenant cluster namespace in the MC cluster
// that tries to send an HTTP request to a "hello world" app
// (https://github.com/giantswarm/loadtest-app) running in the tenant cluster.
// The app is installed in the WC together with the nginx ingress controller
// app (https://github.com/giantswarm/nginx-ingress-controller-app), so that it
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

	cpKubeConfig, err := ctrlclient.GetCPKubeConfig()
	if err != nil {
		t.Fatal(err)
	}

	tcKubeConfig, err := ctrlclient.GetTCKubeConfig()
	if err != nil {
		t.Fatal(err)
	}

	logger.Debugf(ctx, "Testing that we can send http requests to a deployed app exposed via Ingress")

	clusterList := &capi.ClusterList{}
	err = cpCtrlClient.List(ctx, clusterList, client.MatchingLabels{capi.ClusterLabelName: clusterID})
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
			Version:            "0.4.2",
			WaitForDeploy:      true,
		},
	}
	err = appTest.InstallApps(ctx, apps)
	if err != nil {
		t.Fatal(err)
	}

	pod, cm, err := createPodThatSendsHttpRequestToEndpoint(ctx, cpCtrlClient, clusterID, appEndpoint)
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		_ = cpCtrlClient.Delete(ctx, pod)
		_ = cpCtrlClient.Delete(ctx, cm)
		_ = appTest.CleanUp(ctx, apps)
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

func createPodThatSendsHttpRequestToEndpoint(ctx context.Context, ctrlClient client.Client, namespace, httpEndpoint string) (*corev1.Pod, *corev1.ConfigMap, error) {
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
		return nil, nil, err
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
					Image:   "busybox",
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
		return nil, nil, err
	}

	return pod, cm, nil
}
