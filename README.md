# GiantSwarm Sonobuoy plugin

Sonobuoy plugin that runs some tests as a sonobuoy plugin.

## Running the plugin

The tests that will be executed by the plugin need kubeconfigs to access the Management Cluster and the Workload Cluster.
**These clusters need to exist before running the tests.**

The required kubeconfigs are passed as environment variables. We can generate them from our local kubeconfig file.
For example, if our kubeconfig points to the Control Plane cluster, we can generate the Control Plane kubeconfig

```bash
kubectl config view --flatten=true --minify > cp_kubeconfig.yaml
```

We just need to do the same for the Tenant Cluster kubeconfig.

Then we are ready to run the tests.

```bash
sonobuoy run \
    --kubeconfig "cp_kubeconfig.yaml" \
    --namespace "4zxet-sonobuoy" \
    --plugin https://raw.githubusercontent.com/giantswarm/sonobuoy-plugin/master/giantswarm-plugin.yaml \
    --plugin-env giantswarm.TC_KUBECONFIG="$(cat "tc_kubeconfig.yaml")" \
    --plugin-env giantswarm.CP_KUBECONFIG="$(cat "cp_kubeconfig.yaml")" \
    --plugin-env giantswarm.CLUSTER_ID="4zxet" \
    --mode=certified-conformance \
    --wait
```

When this command finishes, we can see the results

```bash
sonobuoy status --namespace 4zxet-sonobuoy 
```

Or retrieve the logs

```bash
outfile=$(sonobuoy retrieve) && \
  mkdir results && tar -xf $outfile -C results &&
  cat results/plugins/giantswarm/results/global/out
```

## Tests

- [Control Plane to Tenant Cluster connectivity](./tests/cptcconnectivity/README.md)
- [Deploy hello world and ingress apps](./tests/ingress/README.md)
- [Custom resources](./tests/customresources/README.md)

### Custom Resources

Custom Resource tests are checking values from following Cluster API and Cluster API Azure CRs:

- Cluster
- MachinePool
- AzureCluster
- AzureMachinePool

#### Cluster

Metadata checks:

- `release.giantswarm.io/version` label is set
- `azure-operator.giantswarm.io/version` label is set
- `release.giantswarm.io/last-deployed-version` annotation is set
- `release.giantswarm.io/version` label matches `release.giantswarm.io/last-deployed-version` annotation

Status checks:

- `Cluster.Status.ControlPlaneInitialized` is set to `true`
- `Cluster.Status.ControlPlaneReady` is set to `true`
- `Cluster.Status.InfrastructureReady` is set to `true`
- Waiting for `Cluster.Status.Conditions[Ready]` to have status `True`
- Waiting for `Cluster.Status.Conditions[Creating]` to have Status `False`
- `Cluster.Status.Conditions[Creating]` have Reason `CreationCompleted`
- Waiting for `Cluster.Status.Conditions[Upgrading]` to have Status `False`
- `Cluster.Status.Conditions[ControlPlaneReady]` have Status `True`
- `Cluster.Status.Conditions[InfrastructureReady]` have Status `True`
- `Cluster.Status.Conditions[NodePoolsReady]` have Status `True`

#### MachinePool

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
- Owner reference is set to Cluster object

Status checks:

- `Status.Replicas` is withing defined cluster autoscaler min and max values
- `Status.Replicas` equal to `Status.ReadyReplicas`
- Waiting for `Cluster.Status.Conditions[Ready]` to have status `True`
- Waiting for `Cluster.Status.Conditions[Creating]` to have Status `False`
- Waiting for `Cluster.Status.Conditions[Upgrading]` to have Status `False`
- `Cluster.Status.Conditions[InfrastructureReady]` have Status `True`
- `Cluster.Status.Conditions[ReplicasReady]` have Status `True`

#### AzureCluster

Metadata checks:

- `release.giantswarm.io/version` label is set
- `azure-operator.giantswarm.io/version` label is set
- AzureCluster and Cluster have matching `release.giantswarm.io/version` labels
- AzureCluster and Cluster have matching `azure-operator.giantswarm.io/version` labels
- Owner reference is set to Cluster object

Spec checks:

- Exactly 1 CIDR block is allocated in `AzureCluster.Spec.NetworkSpec.Vnet.CIDRBlocks`
- Number of subnets allocated in `AzureCluster.Spec.NetworkSpec.Subnets` is equal to the number of MachinePool objects
  for the cluster
- Every subnet in `AzureCluster.Spec.NetworkSpec.Subnets` has a name equal to MachinePool object name
- For every subnet in `AzureCluster.Spec.NetworkSpec.Subnets`, exactly 1 CIDR block is allocated

Status checks:

- Waiting for `Cluster.Status.Conditions[Ready]` to have status `True`
- `AzureCluster.Status.Ready` is set to `true`

#### AzureMachinePool

Metadata checks:

- `giantswarm.io/machine-pool` label is set
- `release.giantswarm.io/version` label is set
- `azure-operator.giantswarm.io/version` label is set
- AzureMachinePool and Cluster have matching `release.giantswarm.io/version` labels
- AzureMachinePool and Cluster have matching `azure-operator.giantswarm.io/version` labels
- AzureMachinePool and MachinePool have matching `giantswarm.io/machine-pool` labels
- Owner reference is set to MachinePool object

Spec checks:

- `AzureMachinePool.Spec.ProviderID` is set
- `AzureMachinePool.Spec.ProviderIDList` has number of IDs equal to the number of discovered replicas
  in `MachinePool.Status.Replicas`

Status checks:

- Waiting for `Cluster.Status.Conditions[Ready]` to have status `True`
- `AzureMachinePool.Status.Replicas` is equal to `MachinePool.Status.Replicas`
- `AzureMachinePool.Status.ProvisioningState` is set to `Succeeded`
- `AzureMachinePool.Status.Ready` is set to `true`
