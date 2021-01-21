package customresources

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/giantswarm/apiextensions/v3/pkg/annotation"
	"github.com/giantswarm/apiextensions/v3/pkg/label"
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

	cluster := getTestedCluster(ctx, t, cpCtrlClient)

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
		mp := machinePool

		//
		// Check metadata
		//

		// Check if 'giantswarm.io/machine-pool' label is set
		assertLabelIsSet(t, &mp, label.MachinePool)

		// Check if 'release.giantswarm.io/version' label is set
		assertLabelIsSet(t, &mp, label.ReleaseVersion)

		// Check if Cluster and MachinePool have matching 'release.giantswarm.io/version' labels
		assertLabelIsEqual(t, cluster, &mp, label.ReleaseVersion)

		// Check if 'release.giantswarm.io/last-deployed-version' annotation is set
		assertAnnotationIsSet(t, &mp, annotation.LastDeployedReleaseVersion)

		// Check if Cluster and MachinePool have matching 'release.giantswarm.io/last-deployed-version' annotations
		assertAnnotationIsEqual(t, cluster, &mp, annotation.LastDeployedReleaseVersion)

		// Check if 'azure-operator.giantswarm.io/version' label is set
		assertLabelIsSet(t, &mp, label.AzureOperatorVersion)

		// Check that Cluster and MachinePool have matching 'azure-operator.giantswarm.io/version' labels
		assertLabelIsEqual(t, cluster, &mp, label.AzureOperatorVersion)

		// Check if 'cluster.k8s.io/cluster-api-autoscaler-node-group-min-size' annotation is set
		assertAnnotationIsSet(t, &mp, annotation.NodePoolMinSize)

		// Check if 'cluster.k8s.io/cluster-api-autoscaler-node-group-max-size' annotation is set
		assertAnnotationIsSet(t, &mp, annotation.NodePoolMaxSize)

		//
		// Check Spec
		//

		// Check if specified number of replicas is discovered
		if *mp.Spec.Replicas != mp.Status.Replicas {
			t.Fatalf("specified %d replicas, found %d", *mp.Spec.Replicas, mp.Status.Replicas)
		}

		// Check if all discovered replicas are ready
		if mp.Status.Replicas != mp.Status.ReadyReplicas {
			t.Fatalf("%d replicas found, but %d are ready", mp.Status.Replicas, mp.Status.AvailableReplicas)
		}

		// Wait for Ready condition to be True
		waitForMachinePoolCondition(&mp, capi.ReadyCondition, capiconditions.IsTrue, machinePoolGetter)

		// Wait for Creating condition to be False
		waitForMachinePoolCondition(&mp, conditions.Creating, capiconditions.IsFalse, machinePoolGetter)

		// Wait for Upgrading condition to be False
		waitForMachinePoolCondition(&mp, conditions.Upgrading, capiconditions.IsFalse, machinePoolGetter)

		// Verify that InfrastructureReady condition is True
		if !conditions.IsInfrastructureReadyTrue(&mp) {
			t.Fatalf("MachinePool InfrastructureReady condition is not True")
		}

		// Verify that ReplicasReady condition is True
		if !conditions.IsReplicasReadyTrue(&mp) {
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
