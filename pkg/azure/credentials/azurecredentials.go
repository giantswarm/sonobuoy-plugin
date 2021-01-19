package credentials

import (
	"context"

	"github.com/giantswarm/apiextensions/v3/pkg/label"
	"github.com/giantswarm/microerror"
	corev1 "k8s.io/api/core/v1"
	capi "sigs.k8s.io/cluster-api/api/v1alpha3"
	ctrl "sigs.k8s.io/controller-runtime/pkg/client"
)

type ServicePrincipal struct {
	ClientID       string
	ClientSecret   string
	TenantID       string
	SubscriptionID string
}

func ForCluster(ctx context.Context, client ctrl.Client, cluster *capi.Cluster) (*ServicePrincipal, error) {
	// Extract the organization name.
	var orgName string
	{
		orgName = cluster.Labels[label.Organization]
		if orgName == "" {
			return nil, microerror.Maskf(executionFailedError, "Organization label not found or empty.")
		}
	}

	// Find the secret holding organization's credentials.
	secret, err := findSecret(ctx, client, orgName)
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

func findSecret(ctx context.Context, client ctrl.Client, orgName string) (*corev1.Secret, error) {
	// Look for a secret with labels "app: credentiald" and "giantswarm.io/organization: org"
	secrets := &corev1.SecretList{}

	err := client.List(ctx, secrets, ctrl.MatchingLabels{"app": "credentiald", label.Organization: orgName})
	if err != nil {
		return nil, microerror.Mask(err)
	}

	if len(secrets.Items) == 1 {
		return &secrets.Items[0], nil
	} else if len(secrets.Items) == 0 {
		secret := &corev1.Secret{}

		// Organization-specific secret not found, use secret named "credential-default".
		err := client.Get(ctx, ctrl.ObjectKey{Namespace: "giantswarm", Name: "credential-default"}, secret)
		if err != nil {
			return nil, microerror.Mask(err)
		}

		return secret, nil
	}

	return nil, microerror.Maskf(executionFailedError, "Unable to find secret for organization %s", orgName)
}
