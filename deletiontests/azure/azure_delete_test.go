package ingress

import (
	"context"
	"os"
	"testing"

	"github.com/giantswarm/backoff"
	"github.com/giantswarm/microerror"
	"github.com/giantswarm/micrologger"
	"k8s.io/apimachinery/pkg/api/errors"
	capz "sigs.k8s.io/cluster-api-provider-azure/api/v1alpha3"
	capi "sigs.k8s.io/cluster-api/api/v1alpha3"
	ctrl "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/giantswarm/sonobuoy-plugin/v5/pkg/azure"
	"github.com/giantswarm/sonobuoy-plugin/v5/pkg/azure/credentials"
	"github.com/giantswarm/sonobuoy-plugin/v5/pkg/capiutil"
	"github.com/giantswarm/sonobuoy-plugin/v5/pkg/ctrlclient"
	"github.com/giantswarm/sonobuoy-plugin/v5/pkg/provider"
)

func Test_AzureDelete(t *testing.T) {
	var err error

	ctx := context.Background()

	enableThisTest, exists := os.LookupEnv("TEST_DELETION")
	if !exists || enableThisTest != "1" {
		t.Skip("Skipping cluster deletion test (pass 'TEST_DELETION=1' env var to enable it)")
	}

	logger, err := micrologger.New(micrologger.Config{})
	if err != nil {
		t.Fatal(err)
	}

	providerName, exists := os.LookupEnv(provider.ProviderEnvVarName)
	if !exists {
		t.Fatalf("missing %s environment variable", provider.ProviderEnvVarName)
	}

	if providerName != "azure" {
		logger.Debugf(ctx, "Only Azure provider is supported by this test, skipping")
		return
	}

	cpCtrlClient, err := ctrlclient.CreateCPCtrlClient()
	if err != nil {
		t.Fatalf("error creating CP k8s client: %v", err)
	}

	clusterID, exists := os.LookupEnv("CLUSTER_ID")
	if !exists {
		t.Fatal("missing CLUSTER_ID environment variable")
	}

	cluster, err := capiutil.FindCluster(ctx, cpCtrlClient, clusterID)
	if err != nil {
		t.Fatal(microerror.JSON(err))
	}

	servicePrincipal, err := credentials.ForCluster(ctx, cpCtrlClient, cluster)
	if err != nil {
		t.Fatalf("can't get service principal credentials: %v", err)
	}

	azureClient, err := azure.NewClient(azure.ClientConfig{ServicePrincipal: servicePrincipal})
	if err != nil {
		t.Fatalf("error creating azure client: %v", err)
	}

	logger.Debugf(ctx, "Checking if resource group exists.")

	// Check the resourge group exists.
	exists, err = azureClient.ResourceGroup.Exists(ctx, clusterID)
	if err != nil {
		t.Fatalf("Unable to check if the cluster's resource group exists: %v", err)
	}
	if !exists {
		t.Fatal("Expected Resource Group to exist but it doesn't.")
	}

	// Delete cluster.
	{
		logger.Debugf(ctx, "Deleting cluster %s", clusterID)
		err = deleteCluster(ctx, cpCtrlClient, logger, clusterID)
		if err != nil {
			t.Fatalf("error deleting cluster: %v", err)
		}
		logger.Debugf(ctx, "Cluster %s deletion successful", clusterID)
	}

	// Wait for Cluster CR to be deleted.
	{
		logger.Debugf(ctx, "Waiting for cluster CR for cluster %s to be deleted", clusterID)
		o := func() error {
			clusters := &capi.ClusterList{}
			err := cpCtrlClient.List(ctx, clusters, ctrl.MatchingLabels{capi.ClusterLabelName: clusterID})
			if err != nil {
				return microerror.Mask(err)
			}

			if len(clusters.Items) > 0 {
				return microerror.Maskf(customResourceStillExistsError, "Cluster CR for cluster %s still exists (%d found)", clusterID, len(clusters.Items))
			}

			return nil
		}
		b := backoff.NewConstant(backoff.LongMaxWait, backoff.LongMaxInterval)
		n := backoff.NewNotifier(logger, ctx)
		err = backoff.RetryNotify(o, b, n)
		if err != nil {
			t.Fatalf("Failed waiting for Cluster CR to be deleted: %v", err)
		}
	}

	// Check the resource group is missing.
	// Using a backoff here to overcome Azure API eventual integrity.
	{
		logger.Debugf(ctx, "Waiting for resource group %s to be deleted", clusterID)

		o := func() error {
			exists, err := azureClient.ResourceGroup.Exists(ctx, clusterID)
			if err != nil {
				return microerror.Maskf(executionFailedError, "Error checking if the resource group exists: %v", err)
			}

			if exists {
				return microerror.Maskf(executionFailedError, "Resource group still exists: %v", err)
			}

			return nil
		}
		b := backoff.NewConstant(backoff.MediumMaxWait, backoff.ShortMaxInterval)
		n := backoff.NewNotifier(logger, ctx)
		err = backoff.RetryNotify(o, b, n)
		if err != nil {
			t.Fatalf("Failed waiting for Resource Group to be deleted: %v", err)
		}
	}
}

func deleteCluster(ctx context.Context, client ctrl.Client, logger micrologger.Logger, clusterID string) error {
	labelSelector := ctrl.MatchingLabels{}
	labelSelector[capi.ClusterLabelName] = clusterID

	crNamespace, err := getClusterNamespace(ctx, client, labelSelector)
	if IsNotFound(err) {
		// fall through
	} else if err != nil {
		return microerror.Mask(err)
	}
	namespace := ctrl.InNamespace(crNamespace)

	// delete provider-independent cluster CRs
	{
		err = client.DeleteAllOf(ctx, &capi.Cluster{}, labelSelector, namespace)
		if errors.IsNotFound(err) {
			// fall through
		} else if err != nil {
			return microerror.Mask(err)
		}
	}

	inNamespace := ctrl.InNamespace(crNamespace)

	// delete AzureCluster CR
	{
		err = client.DeleteAllOf(ctx, &capz.AzureCluster{}, labelSelector, inNamespace)
		if errors.IsNotFound(err) {
			logger.Debugf(ctx, "AzureCluster CR not found for cluster ID %q", clusterID)
		} else if err != nil {
			return microerror.Mask(err)
		}
	}

	return nil
}

func getClusterNamespace(ctx context.Context, client ctrl.Client, labelSelector ctrl.MatchingLabels) (string, error) {
	var cr capi.Cluster
	{
		crs := &capi.ClusterList{}

		err := client.List(ctx, crs, labelSelector)
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
