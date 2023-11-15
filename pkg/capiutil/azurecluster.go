package capiutil

import (
	"context"

	"github.com/giantswarm/microerror"
	capz "sigs.k8s.io/cluster-api-provider-azure/api/v1beta1"
	capiv1alpha3 "sigs.k8s.io/cluster-api/api/v1beta1"
	ctrl "sigs.k8s.io/controller-runtime/pkg/client"
)

func FindAzureCluster(ctx context.Context, client ctrl.Client, azureClusterID string) (*capz.AzureCluster, error) {
	var azureCluster *capz.AzureCluster
	{
		azureClusterList := &capz.AzureClusterList{}
		err := client.List(ctx, azureClusterList, ctrl.MatchingLabels{capiv1alpha3.ClusterNameLabel: azureClusterID})
		if err != nil {
			return nil, microerror.Mask(err)
		}

		if len(azureClusterList.Items) == 0 {
			return nil, microerror.Maskf(notFoundError, "can't find AzureCluster with ID %q", azureClusterID)
		} else if len(azureClusterList.Items) == 1 {
			azureCluster = &azureClusterList.Items[0]
		} else {
			return nil, microerror.Maskf(tooManyObjectsError, "found %d AzureClusters with ID %s", len(azureClusterList.Items), azureClusterID)
		}
	}
	return azureCluster, nil
}
