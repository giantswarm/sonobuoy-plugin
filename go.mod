module github.com/giantswarm/azure-sonobuoy/v5

go 1.15

require (
	github.com/giantswarm/apptest v0.8.0
	github.com/giantswarm/backoff v0.2.0
	github.com/giantswarm/microerror v0.3.0
	github.com/giantswarm/micrologger v0.4.0
	k8s.io/api v0.18.9
	k8s.io/apiextensions-apiserver v0.18.9
	k8s.io/apimachinery v0.18.9
	k8s.io/client-go v0.18.9
	sigs.k8s.io/cluster-api v0.3.11
	sigs.k8s.io/cluster-api-provider-azure v0.4.9
	sigs.k8s.io/controller-runtime v0.6.3
)

replace (
	sigs.k8s.io/cluster-api => github.com/giantswarm/cluster-api v0.3.10-gs
	sigs.k8s.io/cluster-api-provider-azure => github.com/giantswarm/cluster-api-provider-azure v0.4.9-gsalpha2
)
