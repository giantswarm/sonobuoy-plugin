package customresources

import (
	"context"
	"os"
	"sort"
	"testing"
	"time"

	"github.com/giantswarm/apiextensions/v3/pkg/label"
	"github.com/giantswarm/microerror"
	capz "sigs.k8s.io/cluster-api-provider-azure/api/v1alpha3"
	capi "sigs.k8s.io/cluster-api/api/v1alpha3"
	capiconditions "sigs.k8s.io/cluster-api/util/conditions"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/giantswarm/sonobuoy-plugin/v5/pkg/capiutil"
	"github.com/giantswarm/sonobuoy-plugin/v5/pkg/ctrlclient"
)

func Test_AzureClusterCR(t *testing.T) {
	var err error
	ctx := context.Background()

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

	azureClusterGetter := func() *capz.AzureCluster {
		return getTestedAzureCluster(ctx, t, cpCtrlClient)
	}

	azureCluster := azureClusterGetter()

	//
	// Check Metadata
	//

	// Check if 'release.giantswarm.io/version' label is set
	assertLabelIsSet(t, cluster, label.ReleaseVersion)

	// Check that Cluster and AzureCluster desired release version matches
	assertLabelIsEqual(t, cluster, azureCluster, label.ReleaseVersion)

	// Check if 'azure-operator.giantswarm.io/version' label is set
	assertLabelIsSet(t, cluster, label.AzureOperatorVersion)

	// Check that Cluster and AzureCluster azure-operator version matches
	assertLabelIsEqual(t, cluster, azureCluster, label.AzureOperatorVersion)

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
	machinePools := getTestedMachinePools(ctx, t, cpCtrlClient)
	sort.Slice(machinePools, func(i int, j int) bool {
		return machinePools[i].Name < machinePools[j].Name
	})

	subnets := azureCluster.Spec.NetworkSpec.Subnets

	// Check number of allocated subnets
	if len(subnets) != len(machinePools) {
		t.Fatalf("AzureCluster '%s/%s': expected %d subnets in Spec.NetworkSpec.Subnets (to match number of MachinePools), but got %d instead",
			azureCluster.Namespace,
			azureCluster.Name,
			len(machinePools),
			len(subnets))
	}

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

	// Wait for Ready condition to be True
	waitForAzureClusterCondition(azureCluster, capi.ReadyCondition, capiconditions.IsTrue, azureClusterGetter)

	if !azureCluster.Status.Ready {
		t.Fatalf("AzureCluster '%s/%s': expected Status.Ready == true, but got Status.Ready == %t",
			azureCluster.Namespace,
			azureCluster.Name,
			azureCluster.Status.Ready)
	}
}

func getTestedAzureCluster(ctx context.Context, t *testing.T, cpCtrlClient client.Client) *capz.AzureCluster {
	clusterID, exists := os.LookupEnv("CLUSTER_ID")
	if !exists {
		t.Fatal("missing CLUSTER_ID environment variable")
	}

	azureClusterList := &capz.AzureClusterList{}
	err := cpCtrlClient.List(ctx, azureClusterList, client.MatchingLabels{capi.ClusterLabelName: clusterID})
	if err != nil {
		t.Fatalf("error listing AzureClusters in CP k8s API: %v", err)
	}

	if len(azureClusterList.Items) != 1 {
		t.Fatalf("found %d AzureClusters with cluster ID %s", len(azureClusterList.Items), clusterID)
	}

	azureCluster := azureClusterList.Items[0]
	return &azureCluster
}

func waitForAzureClusterCondition(cluster *capz.AzureCluster, conditionType capi.ConditionType, check conditionCheck, azureClusterGetterFunc azureClusterGetterFunc) {
	checkResult := check(cluster, conditionType)

	for ; checkResult != true; checkResult = check(cluster, conditionType) {
		time.Sleep(1 * time.Minute)
		updatedAzureClusterCR := azureClusterGetterFunc()
		*cluster = *updatedAzureClusterCR
	}
}

type azureClusterGetterFunc func() *capz.AzureCluster
