package ingress

import (
	"context"
	"fmt"
	"github.com/giantswarm/microerror"
	"github.com/giantswarm/micrologger"
	"github.com/giantswarm/sonobuoy-plugin/v5/tests/ctrlclient"
	"k8s.io/apimachinery/pkg/api/errors"
	"os"
	capiv1alpha3 "sigs.k8s.io/cluster-api/api/v1alpha3"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"testing"
)

func Test_AzureDelete(t *testing.T) {
	var err error

	ctx := context.Background()

	logger, err := micrologger.New(micrologger.Config{})
	if err != nil {
		t.Fatal(err)
	}

	cpCtrlClient, err := ctrlclient.CreateCPCtrlClient(ctx)
	if err != nil {
		t.Fatalf("error creating CP k8s client: %v", err)
	}

	clusterID, exists := os.LookupEnv("CLUSTER_ID")
	if !exists {
		t.Fatal("missing CLUSTER_ID environment variable")
	}

	// Delete cluster.
	err = deleteCluster(ctx, cpCtrlClient, logger, clusterID)
	if err != nil {
		t.Fatalf("error deleting cluster: %v", err)
	}

	// Wait for ClusterCR to be deleted.
	// Check the resource group is missing.

	//

	//

	//
	//cpKubeConfig, err := ctrlclient.GetCPKubeConfig(ctx)
	//if err != nil {
	//	t.Fatal(err)
	//}
}

func deleteCluster(ctx context.Context, ctrlClient client.Client, logger micrologger.Logger, clusterID string) error {
	logger.LogCtx(ctx, "level", "debug", "message", fmt.Sprintf("Deleting cluster %s", clusterID))

	labelSelector := client.MatchingLabels{}
	labelSelector[capiv1alpha3.ClusterLabelName] = clusterID

	crNamespace, err := getClusterNamespace(ctx, ctrlClient, labelSelector)
	if IsNotFound(err) {
		// fall through
	} else if err != nil {
		return microerror.Mask(err)
	}
	namespace := client.InNamespace(crNamespace)

	// delete provider-independent cluster CRs
	{
		err = ctrlClient.DeleteAllOf(ctx, &apiv1alpha2.Cluster{}, labelSelector, namespace)
		if errors.IsNotFound(err) {
			// fall through
		} else if err != nil {
			return microerror.Mask(err)
		}
	}

	inNamespace := client.InNamespace(crNamespace)

	// delete AzureCluster CR
	{
		err = ctrlClient.DeleteAllOf(ctx, &capzV1alpha3.AzureCluster{}, labelSelector, inNamespace)
		if errors.IsNotFound(err) {
			logger.LogCtx(ctx, "level", "debug", "message", fmt.Sprintf("AzureCluster CR not found for cluster ID %q", clusterID))
		} else if err != nil {
			return microerror.Mask(err)
		}
	}

	return nil
}

func getClusterNamespace(ctx context.Context, ctrlClient client.Client, labelSelector client.MatchingLabels) (string, error) {
	var cr apiv1alpha2.Cluster
	{
		crs := &apiv1alpha2.ClusterList{}

		err := ctrlClient.List(ctx, crs, labelSelector)
		if err != nil {
			return "", microerror.Mask(err)
		}
		if len(crs.Items) < 1 {
			return "", microerror.Maskf(notFoundError, "Cluster CR not found")
		}
		if len(crs.Items) > 1 {
			return "", microerror.Maskf(executionFailedError, "%d Cluster objects with same Cluster ID label when only one is allowed", len(crs.Items))
		}

		cr = crs.Items[0]
	}
	return cr.GetNamespace(), nil
}
