module github.com/giantswarm/sonobuoy-plugin/v5

go 1.15

require (
	github.com/Azure/azure-sdk-for-go v46.4.0+incompatible
	github.com/Azure/go-autorest/autorest v0.11.7
	github.com/Azure/go-autorest/autorest/azure/auth v0.5.2
	github.com/Azure/go-autorest/autorest/to v0.4.0
	github.com/giantswarm/apiextensions/v3 v3.13.0
	github.com/giantswarm/apptest v0.9.0
	github.com/giantswarm/backoff v0.2.0
	github.com/giantswarm/conditions v0.3.0
	github.com/giantswarm/microerror v0.3.0
	github.com/giantswarm/micrologger v0.4.0
	k8s.io/api v0.18.9
	k8s.io/apiextensions-apiserver v0.18.9
	k8s.io/apimachinery v0.18.9
	k8s.io/client-go v0.18.9
	sigs.k8s.io/cluster-api v0.3.11
	sigs.k8s.io/cluster-api-provider-azure v0.4.9
	sigs.k8s.io/controller-runtime v0.6.4
)

replace (
	sigs.k8s.io/cluster-api => github.com/giantswarm/cluster-api v0.3.10-gs
	sigs.k8s.io/cluster-api-provider-azure => github.com/giantswarm/cluster-api-provider-azure v0.4.9-gsalpha2
)
