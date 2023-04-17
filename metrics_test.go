package sonobuoy_plugin

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
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

func Test_Metrics(t *testing.T) {
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

	// List of metrics that must be present.
	metrics := []string{
		// API server metrics in prometheus-rules
		"apiserver_flowcontrol_dispatched_requests_total",
		"apiserver_flowcontrol_request_concurrency_limit",
		"apiserver_request_duration_seconds_bucket",
		"apiserver_admission_webhook_rejection_count",
		"apiserver_admission_webhook_admission_duration_seconds_sum",
		"apiserver_admission_webhook_admission_duration_seconds_count",
		"apiserver_request_total",
		"apiserver_audit_event_total",

		// Kubelet
		"kube_node_status_condition",
		"kube_node_spec_unschedulable",
		"kube_node_created",

		// Controller manager
		"workqueue_queue_duration_seconds_count",
		"workqueue_queue_duration_seconds_bucket",

		// Scheduler
		"scheduler_pod_scheduling_duration_seconds_count",
		"scheduler_pod_scheduling_duration_seconds_bucket",

		// ETCD
		"etcd_request_duration_seconds_count",
		"etcd_request_duration_seconds_bucket",

		// Coredns"
		"coredns_dns_request_duration_seconds_count",
		"coredns_dns_request_duration_seconds_bucket",
	}

	// Wait for all queries to be compliant with expectations.
	{
		o := func() error {
			kc, err := ctrlclient.GetCPKubeConfig()
			if err != nil {
				t.Fatal(err)
			}

			for _, metric := range metrics {
				query := fmt.Sprintf("absent(%s) or vector(0)", metric)
				stdout, _, err := podrunner.ExecInPod(ctx, logger, podName, namespace, "prometheus", []string{"wget", "-q", "-O-", fmt.Sprintf("prometheus-operated.%s-prometheus:9090/%s/api/v1/query?query=%s", clusterID, clusterID, url.QueryEscape(query))}, kc)
				if err != nil {
					return microerror.Maskf(podExecError, "Can't exec command in pod %s: %s.", podName, err)
				}

				// {"status":"success","data":{"resultType":"vector","result":[{"metric":{},"value":[1681718763.145,"1"]}]}}

				type result struct {
					Value []any
				}

				response := struct {
					Status string
					Data   struct {
						ResultType string
						Result     []result
					}
				}{}

				err = json.Unmarshal([]byte(stdout), &response)
				if err != nil {
					return microerror.Maskf(unexpectedAnswerError, "Can't parse prometheus query output: %s", err)
				}

				if response.Status != "success" {
					return microerror.Maskf(prometheusQueryError, "Unexpected response status %s when running query %q", response.Status, query)
				}

				if response.Data.ResultType != "vector" {
					return microerror.Maskf(prometheusQueryError, "Unexpected response type %s when running query %q (wanted vector)", response.Status, query)
				}

				if len(response.Data.Result) != 1 {
					return microerror.Maskf(prometheusQueryError, "Unexpected count of results when running query %q (wanted 1, got %d)", query, len(response.Data.Result))
				}

				// Second field of first result is the metric value. [1681718763.145,"1"] => "1"
				str, ok := (response.Data.Result[0].Value[1]).(string)
				if !ok {
					return microerror.Maskf(prometheusQueryError, "Cannot cast result value to string when running query %q", query)
				}
				if str != "0" {
					return microerror.Maskf(prometheusQueryError, "Unexpected value for query %q (wanted '0', got %q)", query, str)
				}

				logger.Debugf(ctx, "Metric %q was found", metric)
			}

			return nil
		}

		b := backoff.NewConstant(backoff.LongMaxWait, 1*time.Minute)
		n := backoff.NewNotifier(logger, ctx)
		err = backoff.RetryNotify(o, b, n)
		if err != nil {
			t.Fatalf("Error waiting for prometheus metrics to be present.")
		}
	}
}
