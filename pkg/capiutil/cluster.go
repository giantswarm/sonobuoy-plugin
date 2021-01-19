package capiutil

import (
	"context"

	"github.com/giantswarm/microerror"
	capi "sigs.k8s.io/cluster-api/api/v1alpha3"
	ctrl "sigs.k8s.io/controller-runtime/pkg/client"
)

func FindCluster(ctx context.Context, client ctrl.Client, clusterID string) (*capi.Cluster, error) {
	var cluster *capi.Cluster
	{
		var clusterList capi.ClusterList
		err := client.List(ctx, &clusterList, ctrl.MatchingLabels{capi.ClusterLabelName: clusterID})
		if err != nil {
			return nil, microerror.Mask(err)
		}

		if len(clusterList.Items) > 0 {
			cluster = &clusterList.Items[0]
		} else {
			return nil, microerror.Maskf(notFoundError, "can't find cluster with ID %q", clusterID)
		}
	}

	return cluster, nil
}
