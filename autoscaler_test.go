package sonobuoy_plugin

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/Azure/go-autorest/autorest/to"
	"github.com/giantswarm/backoff"
	"github.com/giantswarm/microerror"
	"github.com/giantswarm/micrologger"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/kyverno/kyverno/api/kyverno/v2alpha1"
	"github.com/kyverno/kyverno/api/kyverno/v2beta1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/giantswarm/sonobuoy-plugin/v5/pkg/capiutil"
	"github.com/giantswarm/sonobuoy-plugin/v5/pkg/ctrlclient"
	"github.com/giantswarm/sonobuoy-plugin/v5/pkg/provider"
)

const (
	helloWorldNamespace      = "default"
	helloWorldDeploymentName = "helloworld"
)

// Test_Autoscaler checks the Cluster Autoscaler works by creating a deployment with PodAntiAffinity and scaling it up and down.
func Test_Autoscaler(t *testing.T) {
	t.Parallel()

	var err error

	ctx := context.Background()

	tcCtrlClient, err := ctrlclient.CreateTCCtrlClient()
	if err != nil {
		t.Fatalf("error creating TC k8s client: %v", err)
	}

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

	cluster, err := capiutil.FindCluster(ctx, cpCtrlClient, clusterID)
	if err != nil {
		t.Fatalf("error finding cluster: %s", microerror.JSON(err))
	}

	providerSupport, err := provider.GetProviderSupport(ctx, logger, cpCtrlClient, cluster)
	if err != nil {
		t.Fatal(err)
	}

	machinePoolName, err := providerSupport.GetTestingMachinePoolForCluster(ctx, cpCtrlClient, clusterID)
	if err != nil {
		t.Fatalf("error finding MachinePools for cluster %q: %s", clusterID, microerror.JSON(err))
	}

	logger.Debugf(ctx, "Testing the Cluster Autoscaler with machine pool %s", machinePoolName)
	logger.Debugf(ctx, "Creating %s deployment", helloWorldDeploymentName)

	nodeSelectorLabel := providerSupport.GetNodeSelectorLabel()

	polex, err := createAutoscalerPolex(ctx, tcCtrlClient)
	if err != nil {
		t.Fatal(err)
	}

	deployment, err := createDeployment(ctx, tcCtrlClient, 1, machinePoolName, nodeSelectorLabel)
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		_ = tcCtrlClient.Delete(ctx, deployment)
		_ = tcCtrlClient.Delete(ctx, polex)
	})

	// Get number of worker nodes.
	workersCount, err := getWorkersCount(ctx, tcCtrlClient, machinePoolName, nodeSelectorLabel)
	if err != nil {
		t.Fatalf("%v", err)
	}

	logger.Debugf(ctx, "Found %d worker nodes", workersCount)

	// Scale helloworld deployment to len(workers) + 2 replicas to trigger a scale up.
	expectedWorkersCount := int32(workersCount + 2)
	logger.Debugf(ctx, "Scaling deployment %s/%s to %d replicas", helloWorldNamespace, helloWorldDeploymentName, expectedWorkersCount)
	err = scaleDeployment(ctx, tcCtrlClient, expectedWorkersCount)
	if err != nil {
		t.Fatalf("%v", err)
	}

	logger.Debugf(ctx, "Waiting for %d worker nodes to exist", expectedWorkersCount)

	// Wait for nodes to increase by one.
	o := func() error {
		workersCount, err := getWorkersCount(ctx, tcCtrlClient, machinePoolName, nodeSelectorLabel)
		if err != nil {
			return microerror.Mask(err)
		}

		if int32(workersCount) != expectedWorkersCount {
			return microerror.Maskf(executionFailedError, "Expecting %d workers, %d found", expectedWorkersCount, workersCount)
		}

		return nil
	}
	b := backoff.NewConstant(120*time.Minute, 1*time.Minute)
	n := backoff.NewNotifier(logger, ctx)
	err = backoff.RetryNotify(o, b, n)
	if err != nil {
		t.Fatalf("timeout waiting for cluster to scale up: %v", err)
	}

	// Scale down deployment, wait for one node to get deleted.
	expectedWorkersCount = expectedWorkersCount - 1
	logger.Debugf(ctx, "Scaling deployment %s/%s to %d replicas", helloWorldNamespace, helloWorldDeploymentName, expectedWorkersCount)
	err = scaleDeployment(ctx, tcCtrlClient, expectedWorkersCount)
	if err != nil {
		t.Fatalf("timeout waiting for cluster to scale down: %v", err)
	}

	b = backoff.NewConstant(120*time.Minute, 1*time.Minute)
	err = backoff.RetryNotify(o, b, n)
	if err != nil {
		t.Fatalf("timeout waiting for cluster to scale down: %v", err)
	}
}

