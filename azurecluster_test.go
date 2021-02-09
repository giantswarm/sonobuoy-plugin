package sonobuoy_plugin

import (
	"context"
	"os"
	"sort"
	"testing"

	"github.com/giantswarm/apiextensions/v3/pkg/label"
	"github.com/giantswarm/backoff"
	"github.com/giantswarm/microerror"
	"github.com/giantswarm/micrologger"
	capz "sigs.k8s.io/cluster-api-provider-azure/api/v1alpha3"
	capi "sigs.k8s.io/cluster-api/api/v1alpha3"
	capiconditions "sigs.k8s.io/cluster-api/util/conditions"

	"github.com/giantswarm/sonobuoy-plugin/v5/pkg/assert"
	"github.com/giantswarm/sonobuoy-plugin/v5/pkg/capiutil"
	"github.com/giantswarm/sonobuoy-plugin/v5/pkg/ctrlclient"
)

func Test_AzureClusterCR(t *testing.T) {
	t.Parallel()

	var err error
	ctx := context.Background()

	regularLogger, err := micrologger.New(micrologger.Config{})
	if err != nil {
		t.Fatal(err)
	}

	logger := NewTestLogger(regularLogger, t)

	clusterID, exists := os.LookupEnv("CLUSTER_ID")
	if !exists {
		t.Fatal("missing CLUSTER_ID environment variable")
	}

	cpCtrlClient, err := ctrlclient.CreateCPCtrlClient()
	if err != nil {
		t.Fatalf("error creating CP k8s client: %v", err)
	}

	cluster, err := capiutil.FindCluster(ctx, cpCtrlClient, clusterID)
	if err != nil {
		t.Fatalf("error finding cluster: %s", microerror.JSON(err))
	}

	clusterGetter := func(clusterName string) capiutil.TestedObject {
		cluster, err := capiutil.FindCluster(ctx, cpCtrlClient, clusterName)
		if err != nil {
			t.Fatalf("error finding cluster: %s", microerror.JSON(err))
		}

		return cluster
	}

	// Wait for Ready condition to be True
	capiutil.WaitForCondition(t, ctx, logger, cluster, capi.ReadyCondition, capiconditions.IsTrue, clusterGetter)

	azureClusterGetter := func(azureClusterName string) capiutil.TestedObject {
		azureCluster, err := capiutil.FindAzureCluster(ctx, cpCtrlClient, azureClusterName)
		if err != nil {
			t.Fatalf("error finding cluster: %s", microerror.JSON(err))
		}

		return azureCluster
	}

	azureCluster := azureClusterGetter(clusterID).(*capz.AzureCluster)

	//
	// Check Metadata
	//

	// Check if 'release.giantswarm.io/version' label is set
	assert.LabelIsSet(t, cluster, label.ReleaseVersion)

	// Check if 'azure-operator.giantswarm.io/version' label is set
	assert.LabelIsSet(t, cluster, label.AzureOperatorVersion)

	// Wait for Ready condition to be True
	capiutil.WaitForCondition(t, ctx, logger, azureCluster, capi.ReadyCondition, capiconditions.IsTrue, azureClusterGetter)

	// Check that Cluster and AzureCluster desired release version matches
	assert.LabelIsEqual(t, cluster, azureCluster, label.ReleaseVersion)

	// Check that Cluster and AzureCluster azure-operator version matches
	assert.LabelIsEqual(t, cluster, azureCluster, label.AzureOperatorVersion)

	// Assert that AzureCluster owner reference is set to the specified Cluster
	assert.ExpectedOwnerReferenceIsSet(t, azureCluster, cluster)

	//
	// Check Spec
	//

	// Check if we have allocated correct number of CIDR blocks for the VNet
	const expectedVNetCIDRBlocks = 1
	allocatedVNetsCIDRBlocks := len(azureCluster.Spec.NetworkSpec.Vnet.CIDRBlocks)
	if allocatedVNetsCIDRBlocks != expectedVNetCIDRBlocks {
		t.Fatalf("AzureCluster '%s/%s': expected %d VNet CIDR block to be set in Spec.NetworkSpec.Vnet.CIDRBlocks, but found %d instead",
			azureCluster.Namespace,
			azureCluster.Name,
			expectedVNetCIDRBlocks,
			allocatedVNetsCIDRBlocks)
	}

	// Check subnets, first we get MachinePools, as we need one subnet per node pool
	o := func() error {
		azureCluster := azureClusterGetter(clusterID).(*capz.AzureCluster)

		machinePools, err := capiutil.FindAllMachinePoolsForCluster(ctx, cpCtrlClient, clusterID)
		if err != nil {
			return microerror.Mask(err)
		}

		if len(azureCluster.Spec.NetworkSpec.Subnets) != len(machinePools) {
			return microerror.Maskf(
				unexpectedValueError,
				"AzureCluster '%s/%s': expected %d subnets in Spec.NetworkSpec.Subnets (to match number of MachinePools), but got %d instead",
				azureCluster.Namespace,
				azureCluster.Name,
				len(machinePools),
				len(azureCluster.Spec.NetworkSpec.Subnets))
		}

		return nil
	}

	b := backoff.NewExponential(backoff.MediumMaxWait, backoff.LongMaxInterval)
	n := backoff.NewNotifier(logger, ctx)
	err = backoff.RetryNotify(o, b, n)
	if err != nil {
		t.Fatalf("error while waiting for number of subnets in AzureCluster to match number of node pools")
	}

	machinePools, err := capiutil.FindAllMachinePoolsForCluster(ctx, cpCtrlClient, clusterID)
	if err != nil {
		t.Fatalf("error finding MachinePools for cluster %q: %s", clusterID, microerror.JSON(err))
	}

	sort.Slice(machinePools, func(i int, j int) bool {
		return machinePools[i].Name < machinePools[j].Name
	})

	subnets := azureCluster.Spec.NetworkSpec.Subnets

	sort.Slice(subnets, func(i int, j int) bool {
		return subnets[i].Name < subnets[j].Name
	})

	const expectedSubnetCIDRBlocks = 1
	for i := range subnets {
		// Check if subnet name matches MachinePool name
		if subnets[i].Name != machinePools[i].Name {
			t.Fatalf("AzureCluster '%s/%s': expected subnet name %q (in Spec.NetworkSpec.Subnets) to match MachinePool name %q",
				azureCluster.Namespace,
				azureCluster.Name,
				subnets[i].Name,
				machinePools[i].Name)
		}

		// Check if we have allocated correct number of CIDR blocks for the subnet
		allocatedSubnetCIDRBlocks := len(subnets[i].CIDRBlocks)
		if allocatedSubnetCIDRBlocks != expectedSubnetCIDRBlocks {
			t.Fatalf("AzureCluster '%s/%s': expected %d CIDR block to be set in Spec.NetworkSpec.Subnets[%s], but found %d instead",
				azureCluster.Namespace,
				azureCluster.Name,
				expectedSubnetCIDRBlocks,
				subnets[i].Name,
				allocatedSubnetCIDRBlocks)
		}
	}

	//
	// Check Status
	//
	if !azureCluster.Status.Ready {
		t.Fatalf("AzureCluster '%s/%s': expected Status.Ready == true, but got Status.Ready == %t",
			azureCluster.Namespace,
			azureCluster.Name,
			azureCluster.Status.Ready)
	}
}
