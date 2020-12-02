# Ingress

This test creates a Pod in the Control Plane that tries to send an HTTP request to a "hello world" app running in the TC.
The app is installed in the TC together with the nginx ingress controller app, so that it can receive traffic from outside the cluster.
