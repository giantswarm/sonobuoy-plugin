package sonobuoy_plugin

import (
	"context"
	"errors"
	"os"
	"reflect"
	"sort"
	"testing"

	"github.com/giantswarm/backoff"
	"github.com/giantswarm/microerror"
	"github.com/giantswarm/micrologger"
	capi "sigs.k8s.io/cluster-api/api/v1alpha3"
	capiconditions "sigs.k8s.io/cluster-api/util/conditions"
	ctrl "sigs.k8s.io/controller-runtime/pkg/client"

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

	if provider.GetProvider() != "azure" {
		t.Logf("this test is not implemented on %#q yet, skipping", provider.GetProvider())
		t.SkipNow()
		return
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

	providerSupport, err := provider.GetProviderSupport(ctx, cpCtrlClient, cluster)
	if err != nil {
		t.Fatal(err)
	}

	machinePool, err := providerSupport.CreateNodePool(ctx, cpCtrlClient, cluster, providerSupport.GetProviderAZs())
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		_ = cpCtrlClient.Delete(ctx, machinePool)
	})

	// Wait for Node Pool to come up.
	{
		o := func() error {
			err := cpCtrlClient.Get(ctx, ctrl.ObjectKey{Name: machinePool.Name, Namespace: machinePool.Namespace}, machinePool)
			if err != nil {
				// Wrap masked error with backoff.Permanent() to stop retries on unrecoverable error.
				return backoff.Permanent(microerror.Mask(err))
			}

			// Return error for retry until node pool nodes are Ready.
			if !capiconditions.IsTrue(machinePool, capi.ReadyCondition) {
				return errors.New("node pool is not ready yet")
			}

			return nil
		}
		b := backoff.NewConstant(backoff.LongMaxWait, backoff.LongMaxInterval)
		n := backoff.NewNotifier(logger, ctx)
		err = backoff.RetryNotify(o, b, n)
		if err != nil {
			t.Fatalf("failed to get MachinePool %q for Cluster %q: %s", machinePool.Name, clusterID, microerror.JSON(err))
		}
	}

	expectedZones := machinePool.Spec.FailureDomains
	actualZones, err := providerSupport.GetNodePoolAZs(ctx, cluster.Name, machinePool.Name)
	if err != nil {
		t.Fatal(err)
	}

	sort.Strings(actualZones)
	sort.Strings(expectedZones)

	if !reflect.DeepEqual(actualZones, expectedZones) {
		t.Fatalf("The AZs used are not correct. Expected %s, got %s", expectedZones, actualZones)
	}
}
