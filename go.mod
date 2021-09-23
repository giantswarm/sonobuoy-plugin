module github.com/giantswarm/sonobuoy-plugin/v5

go 1.15

require (
	github.com/Azure/azure-sdk-for-go v55.2.0+incompatible
	github.com/Azure/go-autorest/autorest v0.11.18
	github.com/Azure/go-autorest/autorest/azure/auth v0.5.3
	github.com/Azure/go-autorest/autorest/to v0.4.0
	github.com/giantswarm/apiextensions/v3 v3.13.0
	github.com/giantswarm/apptest v0.9.0
	github.com/giantswarm/backoff v0.2.0
	github.com/giantswarm/conditions v0.4.0
	github.com/giantswarm/microerror v0.3.0
	github.com/giantswarm/micrologger v0.4.0
	k8s.io/api v0.21.3
	k8s.io/apiextensions-apiserver v0.21.3
	k8s.io/apimachinery v0.21.3
	k8s.io/client-go v0.21.3
	sigs.k8s.io/cluster-api v0.4.2
	sigs.k8s.io/cluster-api-provider-azure v0.5.2
	sigs.k8s.io/controller-runtime v0.9.6
)

replace sigs.k8s.io/cluster-api => sigs.k8s.io/cluster-api v0.4.2
