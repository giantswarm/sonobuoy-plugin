package ingress

import (
	"context"
	"testing"
	"time"

	"github.com/Azure/go-autorest/autorest/to"
	"github.com/giantswarm/backoff"
	"github.com/giantswarm/microerror"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/giantswarm/micrologger"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/giantswarm/sonobuoy-plugin/v5/pkg/ctrlclient"
)

const (
	helloWorldNamespace      = "default"
	helloWorldDeploymentName = "helloworld"
)

func Test_Autoscaler(t *testing.T) {
	var err error

	ctx := context.Background()

	tcCtrlClient, err := ctrlclient.CreateTCCtrlClient(ctx)
	if err != nil {
		t.Fatalf("error creating TC k8s client: %v", err)
	}

	logger, err := micrologger.New(micrologger.Config{})
	if err != nil {
		t.Fatal(err)
	}

	logger.Debugf(ctx, "Creating %s deployment", helloWorldDeploymentName)

	deployment, err := createDeployment(ctx, tcCtrlClient, 1)
	if err != nil {
		t.Fatal(err)
	}

	// Get number of worker nodes.
	workersCount, err := getWorkersCount(ctx, tcCtrlClient)
	if err != nil {
		cleanupAndFatal(ctx, deployment, tcCtrlClient, t, "%v", err)
	}

	logger.Debugf(ctx, "Found %d worker nodes", workersCount)

	// Scale helloworld deployment to len(workers) + 1 replicas to trigger a scale up.
	expectedWorkersCount := int32(workersCount + 1)
	logger.Debugf(ctx, "Scaling deployment %s/%s to %d replicas", helloWorldNamespace, helloWorldDeploymentName, expectedWorkersCount)
	err = scaleDeployment(ctx, tcCtrlClient, expectedWorkersCount)
	if err != nil {
		cleanupAndFatal(ctx, deployment, tcCtrlClient, t, "%v", err)
	}

	logger.Debugf(ctx, "Waiting for %d worker nodes to exist", expectedWorkersCount)

	// Wait for nodes to increase by one.
	o := func() error {
		workersCount, err := getWorkersCount(ctx, tcCtrlClient)
		if err != nil {
			return microerror.Mask(err)
		}

		if int32(workersCount) != expectedWorkersCount {
			return microerror.Maskf(executionFailedError, "Expecting %d workers, %d found", expectedWorkersCount, workersCount)
		}

		return nil
	}
	b := backoff.NewConstant(backoff.LongMaxWait, 10*time.Second)
	n := backoff.NewNotifier(logger, ctx)
	err = backoff.RetryNotify(o, b, n)
	if err != nil {
		cleanupAndFatal(ctx, deployment, tcCtrlClient, t, "timeout waiting for cluster to scale up: %v", err)
	}

	// Scale down deployment, wait for one node to get deleted.
	expectedWorkersCount = expectedWorkersCount - 1
	logger.Debugf(ctx, "Scaling deployment %s/%s to %d replicas", helloWorldNamespace, helloWorldDeploymentName, expectedWorkersCount)
	err = scaleDeployment(ctx, tcCtrlClient, expectedWorkersCount)
	if err != nil {
		cleanupAndFatal(ctx, deployment, tcCtrlClient, t, "timeout waiting for cluster to scale down: %v", err)
	}

	err = backoff.RetryNotify(o, b, n)
	if err != nil {
		cleanupAndFatal(ctx, deployment, tcCtrlClient, t, "timeout waiting for cluster to scale down: %v", err)
	}

	tcCtrlClient.Delete(ctx, deployment)
}

func cleanupAndFatal(ctx context.Context, deployment *appsv1.Deployment, ctrlClient client.Client, t *testing.T, msg string, args ...interface{}) {
	ctrlClient.Delete(ctx, deployment)
	t.Fatalf(msg, args...)
}

func getWorkersCount(ctx context.Context, ctrlClient client.Client) (int, error) {
	workers := &corev1.NodeList{}
	err := ctrlClient.List(ctx, workers, client.MatchingLabels{"kubernetes.io/role": "worker"})
	if err != nil {
		return -1, err
	}

	return len(workers.Items), nil
}

func scaleDeployment(ctx context.Context, ctrlClient client.Client, expectedWorkersCount int32) error {
	deployment := &appsv1.Deployment{}
	err := ctrlClient.Get(ctx, client.ObjectKey{Namespace: helloWorldNamespace, Name: helloWorldDeploymentName}, deployment)
	if err != nil {
		return microerror.Mask(err)
	}

	deployment.Spec.Replicas = &expectedWorkersCount

	err = ctrlClient.Update(ctx, deployment)
	if err != nil {
		return microerror.Mask(err)
	}

	return nil
}

func createDeployment(ctx context.Context, ctrlClient client.Client, replicas int32) (*appsv1.Deployment, error) {
	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      helloWorldDeploymentName,
			Namespace: helloWorldNamespace,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app": helloWorldDeploymentName,
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app": helloWorldDeploymentName,
					},
				},
				Spec: corev1.PodSpec{
					Affinity: &corev1.Affinity{
						PodAntiAffinity: &corev1.PodAntiAffinity{
							RequiredDuringSchedulingIgnoredDuringExecution: []corev1.PodAffinityTerm{
								{
									LabelSelector: &metav1.LabelSelector{
										MatchExpressions: []metav1.LabelSelectorRequirement{
											{
												Key:      "app",
												Operator: "In",
												Values:   []string{helloWorldDeploymentName},
											},
										},
									},
									TopologyKey: "kubernetes.io/hostname",
								},
							},
						},
					},
					SecurityContext: &corev1.PodSecurityContext{
						RunAsUser: to.Int64Ptr(1000),
					},
					Containers: []corev1.Container{
						{
							Name:  helloWorldDeploymentName,
							Image: "quay.io/giantswarm/helloworld:latest",
						},
					},
				},
			},
		},
	}
	err := ctrlClient.Create(ctx, deployment)
	if err != nil {
		return nil, err
	}

	return deployment, nil
}
