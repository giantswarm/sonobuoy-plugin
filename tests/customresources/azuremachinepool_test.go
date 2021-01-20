package customresources

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/giantswarm/apiextensions/v3/pkg/annotation"
	"github.com/giantswarm/apiextensions/v3/pkg/label"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	capzexp "sigs.k8s.io/cluster-api-provider-azure/exp/api/v1alpha3"
	capi "sigs.k8s.io/cluster-api/api/v1alpha3"
	capiexp "sigs.k8s.io/cluster-api/exp/api/v1alpha3"
	capiconditions "sigs.k8s.io/cluster-api/util/conditions"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/giantswarm/sonobuoy-plugin/v5/pkg/ctrlclient"
)

func Test_AzureMachinePoolCR(t *testing.T) {
	var err error
	ctx := context.Background()

	cpCtrlClient, err := ctrlclient.CreateCPCtrlClient(ctx)
	if err != nil {
		t.Fatalf("error creating CP k8s client: %v", err)
	}

	cluster := getTestedCluster(ctx, t, cpCtrlClient)
	azureMachinePools := getTestedAzureMachinePools(ctx, t, cpCtrlClient)

	azureMachinePoolGetter := func(azureMachinePool *capzexp.AzureMachinePool) *capzexp.AzureMachinePool {
		freshAzureMachinePool := capzexp.AzureMachinePool{}
		azureMachinePoolKey := client.ObjectKey{Namespace: azureMachinePool.Namespace, Name: azureMachinePool.Name}

		err := cpCtrlClient.Get(ctx, azureMachinePoolKey, &freshAzureMachinePool)
		if err != nil {
			t.Fatalf("error getting AzureMachinePool %s", azureMachinePoolKey.String())
		}

		return &freshAzureMachinePool
	}

	for _, azureMachinePool := range azureMachinePools {
		amp := azureMachinePool

		// Check that Cluster and AzureMachinePool desired release version matches
		assertLabelIsEqual(t, cluster, &amp, label.ReleaseVersion)

		// Check that Cluster and AzureMachinePool last deployed release version matches
		assertAnnotationIsEqual(t, cluster, &amp, annotation.LastDeployedReleaseVersion)

		// Check that Cluster and AzureMachinePool azure-operator version matches
		assertLabelIsEqual(t, cluster, &amp, label.AzureOperatorVersion)

		machinePool := getMachinePoolFromMetadata(ctx, t, cpCtrlClient, amp.ObjectMeta)

		// Check that MachinePool and AzureMachinePool giantswarm.io/machine-pool label matches
		assertLabelIsEqual(t, machinePool, &amp, label.MachinePool)

		// Wait for Ready condition to be True
		waitForAzureMachinePoolCondition(&amp, capi.ReadyCondition, capiconditions.IsTrue, azureMachinePoolGetter)
	}
}

func getTestedAzureMachinePools(ctx context.Context, t *testing.T, cpCtrlClient client.Client) []capzexp.AzureMachinePool {
	clusterID, exists := os.LookupEnv("CLUSTER_ID")
	if !exists {
		t.Fatal("missing CLUSTER_ID environment variable")
	}

	azureMachinePoolList := &capzexp.AzureMachinePoolList{}
	err := cpCtrlClient.List(ctx, azureMachinePoolList, client.MatchingLabels{capi.ClusterLabelName: clusterID})
	if err != nil {
		t.Fatalf("error listing AzureMachinePools in CP k8s API: %v", err)
	}

	return azureMachinePoolList.Items
}

func getMachinePoolFromMetadata(ctx context.Context, t *testing.T, cpCtrlClient client.Client, metadata metav1.ObjectMeta) *capiexp.MachinePool {
	machinePool := capiexp.MachinePool{}
	machinePoolKey := client.ObjectKey{
		Namespace: metadata.Namespace,
		Name:      metadata.Name,
	}

	err := cpCtrlClient.Get(ctx, machinePoolKey, &machinePool)
	if err != nil {
		t.Fatalf("error getting MachinePool %s in CP k8s API: %v", machinePoolKey.String(), err)
	}

	return &machinePool
}

type azureMachinePoolGetterFunc func(azureMachinePool *capzexp.AzureMachinePool) *capzexp.AzureMachinePool

func waitForAzureMachinePoolCondition(azureMachinePool *capzexp.AzureMachinePool, conditionType capi.ConditionType, check conditionCheck, azureMachinePoolGetter azureMachinePoolGetterFunc) {
	checkResult := check(azureMachinePool, conditionType)

	for ; checkResult != true; checkResult = check(azureMachinePool, conditionType) {
		time.Sleep(1 * time.Minute)
		refreshedMachinePoolCR := azureMachinePoolGetter(azureMachinePool)
		*azureMachinePool = *refreshedMachinePoolCR
	}
}
