# GiantSwarm Sonobuoy plugin

Sonobuoy plugin that runs some tests as a sonobuoy plugin.

## Running the plugin

The tests that will be executed by the plugin need kubeconfigs to access the Control Plane Cluster, and the Tenant Cluster.
**So these cluster need to exist before running the tests.**

The required kubeconfigs are passed as environment variables. We can generate them from our local kubeconfig file.
For example, if our kubeconfig points to the Control Plane cluster, we can generate the Control Plane kubeconfig

```bash
kubectl config view --flatten=true --minify > cp_kubeconfig.yaml
```

We just need to do the same for the Tenant Cluster kubeconfig.

Then we are ready to run the tests.

```bash
sonobuoy run \                                                                                                                                                                         [giantswarm-godsmack:default]
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
sonobuoy status
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
