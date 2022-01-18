package capiutil

import (
	"context"

	"github.com/giantswarm/microerror"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	capi "sigs.k8s.io/cluster-api/api/v1beta1"
	ctrl "sigs.k8s.io/controller-runtime/pkg/client"
)

func FindCluster(ctx context.Context, client ctrl.Client, clusterID string) (*capi.Cluster, error) {
	cluster := &capi.Cluster{}
	{
		err := client.Get(ctx, ctrl.ObjectKey{Namespace: v1.NamespaceDefault, Name: clusterID}, cluster)
		if apierrors.IsNotFound(err) {
			// no cluster CR with a matching name, try to find it with labels
			// no-op
		} else if err != nil {
			return nil, microerror.Mask(err)
		} else {
			return cluster, nil
		}

		var clusterList capi.ClusterList
		err = client.List(ctx, &clusterList, ctrl.MatchingLabels{capi.ClusterLabelName: clusterID})
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
