package sonobuoy_plugin

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/giantswarm/backoff"
	"github.com/giantswarm/microerror"
	"github.com/giantswarm/micrologger"
	v1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/giantswarm/sonobuoy-plugin/v5/pkg/capiutil"
	"github.com/giantswarm/sonobuoy-plugin/v5/pkg/ctrlclient"
	"github.com/giantswarm/sonobuoy-plugin/v5/pkg/provider"
)

// Test_CgroupsV1 creates a node pool with cgroups V1 and ensures nodes become ready.
func Test_CgroupsV1(t *testing.T) {
	t.Parallel()

	var err error

	ctx := context.Background()

	regularLogger, err := micrologger.New(micrologger.Config{})
	if err != nil {
		t.Fatal(err)
	}

	logger := NewTestLogger(regularLogger, t)

	tcCtrlClient, err := ctrlclient.CreateTCCtrlClient()
	if err != nil {
		t.Fatalf("error creating TC k8s client: %v", err)
	}

	cpCtrlClient, err := ctrlclient.CreateCPCtrlClient()
	if err != nil {
		t.Fatalf("error creating CP k8s client: %v", err)
	}

	clusterID, exists := os.LookupEnv("CLUSTER_ID")
	if !exists {
		t.Fatalf("missing CLUSTER_ID environment variable")
	}

	cluster, err := capiutil.FindCluster(ctx, cpCtrlClient, clusterID)
	if err != nil {
		t.Fatalf("error finding cluster: %s", microerror.JSON(err))
	}

	providerSupport, err := provider.GetProviderSupport(ctx, logger, cpCtrlClient, cluster)
	if err != nil {
		t.Fatal(err)
	}

	machinePoolObjectKey, err := providerSupport.CreateNodePoolAndWaitReady(ctx, cpCtrlClient, cluster, providerSupport.GetProviderAZs(), true)
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		_ = providerSupport.DeleteNodePool(ctx, cpCtrlClient, *machinePoolObjectKey)
	})

	o := func() error {
		desiredNodes := 3

		nodes := v1.NodeList{}
		err = tcCtrlClient.List(ctx, &nodes, client.MatchingLabels{
			providerSupport.GetNodeSelectorLabel(): machinePoolObjectKey.Name,
		})
		if err != nil {
			t.Fatal(err)
		}

		readyNodes := 0
		for _, node := range nodes.Items {
			for _, condition := range node.Status.Conditions {
				if condition.Type == "Ready" {
					if condition.Status == "True" {
						readyNodes = readyNodes + 1
					}

					break
				}
			}
		}

		if readyNodes != desiredNodes {
			return microerror.Maskf(executionFailedError, "Expected cgroups v1 node pool to have %d ready replicas, but only had %d", desiredNodes, readyNodes)
		}

		return nil
	}

	b := backoff.NewConstant(backoff.LongMaxWait, 1*time.Minute)
	n := backoff.NewNotifier(logger, ctx)
	err = backoff.RetryNotify(o, b, n)
	if err != nil {
		t.Fatal("Node pool with cgroups v1 did not become ready in time.")
	}
}
