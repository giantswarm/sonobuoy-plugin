package capiutil

import (
	"context"

	"github.com/giantswarm/apiextensions/v3/pkg/label"
	"github.com/giantswarm/microerror"
	capzexp "sigs.k8s.io/cluster-api-provider-azure/exp/api/v1alpha3"
	capi "sigs.k8s.io/cluster-api/api/v1beta1"
	ctrl "sigs.k8s.io/controller-runtime/pkg/client"
)

func FindAzureMachinePool(ctx context.Context, client ctrl.Client, azureMachinePoolID string) (*capzexp.AzureMachinePool, error) {
	var azureMachinePool *capzexp.AzureMachinePool
	{
		var azureMachinePoolList capzexp.AzureMachinePoolList
		err := client.List(ctx, &azureMachinePoolList, ctrl.MatchingLabels{label.MachinePool: azureMachinePoolID})
		if err != nil {
			return nil, microerror.Mask(err)
		}

		if len(azureMachinePoolList.Items) == 0 {
			return nil, microerror.Maskf(notFoundError, "can't find AzureMachinePool with ID %q", azureMachinePoolID)
		} else if len(azureMachinePoolList.Items) == 1 {
			azureMachinePool = &azureMachinePoolList.Items[0]
		} else {
			return nil, microerror.Maskf(tooManyObjectsError, "found %d AzureMachinePool with ID %s", len(azureMachinePoolList.Items), azureMachinePoolID)
		}
	}

	return azureMachinePool, nil
}

// FindNonTestingAzureMachinePoolsForCluster returns list of `AzureMachinePool` belonging to the
// specified cluster ID.
// It filters out potential `AzureMachinePool` created by other e2e tests.
func FindNonTestingAzureMachinePoolsForCluster(ctx context.Context, client ctrl.Client, clusterID string) ([]capzexp.AzureMachinePool, error) {
	var azureMachinePools []capzexp.AzureMachinePool
	{
		azureMachinePoolList, err := FindAllAzureMachinePoolsForCluster(ctx, client, clusterID)
		if err != nil {
			return nil, microerror.Mask(err)
		}

		for _, azureMachinePool := range azureMachinePoolList {
			_, isE2E := azureMachinePool.Labels[E2ENodepool]
			if isE2E {
				continue
			}

			azureMachinePools = append(azureMachinePools, azureMachinePool)
		}
	}

	return azureMachinePools, nil
}

// FindAllAzureMachinePoolsForCluster returns list of `AzureMachinePool` belonging to the
// specified cluster ID.
func FindAllAzureMachinePoolsForCluster(ctx context.Context, client ctrl.Client, clusterID string) ([]capzexp.AzureMachinePool, error) {
	var azureMachinePoolList capzexp.AzureMachinePoolList
	err := client.List(ctx, &azureMachinePoolList, ctrl.MatchingLabels{capi.ClusterLabelName: clusterID})
	if err != nil {
		return nil, microerror.Mask(err)
	}

	return azureMachinePoolList.Items, nil
}
