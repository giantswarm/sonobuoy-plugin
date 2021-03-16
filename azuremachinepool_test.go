package sonobuoy_plugin

import (
	"context"
	"os"
	"testing"

	"github.com/giantswarm/apiextensions/v3/pkg/label"
	"github.com/giantswarm/conditions/pkg/conditions"
	"github.com/giantswarm/microerror"
	"github.com/giantswarm/micrologger"
	capz "sigs.k8s.io/cluster-api-provider-azure/api/v1alpha3"
	capi "sigs.k8s.io/cluster-api/api/v1alpha3"
	capiconditions "sigs.k8s.io/cluster-api/util/conditions"

	"github.com/giantswarm/sonobuoy-plugin/v5/pkg/assert"
	"github.com/giantswarm/sonobuoy-plugin/v5/pkg/capiutil"
	"github.com/giantswarm/sonobuoy-plugin/v5/pkg/ctrlclient"
	"github.com/giantswarm/sonobuoy-plugin/v5/pkg/provider"
)

func Test_AzureMachinePoolCR(t *testing.T) {
	t.Parallel()

	var err error
	ctx := context.Background()

	regularLogger, err := micrologger.New(micrologger.Config{})
	if err != nil {
		t.Fatal(err)
	}

	logger := NewTestLogger(regularLogger, t)

	if provider.GetProvider() != "azure" {
		t.Log("can't run azure test in %#q cluster, skipping", provider.GetProvider())
		t.SkipNow()
		return
	}

	clusterID, exists := os.LookupEnv("CLUSTER_ID")
	if !exists {
		t.Fatal("missing CLUSTER_ID environment variable")
	}

	cpCtrlClient, err := ctrlclient.CreateCPCtrlClient()
	if err != nil {
		t.Fatalf("error creating CP k8s client: %v", err)
	}

	clusterGetter := func(clusterName string) capiutil.TestedObject {
		cluster, err := capiutil.FindCluster(ctx, cpCtrlClient, clusterName)
		if err != nil {
			t.Fatalf("error finding cluster: %s", microerror.JSON(err))
		}

		return cluster
	}

	cluster := clusterGetter(clusterID).(*capi.Cluster)

	azureMachinePools, err := capiutil.FindNonTestingAzureMachinePoolsForCluster(ctx, cpCtrlClient, clusterID)
	if err != nil {
		t.Fatalf("error finding MachinePools for cluster %q: %s", clusterID, microerror.JSON(err))
	}

	if len(azureMachinePools) == 0 {
		t.Fatal("Expected one azure machine pool to exist, none found.")
	}

	azureMachinePoolGetter := func(azureMachinePoolID string) capiutil.TestedObject {
		machinePool, err := capiutil.FindAzureMachinePool(ctx, cpCtrlClient, azureMachinePoolID)
		if err != nil {
			t.Fatalf("error finding AzureMachinePool %s: %s", azureMachinePoolID, microerror.JSON(err))
		}

		return machinePool
	}

	machinePoolGetter := func(machinePoolID string) capiutil.TestedObject {
		machinePool, err := capiutil.FindMachinePool(ctx, cpCtrlClient, machinePoolID)
		if err != nil {
			t.Fatalf("error finding MachinePool %s: %s", machinePoolID, microerror.JSON(err))
		}

		return machinePool
	}

	for _, azureMachinePool := range azureMachinePools {
		amp := azureMachinePool

		//
		// Check Metadata
		//

		// Check if 'giantswarm.io/machine-pool' label is set
		assert.LabelIsSet(t, &amp, label.MachinePool)

		// Check if 'release.giantswarm.io/version' label is set
		assert.LabelIsSet(t, cluster, label.ReleaseVersion)

		// Check if 'azure-operator.giantswarm.io/version' label is set
		assert.LabelIsSet(t, cluster, label.AzureOperatorVersion)

		//
		// Wait for main conditions checking the remaining parts of the resource:
		//   Ready == True
		//
		capiutil.WaitForCondition(t, ctx, logger, &amp, capi.ReadyCondition, capiconditions.IsTrue, azureMachinePoolGetter)

		// Check that Cluster and AzureMachinePool desired release version matches
		assert.LabelIsEqual(t, cluster, &amp, label.ReleaseVersion)

		// Check that Cluster and AzureMachinePool azure-operator version matches
		assert.LabelIsEqual(t, cluster, &amp, label.AzureOperatorVersion)

		machinePool, err := capiutil.FindMachinePool(ctx, cpCtrlClient, amp.Name)
		if err != nil {
			t.Fatalf("error finding MachinePool %s: %s", amp.Name, microerror.JSON(err))
		}

		//
		// Since we will be using MachinePool Status in the checks, we should
		// wait for the MachinePool to be ready and to have up-to-date conditions.
		//

		// Wait for Ready condition to be True
		capiutil.WaitForCondition(t, ctx, logger, machinePool, capi.ReadyCondition, capiconditions.IsTrue, machinePoolGetter)

		// Wait for Creating condition to be False
		capiutil.WaitForCondition(t, ctx, logger, machinePool, conditions.Creating, capiconditions.IsFalse, machinePoolGetter)

		// Wait for Upgrading condition to be False
		capiutil.WaitForCondition(t, ctx, logger, machinePool, conditions.Upgrading, capiconditions.IsFalse, machinePoolGetter)

		// Check that MachinePool and AzureMachinePool giantswarm.io/machine-pool label matches
		assert.LabelIsEqual(t, machinePool, &amp, label.MachinePool)

		// Assert that AzureMachinePool owner reference is set to the specified MachinePool
		assert.ExpectedOwnerReferenceIsSet(t, &amp, machinePool)

		//
		// Check Spec
		//
		if len(amp.Spec.ProviderID) == 0 {
			t.Fatalf("AzureMachinePool %s/%s does not have Spec.ProviderID field set", amp.Namespace, amp.Name)
		}

		foundReplicasInMachinePool := machinePool.Status.Replicas
		if len(amp.Spec.ProviderIDList) != int(foundReplicasInMachinePool) {
			t.Fatalf("expected %d replicas for AzureMachinePool %s/%s, but found %d in AzureMachinePool.Spec.ProviderIDList",
				int(foundReplicasInMachinePool),
				amp.Namespace,
				amp.Name,
				len(amp.Spec.ProviderIDList))
		}

		//
		// Check Status
		//
		if amp.Status.Replicas != foundReplicasInMachinePool {
			t.Fatalf("expected %d replicas for AzureMachinePool %s/%s, but found %d in AzureMachinePool.Status.Replicas",
				foundReplicasInMachinePool,
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
