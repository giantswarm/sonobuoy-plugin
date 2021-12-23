module github.com/giantswarm/sonobuoy-plugin/v5

go 1.15

require (
	github.com/Azure/azure-sdk-for-go v46.4.0+incompatible
	github.com/Azure/go-autorest/autorest v0.11.7
	github.com/Azure/go-autorest/autorest/azure/auth v0.5.2
	github.com/Azure/go-autorest/autorest/to v0.4.0
	github.com/aws/aws-sdk-go v1.27.0
	github.com/ghodss/yaml v1.0.0
	github.com/giantswarm/apiextensions/v3 v3.39.0
	github.com/giantswarm/apptest v0.9.0
	github.com/giantswarm/backoff v0.2.0
	github.com/giantswarm/microerror v0.3.0
	github.com/giantswarm/micrologger v0.4.0
	github.com/imdario/mergo v0.3.12 // indirect
	github.com/onsi/gomega v1.14.0 // indirect
	github.com/prometheus/client_golang v1.11.0 // indirect
	go.uber.org/zap v1.17.0 // indirect
	golang.org/x/net v0.0.0-20210716203947-853a461950ff // indirect
	k8s.io/api v0.18.19
	k8s.io/apiextensions-apiserver v0.18.19
	k8s.io/apimachinery v0.18.19
	k8s.io/client-go v0.18.19
	k8s.io/klog/v2 v2.1.0 // indirect
	k8s.io/kube-openapi v0.0.0-20200410145947-bcb3869e6f29 // indirect
	k8s.io/utils v0.0.0-20210709001253-0e1f9d693477 // indirect
	sigs.k8s.io/cluster-api v0.3.22
	sigs.k8s.io/cluster-api-provider-azure v0.4.9
	sigs.k8s.io/controller-runtime v0.6.4
)

replace (
	sigs.k8s.io/cluster-api => github.com/giantswarm/cluster-api v0.3.10-gs
	sigs.k8s.io/cluster-api-provider-azure => github.com/giantswarm/cluster-api-provider-azure v0.4.9-gsalpha2
)
