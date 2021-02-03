package cptcconnectivity

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/giantswarm/backoff"
	"github.com/giantswarm/microerror"
	"github.com/giantswarm/micrologger"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	capiv1alpha3 "sigs.k8s.io/cluster-api/api/v1alpha3"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/giantswarm/sonobuoy-plugin/v5/pkg/ctrlclient"
)

const (
	podName = "e2e-connectivity"
)

func Test_CPTCConnectivity(t *testing.T) {
	var err error

	ctx := context.Background()

	cpCtrlClient, err := ctrlclient.CreateCPCtrlClient()
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

	clusterList := &capiv1alpha3.ClusterList{}
	err = cpCtrlClient.List(ctx, clusterList, client.MatchingLabels{capiv1alpha3.ClusterLabelName: clusterID})
	if err != nil {
		t.Fatalf("error listing Clusters in CP k8s API: %v", err)
	}

	k8sAPIEndpointHost := clusterList.Items[0].Spec.ControlPlaneEndpoint.Host
	k8sAPIEndpointPort := fmt.Sprintf("%d", clusterList.Items[0].Spec.ControlPlaneEndpoint.Port)

	t.Log("testing connectivity between control plane cluster and tenant cluster")
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      podName,
			Namespace: clusterID,
		},
		Spec: corev1.PodSpec{
			RestartPolicy: corev1.RestartPolicyNever,
			Containers:    []corev1.Container{{Name: "test", Image: "busybox", Command: []string{"nc"}, Args: []string{"-z", k8sAPIEndpointHost, k8sAPIEndpointPort}}},
		},
	}
	err = cpCtrlClient.Create(ctx, pod)
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		_ = cpCtrlClient.Delete(ctx, pod)
	})

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
		t.Fatalf("couldn't connect from control plane cluster to tenant cluster: %v", err)
	}
}