func getWorkersCount(ctx context.Context, ctrlClient client.Client, machinePoolName string, labelSelector string) (int, error) {
	workers := &corev1.NodeList{}
	err := ctrlClient.List(ctx, workers, client.MatchingLabels{"kubernetes.io/role": "worker", labelSelector: machinePoolName})
	if err != nil {
		return -1, err
	}

	return len(workers.Items), nil
}

func scaleDeployment(ctx context.Context, ctrlClient client.Client, expectedWorkersCount int32) error {
	o := func() error {
		deployment := &appsv1.Deployment{}
		err := ctrlClient.Get(ctx, client.ObjectKey{Namespace: helloWorldNamespace, Name: helloWorldDeploymentName}, deployment)
		if err != nil {
			return microerror.Mask(err)
		}

		deployment.Spec.Replicas = &expectedWorkersCount

		err = ctrlClient.Update(ctx, deployment)
		if apierrors.IsConflict(err) {
			// Retriable error.
			return microerror.Mask(err)
		} else if err != nil {
			// Wrap masked error with backoff.Permanent() to stop retries on unrecoverable error.
			return backoff.Permanent(microerror.Mask(err))
		}

		return nil
	}

	b := backoff.NewConstant(backoff.ShortMaxWait, backoff.ShortMaxInterval)
	err := backoff.Retry(o, b)
	if err != nil {
		return microerror.Mask(err)
	}

	return nil
}

func createDeployment(ctx context.Context, ctrlClient client.Client, replicas int32, machinePoolName string, nodeSelectorLabel string) (*appsv1.Deployment, error) {
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
					NodeSelector: map[string]string{
						nodeSelectorLabel: machinePoolName,
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

func createAutoscalerPolex(ctx context.Context, ctrlClient client.Client) (*v2alpha1.PolicyException, error) {
	polex := &v2alpha1.PolicyException{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "autoscaler-test",
			Namespace: "giantswarm",
		},
		Spec: v2alpha1.PolicyExceptionSpec{
			Match: v2beta1.MatchResources{
				Any: kyvernov1.ResourceFilters{
					{
						ResourceDescription: kyvernov1.ResourceDescription{
							Kinds:      []string{"Deployment", "Pod"},
							Names:      []string{"helloworld*"},
							Namespaces: []string{"default"},
						},
					},
				},
			},
			Exceptions: []v2alpha1.Exception{
				{
					PolicyName: "disallow-capabilities-strict",
					RuleNames:  []string{"require-drop-all", "autogen-require-drop-all"},
				},
				{
					PolicyName: "disallow-privilege-escalation",
					RuleNames:  []string{"privilege-escalation", "autogen-privilege-escalation"},
				},
				{
					PolicyName: "require-run-as-nonroot",
					RuleNames:  []string{"run-as-non-root", "autogen-run-as-non-root"},
				},
				{
					PolicyName: "restrict-seccomp-strict",
					RuleNames:  []string{"check-seccomp-strict", "autogen-check-seccomp-strict"},
				},
			},
		},
	}
	err := ctrlClient.Create(ctx, polex)
	if err != nil {
		return nil, err
	}

	return polex, nil
}
