package sonobuoy_plugin

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/giantswarm/backoff"
	"github.com/giantswarm/microerror"
	"github.com/giantswarm/micrologger"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

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

		b := backoff.NewConstant(backoff.MediumMaxWait, 1*time.Minute)
		n := backoff.NewNotifier(logger, ctx)
		err = backoff.RetryNotify(o, b, n)
		if err != nil {
			t.Fatalf("Error waiting for prometheus namespace to exist.")
		}
	}

	logger.Debugf(ctx, "Waiting for prometheus pod %q to be running", podName)

	// Wait for prometheus to be running.
	{
		o := func() error {
			pod := &corev1.Pod{}

			err = cpCtrlClient.Get(ctx, client.ObjectKey{Name: podName, Namespace: namespace}, pod)
			if err != nil {
				return microerror.Mask(err)
			}

			for _, cs := range pod.Status.ContainerStatuses {
				if !cs.Ready {
					return microerror.Maskf(podNotReadyError, "Container %s in pod %s is not ready", cs.Name, podName)
				}
			}

			return nil
		}

		b := backoff.NewConstant(backoff.MediumMaxWait, 1*time.Minute)
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
				t.Fatalf("Can't exec command in pod %s: %s.", podName, err)
			}

			type target struct {
				Health           string
				DiscoveredLabels map[string]string
			}

			response := struct {
				Data struct {
					ActiveTargets []target
				}
			}{}

			err = json.Unmarshal([]byte(stdout), &response)
			if err != nil {
				t.Fatalf("Can't parse prometheus targets output: %s", err)
			}

			for _, target := range response.Data.ActiveTargets {
				if target.Health != "up" {
					return microerror.Maskf(targetDownError, "Target %s is not Up (Health = %q)", target.DiscoveredLabels["job"], target.Health)
				}
			}

			return nil
		}

		b := backoff.NewConstant(backoff.MediumMaxWait, 1*time.Minute)
		n := backoff.NewNotifier(logger, ctx)
		err = backoff.RetryNotify(o, b, n)
		if err != nil {
			t.Fatalf("Error waiting for prometheus namespace to exist.")
		}
	}
}
