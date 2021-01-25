package customresources

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/giantswarm/apiextensions/v3/pkg/label"
	"github.com/giantswarm/microerror"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	capz "sigs.k8s.io/cluster-api-provider-azure/api/v1alpha3"
	capzexp "sigs.k8s.io/cluster-api-provider-azure/exp/api/v1alpha3"
	capi "sigs.k8s.io/cluster-api/api/v1alpha3"
	capiexp "sigs.k8s.io/cluster-api/exp/api/v1alpha3"
	capiconditions "sigs.k8s.io/cluster-api/util/conditions"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/giantswarm/sonobuoy-plugin/v5/pkg/capiutil"
	"github.com/giantswarm/sonobuoy-plugin/v5/pkg/ctrlclient"
)

func Test_AzureMachinePoolCR(t *testing.T) {
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

		//
		// Check Metadata
		//

		// Check if 'release.giantswarm.io/version' label is set
		assertLabelIsSet(t, cluster, label.ReleaseVersion)

		// Check that Cluster and AzureMachinePool desired release version matches
		assertLabelIsEqual(t, cluster, &amp, label.ReleaseVersion)

		// Check if 'azure-operator.giantswarm.io/version' label is set
		assertLabelIsSet(t, cluster, label.AzureOperatorVersion)

		// Check that Cluster and AzureMachinePool azure-operator version matches
		assertLabelIsEqual(t, cluster, &amp, label.AzureOperatorVersion)

		// Check if 'giantswarm.io/machine-pool' label is set
		assertLabelIsSet(t, &amp, label.MachinePool)

		machinePool := getMachinePoolFromMetadata(ctx, t, cpCtrlClient, amp.ObjectMeta)

		// Check that MachinePool and AzureMachinePool giantswarm.io/machine-pool label matches
		assertLabelIsEqual(t, machinePool, &amp, label.MachinePool)

		//
		// Check Spec
		//
		if len(amp.Spec.ProviderID) == 0 {
			t.Fatalf("AzureMachinePool %s/%s does not have Spec.ProviderID field set", amp.Namespace, amp.Name)
		}

		desiredReplicas := *machinePool.Spec.Replicas
		if len(amp.Spec.ProviderIDList) != int(desiredReplicas) {
			t.Fatalf("expected %d replicas for AzureMachinePool %s/%s, but found %d in AzureMachinePool.Spec.ProviderIDList",
				int(desiredReplicas),
				amp.Namespace,
				amp.Name,
				len(amp.Spec.ProviderIDList))
		}

		//
		// Check Status
		//
		if amp.Status.Replicas != desiredReplicas {
			t.Fatalf("expected %d replicas for AzureMachinePool %s/%s, but found %d in AzureMachinePool.Status.Replicas",
				desiredReplicas,
				amp.Namespace,
				amp.Name,
				amp.Status.Replicas)
		}

		if amp.Status.ProvisioningState == nil {
			t.Fatalf("AzureMachinePool %s/%s Status.ProvisioningState is not set", amp.Namespace, amp.Name)
		}

		if *amp.Status.ProvisioningState != capz.VMStateSucceeded {
			t.Fatalf("expected AzureMachinePool %s/%s Status.ProvisioningState is equal to %q, but got %q",
				amp.Namespace,
				amp.Name,
				capz.VMStateSucceeded,
				*amp.Status.ProvisioningState)
		}

		// Wait for Ready condition to be True
		waitForAzureMachinePoolCondition(&amp, capi.ReadyCondition, capiconditions.IsTrue, azureMachinePoolGetter)

		if !amp.Status.Ready {
			t.Fatalf("AzureMachinePool %s/%s is not ready, Status.Ready == %t",
				amp.Namespace,
				amp.Name,
				amp.Status.Ready)
		}
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
