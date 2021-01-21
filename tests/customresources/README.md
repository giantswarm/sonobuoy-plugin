# Custom Resource

Custom Resource tests are checking values from following Cluster API and Cluster API Azure CRs:

- Cluster
- MachinePool
- AzureCluster
- AzureMachinePool

## Cluster

Metadata checks:

- `release.giantswarm.io/version` label is set
- `azure-operator.giantswarm.io/version` label is set
- `release.giantswarm.io/last-deployed-version` annotation is set
- `release.giantswarm.io/version` label matches `release.giantswarm.io/last-deployed-version` annotation

Status checks:

- `Cluster.Status.ControlPlaneInitialized` is set to `true`
- `Cluster.Status.ControlPlaneReady` is set to `true`
- `Cluster.Status.InfrastructureReady` is set to `true`
- waiting for `Cluster.Status.Conditions[Ready]` to have status `True`
- waiting for `Cluster.Status.Conditions[Creating]` to have Status `False`
- `Cluster.Status.Conditions[Creating]` have Reason `CreationCompleted`
- waiting for `Cluster.Status.Conditions[Upgrading]` to have Status `False`
- `Cluster.Status.Conditions[ControlPlaneReady]` have Status `True`
- `Cluster.Status.Conditions[InfrastructureReady]` have Status `True`
- `Cluster.Status.Conditions[NodePoolsReady]` have Status `True`

## MachinePool

Metadata checks:

- `giantswarm.io/machine-pool` label is set
- `release.giantswarm.io/version` label is set
- Cluster and MachinePool have matching `release.giantswarm.io/version` labels
- `azure-operator.giantswarm.io/version` label is set
- Cluster and MachinePool have matching `azure-operator.giantswarm.io/version` labels
- `release.giantswarm.io/last-deployed-version` annotation is set
- Cluster and MachinePool have matching `release.giantswarm.io/last-deployed-version` annotations
- `cluster.k8s.io/cluster-api-autoscaler-node-group-min-size` annotation is set
- `cluster.k8s.io/cluster-api-autoscaler-node-group-max-size` annotation is set

Status checks:
- `Spec.Replicas` equal to `Status.Replicas`
- `Status.Replicas` equal to `Status.ReadyReplicas`
- waiting for `Cluster.Status.Conditions[Ready]` to have status `True`
- waiting for `Cluster.Status.Conditions[Creating]` to have Status `False`
- waiting for `Cluster.Status.Conditions[Upgrading]` to have Status `False`
- `Cluster.Status.Conditions[InfrastructureReady]` have Status `True`
- `Cluster.Status.Conditions[ReplicasReady]` have Status `True`
