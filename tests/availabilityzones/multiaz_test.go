package availabilityzones

import (
	"context"
	"errors"
	"os"
	"reflect"
	"sort"
	"testing"

	"github.com/giantswarm/backoff"
	"github.com/giantswarm/microerror"
	"github.com/giantswarm/micrologger/microloggertest"
	capi "sigs.k8s.io/cluster-api/api/v1alpha3"
	ctrl "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/giantswarm/sonobuoy-plugin/v5/tests/ctrlclient"
)

const (
	Provider = "E2E_PROVIDER"
)

func Test_AvailabilityZones(t *testing.T) {
	var err error

	ctx := context.Background()

	cpCtrlClient, err := ctrlclient.CreateCPCtrlClient(ctx)
	if err != nil {
		t.Fatalf("error creating CP k8s client: %v", err)
	}

	clusterID, exists := os.LookupEnv("CLUSTER_ID")
	if !exists {
		t.Fatalf("missing CLUSTER_ID environment variable")
	}

	cluster, err := findCluster(ctx, cpCtrlClient, clusterID)
	if err != nil {
		t.Fatalf("error finding cluster: %s", microerror.JSON(err))
	}

	providerSupport, err := GetProviderSupport(ctx, cpCtrlClient, cluster)
	if err != nil {
		t.Fatal(err)
	}

	machinePool, err := providerSupport.CreateNodePool(ctx, cpCtrlClient, cluster, providerSupport.GetProviderAZs())
	if err != nil {
		t.Fatal(err)
	}

	// Wait for Node Pool to come up.
	{
		o := func() error {
			err := cpCtrlClient.Get(ctx, ctrl.ObjectKey{Name: machinePool.Name, Namespace: machinePool.Namespace}, machinePool)
			if err != nil {
				// Wrap masked error with backoff.Permanent() to stop retries on unrecoverable error.
				return backoff.Permanent(microerror.Mask(err))
			}

			// Return error for retry until node pool nodes are Ready.
			if machinePool.Status.ReadyReplicas != *machinePool.Spec.Replicas {
				return errors.New("node pool is not ready yet")
			}

			return nil
		}
		b := backoff.NewConstant(backoff.LongMaxWait, backoff.LongMaxInterval)
		n := backoff.NewNotifier(microloggertest.New(), ctx)
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

func findCluster(ctx context.Context, client ctrl.Client, clusterID string) (*capi.Cluster, error) {
	var cluster *capi.Cluster
	{
		var clusterList capi.ClusterList
		err := client.List(ctx, &clusterList, ctrl.MatchingLabels{capi.ClusterLabelName: clusterID})
		if err != nil {
			return nil, microerror.Mask(err)
		}

		if len(clusterList.Items) > 0 {
			cluster = &clusterList.Items[0]
		} else {
			return nil, microerror.Maskf(invalidConfigError, "can't find cluster with ID %q", clusterID)
		}
	}

	return cluster, nil
}