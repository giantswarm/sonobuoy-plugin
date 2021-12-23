package sonobuoy_plugin

import (
	"context"
	"os"
	"reflect"
	"sort"
	"testing"

	"github.com/giantswarm/microerror"
	"github.com/giantswarm/micrologger"

	"github.com/giantswarm/sonobuoy-plugin/v5/pkg/capiutil"
	"github.com/giantswarm/sonobuoy-plugin/v5/pkg/ctrlclient"
	"github.com/giantswarm/sonobuoy-plugin/v5/pkg/provider"
)

// Test_AvailabilityZones pulls supported AZs from `provider.Support` implementation, creates a
// node pool with all available AZs and then waits until the created `MachinePool`
// becomes `Ready`. Once the created node pool is ready, the test pulls present
// AZs from created provider node pool instance and compares them to originally
// specified ones.
func Test_AvailabilityZones(t *testing.T) {
	t.Parallel()

	var err error

	ctx := context.Background()

	regularLogger, err := micrologger.New(micrologger.Config{})
	if err != nil {
		t.Fatal(err)
	}

	logger := NewTestLogger(regularLogger, t)

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

	machinePoolObjectKey, err := providerSupport.CreateNodePoolAndWaitReady(ctx, cpCtrlClient, cluster, providerSupport.GetProviderAZs())
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		_ = providerSupport.DeleteNodePool(ctx, cpCtrlClient, *machinePoolObjectKey)
	})

	k8sZones, err := providerSupport.GetNodePoolAZsInCR(ctx, cpCtrlClient, *machinePoolObjectKey)
	if err != nil {
		t.Fatal(err)
	}

	actualZones, err := providerSupport.GetNodePoolAZsInProvider(ctx, cluster.Name, machinePoolObjectKey.Name)
	if err != nil {
		t.Fatal(err)
	}

	sort.Strings(actualZones)
	sort.Strings(k8sZones)

	if !reflect.DeepEqual(actualZones, k8sZones) {
		t.Fatalf("The AZs used are not correct. Expected %s, got %s", k8sZones, actualZones)
	}
}
