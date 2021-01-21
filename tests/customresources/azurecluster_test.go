package customresources

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/giantswarm/apiextensions/v3/pkg/label"
	capz "sigs.k8s.io/cluster-api-provider-azure/api/v1alpha3"
	capi "sigs.k8s.io/cluster-api/api/v1alpha3"
	capiconditions "sigs.k8s.io/cluster-api/util/conditions"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/giantswarm/sonobuoy-plugin/v5/pkg/ctrlclient"
)

func Test_AzureClusterCR(t *testing.T) {
	var err error
	ctx := context.Background()

	cpCtrlClient, err := ctrlclient.CreateCPCtrlClient(ctx)
	if err != nil {
		t.Fatalf("error creating CP k8s client: %v", err)
	}

	cluster := getTestedCluster(ctx, t, cpCtrlClient)

	azureClusterGetter := func() *capz.AzureCluster {
		return getTestedAzureCluster(ctx, t, cpCtrlClient)
	}

	azureCluster := azureClusterGetter()

	// Check that Cluster and MachinePool desired release version matches
	assertLabelIsEqual(t, cluster, azureCluster, label.ReleaseVersion)

	// Check that Cluster and MachinePool azure-operator version matches
	assertLabelIsEqual(t, cluster, azureCluster, label.AzureOperatorVersion)

	// Wait for Ready condition to be True
	waitForAzureClusterCondition(azureCluster, capi.ReadyCondition, capiconditions.IsTrue, azureClusterGetter)
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
