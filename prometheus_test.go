package sonobuoy_plugin

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/blang/semver"
	"github.com/giantswarm/apiextensions/v3/pkg/label"
	"github.com/giantswarm/backoff"
	"github.com/giantswarm/microerror"
	"github.com/giantswarm/micrologger"
	corev1 "k8s.io/api/core/v1"
	capiv1beta1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/giantswarm/sonobuoy-plugin/v5/pkg/capiutil"
	"github.com/giantswarm/sonobuoy-plugin/v5/pkg/ctrlclient"
	"github.com/giantswarm/sonobuoy-plugin/v5/pkg/podrunner"
)

func Test_Prometheus(t *testing.T) {
	t.Parallel()

	var err error

	ctx := context.Background()

	cpCtrlClient, err := ctrlclient.CreateCPCtrlClient()
	if err != nil {
		t.Fatalf("error creating CP k8s client: %v", err)
	}

	regularLogger, err := micrologger.New(micrologger.Config{})
	if err != nil {
		t.Fatal(err)
	}

	logger := NewTestLogger(regularLogger, t)

	clusterID, exists := os.LookupEnv("CLUSTER_ID")
	if !exists {
		t.Fatal("missing CLUSTER_ID environment variable")
	}

	// Get Cluster.
	var cluster *capiv1beta1.Cluster
	{
		cluster, err = capiutil.FindCluster(ctx, cpCtrlClient, clusterID)
		if err != nil {
			t.Fatal(err)
		}
	}

	// Ensure v19+ release.
	{
		releaseName := cluster.GetLabels()[label.ReleaseVersion]
		if releaseName == "" {
			t.Fatalf("Can't get value for label %s from Cluster CR %s", label.ReleaseVersion, cluster.Name)
		}

		rel, err := semver.ParseTolerant(releaseName)
		if err != nil {
			t.Fatal(err)
		}

		if rel.Major < 19 {
			logger.Debugf(ctx, "Release %s does not support prometheus test.", releaseName)
			return
		}
	}

	namespace := fmt.Sprintf("%s-prometheus", clusterID)
	podName := fmt.Sprintf("prometheus-%s-0", clusterID)

	logger.Debugf(ctx, "Waiting for prometheus namespace %q to exist", namespace)

	// Wait for prometheus namespace to exist.
	{
		o := func() error {
			ns := &corev1.Namespace{}
			err = cpCtrlClient.Get(ctx, client.ObjectKey{Name: namespace}, ns)
			if err != nil {
				return microerror.Mask(err)
			}

			return nil
		}

		b := backoff.NewConstant(backoff.LongMaxWait, 1*time.Minute)
		n := backoff.NewNotifier(logger, ctx)
		err = backoff.RetryNotify(o, b, n)
		if err != nil {
			t.Fatalf("Error waiting for prometheus namespace to exist.")
		}
	}

	logger.Debugf(ctx, "Waiting for prometheus targets to be up")

	// Wait for all targets to be "Up".
	{
		o := func() error {
			kc, err := ctrlclient.GetCPKubeConfig()
			if err != nil {
				t.Fatal(err)
			}

			stdout, _, err := podrunner.ExecInPod(ctx, logger, podName, namespace, "prometheus", []string{"wget", "-q", "-O-", fmt.Sprintf("prometheus-operated.%s-prometheus:9090/%s/api/v1/targets", clusterID, clusterID)}, kc)
			if err != nil {
				return microerror.Maskf(podExecError, "Can't exec command in pod %s: %s.", podName, err)
			}

			type target struct {
				Health string
				Labels map[string]string
			}

			response := struct {
				Data struct {
					ActiveTargets []target
				}
			}{}

			err = json.Unmarshal([]byte(stdout), &response)
			if err != nil {
				return microerror.Maskf(unexpectedAnswerError, "Can't parse prometheus targets output: %s", err)
			}

			down := make([]string, 0)
			for _, target := range response.Data.ActiveTargets {
				if target.Health != "up" {
					down = append(down, fmt.Sprintf("%s (Health = %q)", target.Labels["job"], target.Health))
				}
			}

			if len(down) > 0 {
				return microerror.Maskf(targetDownError, "%d target down: %v", len(down), down)
			}

			return nil
		}

		b := backoff.NewConstant(backoff.LongMaxWait, 1*time.Minute)
		n := backoff.NewNotifier(logger, ctx)
		err = backoff.RetryNotify(o, b, n)
		if err != nil {
			t.Fatalf("Error waiting for prometheus targets to be healthy.")
		}
	}
}
