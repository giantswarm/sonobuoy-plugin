# Control Plane to Tenant Cluster connectivity

This test checks that there is connectivity between the CP and the TC k8s API.
It creates a Pod in the tenant cluster namespace in the Control Plane cluster that sends an HTTP request to the tenant cluster k8s API.
