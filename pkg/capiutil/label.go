package capiutil

import (
	"time"

	"github.com/giantswarm/backoff"
	"github.com/giantswarm/microerror"
	capi "sigs.k8s.io/cluster-api/api/v1alpha3"
)

const E2ENodepool = "e2e"

func WaitForLabelSet(clusterGetter func(name string) TestedObject, clusterID string, label string, cluster TestedObject) error {
	o := func() error {
		cluster = clusterGetter(clusterID).(*capi.Cluster)
		if cluster.GetLabels()[label] == "" {
			// Still no label set.
			return microerror.Maskf(labelNotSetError, "Cluster CR still does not have value for label %q", label)
		}

		return nil
	}

	// Wait for cluster CR to have a value for the release label.
	b := backoff.NewConstant(backoff.ShortMaxWait, 10*time.Second)
	err := backoff.Retry(o, b)
	if err != nil {
		return microerror.Mask(err)
	}

	return nil
}
