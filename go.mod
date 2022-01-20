module github.com/giantswarm/sonobuoy-plugin/v5

go 1.15

require (
	github.com/Azure/azure-sdk-for-go v58.1.0+incompatible
	github.com/Azure/go-autorest/autorest v0.11.21
	github.com/Azure/go-autorest/autorest/azure/auth v0.5.8
	github.com/Azure/go-autorest/autorest/to v0.4.0
	github.com/aws/aws-sdk-go v1.27.0
	github.com/ghodss/yaml v1.0.0
	github.com/giantswarm/apiextensions/v3 v3.39.0
	github.com/giantswarm/apptest v0.9.0
	github.com/giantswarm/backoff v0.2.0
	github.com/giantswarm/microerror v0.3.0
	github.com/giantswarm/micrologger v0.4.0
	k8s.io/api v0.23.1
	k8s.io/apiextensions-apiserver v0.23.1
	k8s.io/apimachinery v0.23.1
	k8s.io/client-go v0.23.1
	sigs.k8s.io/cluster-api v0.4.5
	sigs.k8s.io/cluster-api-provider-azure v0.5.3
	sigs.k8s.io/controller-runtime v0.10.3
)

replace (
	github.com/go-logr/logr => github.com/go-logr/logr v0.4.0
	k8s.io/klog/v2 => k8s.io/klog/v2 v2.10.0
	sigs.k8s.io/cluster-api => sigs.k8s.io/cluster-api v0.4.5
)
