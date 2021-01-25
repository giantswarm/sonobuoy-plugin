package customresources

import (
	"context"
	"os"
	"testing"

	"github.com/giantswarm/apiextensions/v3/pkg/label"
	"github.com/giantswarm/microerror"
	capz "sigs.k8s.io/cluster-api-provider-azure/api/v1alpha3"
	capi "sigs.k8s.io/cluster-api/api/v1alpha3"
	capiconditions "sigs.k8s.io/cluster-api/util/conditions"

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

	azureMachinePools, err := capiutil.FindAzureMachinePoolsForCluster(ctx, cpCtrlClient, clusterID)
	if err != nil {
		t.Fatalf("error finding MachinePools for cluster %q: %s", clusterID, microerror.JSON(err))
	}

	azureMachinePoolGetter := func(azureMachinePoolID string) capiutil.TestedObject {
		machinePool, err := capiutil.FindAzureMachinePool(ctx, cpCtrlClient, azureMachinePoolID)
		if err != nil {
			t.Fatalf("error finding AzureMachinePool %s: %s", azureMachinePoolID, microerror.JSON(err))
		}

		return machinePool
	}

	for _, azureMachinePool := range azureMachinePools {
		amp := azureMachinePool

		//
		// Check Metadata
		//

		// Check if 'giantswarm.io/machine-pool' label is set
		assertLabelIsSet(t, &amp, label.MachinePool)

		// Check if 'release.giantswarm.io/version' label is set
		assertLabelIsSet(t, cluster, label.ReleaseVersion)

		// Check if 'azure-operator.giantswarm.io/version' label is set
		assertLabelIsSet(t, cluster, label.AzureOperatorVersion)

		//
		// Wait for main conditions checking the remaining parts of the resource:
		//   Ready == True
		//
		capiutil.WaitForCondition(t, &amp, capi.ReadyCondition, capiconditions.IsTrue, azureMachinePoolGetter)

		// Check that Cluster and AzureMachinePool desired release version matches
		assertLabelIsEqual(t, cluster, &amp, label.ReleaseVersion)

		// Check that Cluster and AzureMachinePool azure-operator version matches
		assertLabelIsEqual(t, cluster, &amp, label.AzureOperatorVersion)

		machinePool, err := capiutil.FindMachinePool(ctx, cpCtrlClient, amp.Name)
		if err != nil {
			t.Fatalf("error finding MachinePool %s: %s", amp.Name, microerror.JSON(err))
		}

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

		if !amp.Status.Ready {
			t.Fatalf("AzureMachinePool %s/%s is not ready, Status.Ready == %t",
				amp.Namespace,
				amp.Name,
				amp.Status.Ready)
		}
	}
}
