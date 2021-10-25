package sonobuoy_plugin

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/giantswarm/apiextensions/v3/pkg/label"
	"github.com/giantswarm/backoff"
	"github.com/giantswarm/microerror"
	"github.com/giantswarm/micrologger"
	"github.com/giantswarm/sonobuoy-plugin/v5/pkg/assert"
	"github.com/giantswarm/sonobuoy-plugin/v5/pkg/capiutil"
	"github.com/giantswarm/sonobuoy-plugin/v5/pkg/ctrlclient"
	"github.com/giantswarm/sonobuoy-plugin/v5/pkg/provider"
	capz "sigs.k8s.io/cluster-api-provider-azure/api/v1alpha4"
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
		t.Logf("can't run azure test in %#q cluster, skipping", provider.GetProvider())
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

	azureMachinePools, err := capiutil.FindNonTestingAzureMachinePoolsForCluster(ctx, cpCtrlClient, clusterID)
	if err != nil {
		t.Fatalf("error finding MachinePools for cluster %q: %s", clusterID, microerror.JSON(err))
	}

	if len(azureMachinePools) == 0 {
		t.Fatal("Expected one azure machine pool to exist, none found.")
	}

	for _, azureMachinePool := range azureMachinePools {
		amp := azureMachinePool

		//
		// Check Metadata
		//

		// Check if 'giantswarm.io/machine-pool' label is set
		assert.LabelIsSet(t, &amp, label.MachinePool)

		// CAPZ cluster.
		o := func() error {
			machinePool, err := capiutil.FindAzureMachinePool(ctx, cpCtrlClient, amp.Name)
			if err != nil {
				return microerror.Maskf(executionFailedError, "error finding AzureMachinePool %s: %s", amp.Name, microerror.JSON(err))
			}

			if !machinePool.Status.Ready {
				return microerror.Maskf(unexpectedValueError, "expected %q AzureMachinePool's status.ready field to be true, was false", amp.Name)
			}

			return nil
		}

		b := backoff.NewExponential(20*time.Minute, backoff.LongMaxInterval)
		n := backoff.NewNotifier(logger, ctx)
		err := backoff.RetryNotify(o, b, n)
		if err != nil {
			t.Fatalf("error while waiting for azure machine pool to become ready")
		}

		machinePool, err := capiutil.FindMachinePool(ctx, cpCtrlClient, amp.Name)
		if err != nil {
			t.Fatalf("error finding MachinePool %s: %s", amp.Name, microerror.JSON(err))
		}

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

		if *amp.Status.ProvisioningState != capz.Succeeded {
			t.Fatalf("expected AzureMachinePool %s/%s Status.ProvisioningState is equal to %q, but got %q",
				amp.Namespace,
				amp.Name,
				capz.Succeeded,
				*amp.Status.ProvisioningState)
		}
	}
}
