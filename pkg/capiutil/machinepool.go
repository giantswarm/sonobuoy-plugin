package capiutil

import (
	"context"

	"github.com/giantswarm/apiextensions/v3/pkg/label"
	"github.com/giantswarm/microerror"
	capi "sigs.k8s.io/cluster-api/api/v1alpha3"
	capiexp "sigs.k8s.io/cluster-api/exp/api/v1alpha3"
	ctrl "sigs.k8s.io/controller-runtime/pkg/client"
)

func FindMachinePool(ctx context.Context, client ctrl.Client, machinePoolID string) (*capiexp.MachinePool, error) {
	var machinePool *capiexp.MachinePool
	{
		var machinePoolList capiexp.MachinePoolList
		err := client.List(ctx, &machinePoolList, ctrl.MatchingLabels{label.MachinePool: machinePoolID})
		if err != nil {
			return nil, microerror.Mask(err)
		}

		if len(machinePoolList.Items) == 0 {
			return nil, microerror.Maskf(notFoundError, "can't find MachinePool with ID %q", machinePoolID)
		} else if len(machinePoolList.Items) == 1 {
			machinePool = &machinePoolList.Items[0]
		} else {
			return nil, microerror.Maskf(tooManyObjectsError, "found %d MachinePools with ID %s", len(machinePoolList.Items), machinePoolID)
		}
	}

	return machinePool, nil
}

func FindMachinePoolsForCluster(ctx context.Context, client ctrl.Client, clusterID string) ([]capiexp.MachinePool, error) {
	var machinePools []capiexp.MachinePool
	{
		var machinePoolList capiexp.MachinePoolList
		err := client.List(ctx, &machinePoolList, ctrl.MatchingLabels{capi.ClusterLabelName: clusterID})
		if err != nil {
			return nil, microerror.Mask(err)
		}

		machinePools = machinePoolList.Items
	}

	return machinePools, nil
}
