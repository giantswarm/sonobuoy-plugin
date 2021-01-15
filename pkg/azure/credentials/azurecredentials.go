package credentials

import (
	"context"

	"github.com/giantswarm/apiextensions/v3/pkg/label"
	"github.com/giantswarm/microerror"
	corev1 "k8s.io/api/core/v1"
	capiv1alpha3 "sigs.k8s.io/cluster-api/api/v1alpha3"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type ServicePrincipal struct {
	ClientID       string
	ClientSecret   string
	TenantID       string
	SubscriptionID string
}

func FromSecret(ctx context.Context, ctrlClient client.Client, clusterID string) (*ServicePrincipal, error) {
	// Find the cluster CR.
	cluster, err := findClusterCR(ctx, ctrlClient, clusterID)
	if err != nil {
		return nil, microerror.Mask(err)
	}
	// Extract the organization name.
	var orgName string
	{
		orgName = cluster.Labels[label.Organization]
		if orgName == "" {
			return nil, microerror.Maskf(executionFailedError, "Organization label not found or empty.")
		}
	}

	// Find the secret holding organization's credentials.
	secret, err := findSecret(ctx, ctrlClient, orgName)
	if err != nil {
		return nil, microerror.Mask(err)
	}

	// Extract fields from the secret.
	sp := ServicePrincipal{
		ClientID:       string(secret.Data["azure.azureoperator.clientid"]),
		ClientSecret:   string(secret.Data["azure.azureoperator.clientsecret"]),
		SubscriptionID: string(secret.Data["azure.azureoperator.subscriptionid"]),
		TenantID:       string(secret.Data["azure.azureoperator.tenantid"]),
	}

	return &sp, nil
}

func findClusterCR(ctx context.Context, ctrlClient client.Client, clusterID string) (*capiv1alpha3.Cluster, error) {
	clusterList := &capiv1alpha3.ClusterList{}
	err := ctrlClient.List(ctx, clusterList, client.MatchingLabels{capiv1alpha3.ClusterLabelName: clusterID})
	if err != nil {
		return nil, microerror.Mask(err)
	}

	if len(clusterList.Items) != 1 {
		return nil, microerror.Maskf(executionFailedError, "Unable to find ClusterCR with label %q = %q. Expected 1 result, %d found.", capiv1alpha3.ClusterLabelName, clusterID, len(clusterList.Items))
	}

	return &clusterList.Items[0], nil
}

func findSecret(ctx context.Context, ctrlClient client.Client, orgName string) (*corev1.Secret, error) {
	// Look for a secret with labels "app: credentiald" and "giantswarm.io/organization: <normalized org name>"
	secrets := &corev1.SecretList{}

	err := ctrlClient.List(ctx, secrets, client.MatchingLabels{"app": "credentiald", label.Organization: asDNSLabelName(orgName)})
	if err != nil {
		return nil, microerror.Mask(err)
	}

	if len(secrets.Items) == 1 {
		return &secrets.Items[0], nil
	} else if len(secrets.Items) == 0 {
		secret := &corev1.Secret{}

		// Organization-specific secret not found, use secret named "credential-default".
		err := ctrlClient.Get(ctx, client.ObjectKey{Namespace: "giantswarm", Name: "credential-default"}, secret)
		if err != nil {
			return nil, microerror.Mask(err)
		}

		return secret, nil
	}

	return nil, microerror.Maskf(executionFailedError, "Unable to find secret for organization %s", orgName)
}
