package customresources

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/giantswarm/conditions/pkg/conditions"
	capi "sigs.k8s.io/cluster-api/api/v1alpha3"
	capiexp "sigs.k8s.io/cluster-api/exp/api/v1alpha3"
	capiconditions "sigs.k8s.io/cluster-api/util/conditions"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/giantswarm/sonobuoy-plugin/v5/pkg/ctrlclient"
)

func Test_MachinePoolCR(t *testing.T) {
	var err error
	ctx := context.Background()

	cpCtrlClient, err := ctrlclient.CreateCPCtrlClient(ctx)
	if err != nil {
		t.Fatalf("error creating CP k8s client: %v", err)
	}

	machinePoolGetter := func(machinePool *capiexp.MachinePool) *capiexp.MachinePool {
		freshMachinePool := capiexp.MachinePool{}
		machinePoolKey := client.ObjectKey{Namespace: machinePool.Namespace, Name: machinePool.Name}

		err := cpCtrlClient.Get(ctx, machinePoolKey, &freshMachinePool)
		if err != nil {
			t.Fatalf("error getting MachinePool %s", machinePoolKey.String())
		}

		return &freshMachinePool
	}

	machinePools := getTestedMachinePools(ctx, t, cpCtrlClient)

	for _, machinePool := range machinePools {
		machinePoolToCheck := machinePool

		// Wait for Ready condition to be True
		waitForMachinePoolCondition(&machinePoolToCheck, capi.ReadyCondition, capiconditions.IsTrue, machinePoolGetter)

		// Wait for Creating condition to be False
		waitForMachinePoolCondition(&machinePoolToCheck, conditions.Creating, capiconditions.IsFalse, machinePoolGetter)

		// Wait for Upgrading condition to be False
		waitForMachinePoolCondition(&machinePoolToCheck, conditions.Upgrading, capiconditions.IsFalse, machinePoolGetter)

		// Verify that InfrastructureReady condition is True
		if !conditions.IsInfrastructureReadyTrue(&machinePoolToCheck) {
			t.Fatalf("MachinePool InfrastructureReady condition is not True")
		}

		// Verify that ReplicasReady condition is True
		if !conditions.IsReplicasReadyTrue(&machinePoolToCheck) {
			t.Fatalf("MachinePool ReplicasReady condition is not True")
		}
	}
}

func waitForMachinePoolCondition(machinePool *capiexp.MachinePool, conditionType capi.ConditionType, check conditionCheck, machinePoolGetter machinePoolGetterFunc) {
	checkResult := check(machinePool, conditionType)

	for ; checkResult != true; checkResult = check(machinePool, conditionType) {
		time.Sleep(1 * time.Minute)
		refreshedMachinePoolCR := machinePoolGetter(machinePool)
		*machinePool = *refreshedMachinePoolCR
	}
}

type machinePoolGetterFunc func(machinePool *capiexp.MachinePool) *capiexp.MachinePool

func getTestedMachinePools(ctx context.Context, t *testing.T, cpCtrlClient client.Client) []capiexp.MachinePool {
	clusterID, exists := os.LookupEnv("CLUSTER_ID")
	if !exists {
		t.Fatal("missing CLUSTER_ID environment variable")
	}

	machinePoolList := &capiexp.MachinePoolList{}
	err := cpCtrlClient.List(ctx, machinePoolList, client.MatchingLabels{capi.ClusterLabelName: clusterID})
	if err != nil {
		t.Fatalf("error listing Clusters in CP k8s API: %v", err)
	}

	return machinePoolList.Items
}
