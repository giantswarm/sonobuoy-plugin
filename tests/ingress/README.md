# Ingress

This test creates a Pod in the tenant cluster namespace in the Control Plane cluster that tries to send an HTTP request to a ["hello world" app](https://github.com/giantswarm/loadtest-app) running in the tenant cluster.
The [app](https://github.com/giantswarm/loadtest-app) is installed in the tenant cluster together with the [nginx ingress controller app](https://github.com/giantswarm/nginx-ingress-controller-app), so that it can receive traffic from outside the cluster.
