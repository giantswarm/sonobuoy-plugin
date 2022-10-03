package sonobuoy_plugin

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	appv1alpha1 "github.com/giantswarm/apiextensions/v3/pkg/apis/application/v1alpha1"
	"github.com/giantswarm/apiextensions/v3/pkg/apis/release/v1alpha1"
	"github.com/giantswarm/apiextensions/v3/pkg/label"
	"github.com/giantswarm/backoff"
	"github.com/giantswarm/microerror"
	"github.com/giantswarm/micrologger"
	v1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	capiv1beta1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/giantswarm/sonobuoy-plugin/v5/pkg/capiutil"
	"github.com/giantswarm/sonobuoy-plugin/v5/pkg/ctrlclient"
	"github.com/giantswarm/sonobuoy-plugin/v5/pkg/podrunner"
)

const (
	ciliumAppName     = "cilium"
	ciliumDsName      = "cilium"
	ciliumDsNamespace = "kube-system"
)

// Test_Cilium test the cilium app comes up healthy.
func Test_Cilium(t *testing.T) {
	t.Parallel()

	var err error

	ctx := context.Background()

	tcCtrlClient, err := ctrlclient.CreateTCCtrlClient()
	if err != nil {
		t.Fatalf("error creating TC k8s client: %v", err)
	}

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

	// Get release.
	release := &v1alpha1.Release{}
	{
		releaseName := cluster.GetLabels()[label.ReleaseVersion]
		if releaseName == "" {
			t.Fatalf("Can't get value for label %s from Cluster CR %s", label.ReleaseVersion, cluster.Name)
		}

		if !strings.HasPrefix(releaseName, "v") {
			releaseName = fmt.Sprintf("v%s", releaseName)
		}

		err = cpCtrlClient.Get(ctx, client.ObjectKey{Name: releaseName}, release)
		if err != nil {
			t.Fatal(err)
		}
	}

	// Check if Cilium is included in the release.
	desiredVersion := ""
	for _, app := range release.Spec.Apps {
		if app.Name == ciliumAppName {
			desiredVersion = app.Version
			break
		}
	}

	if desiredVersion == "" {
		logger.Debugf(ctx, "Release %s does not include cilium.", release.Name)
		return
	}

	// Wait for cilium app to be deployed.
	o := func() error {
		deployedApp := &appv1alpha1.App{}
		err = cpCtrlClient.Get(ctx, client.ObjectKey{Namespace: clusterID, Name: ciliumAppName}, deployedApp)
		if err != nil {
			return microerror.Maskf(appNotReadyError, "App %s does not exist", ciliumAppName)
		}

		if deployedApp.Status.Version != desiredVersion {
			return microerror.Maskf(appNotReadyError, "App %s not updated yet (version is %s, expected %s)", ciliumAppName, deployedApp.Status.Version, desiredVersion)
		}

		switch deployedApp.Status.Release.Status {
		case "failed":
			t.Fatalf("App %s is in failed state", ciliumAppName)
		case "deployed":
			logger.Debugf(ctx, "App %s with version %s deployed correctly.", ciliumAppName, desiredVersion)
		default:
			return microerror.Maskf(appNotReadyError, "App %s with version %s is still in state %s.", ciliumAppName, desiredVersion, deployedApp.Status.Release.Status)
			// Have to wait.
		}

		return nil
	}

	b := backoff.NewConstant(backoff.MediumMaxWait, 1*time.Minute)
	n := backoff.NewNotifier(logger, ctx)
	err = backoff.RetryNotify(o, b, n)
	if err != nil {
		t.Fatalf("Error waiting for apps to be deployed.")
	}

	labelSelector := metav1.LabelSelector{}

	// Wait for cilium daemonset to be satisfied
	o = func() error {
		ds := &v1.DaemonSet{}

		err = tcCtrlClient.Get(ctx, client.ObjectKey{Namespace: ciliumDsNamespace, Name: ciliumDsName}, ds)
		if err != nil {
			t.Fatalf("Error waiting for ds to be satisfied.")
		}

		if ds.Status.DesiredNumberScheduled != ds.Status.NumberAvailable {
			return microerror.Maskf(dsNotReadyError, "DS %s has %d replicas ready but wants %d.", ciliumDsName, ds.Status.NumberAvailable, ds.Status.DesiredNumberScheduled)
		}

		logger.Debugf(ctx, "%d out of %d pods ready and available for DS %s.", ds.Status.NumberAvailable, ds.Status.DesiredNumberScheduled, ciliumDsName)

		labelSelector = *ds.Spec.Selector

		return nil
	}

	err = backoff.RetryNotify(o, b, n)
	if err != nil {
		t.Fatalf("Error waiting for ds to be satisfied.")
	}

	// Check cilium status from inside one of the cilium pods.
	pods := &corev1.PodList{}

	err = tcCtrlClient.List(ctx, pods, client.MatchingLabels(labelSelector.MatchLabels))
	if err != nil || len(pods.Items) == 0 {
		t.Fatalf("Error looking up cilium pod to exec into.")
	}

	pod := pods.Items[0]

	kc, err := ctrlclient.GetTCKubeConfig()
	if err != nil {
		t.Fatal(err)
	}

	stdout, _, err := podrunner.ExecInPod(ctx, logger, pod.Name, ciliumDsNamespace, "cilium-agent", []string{"cilium", "status", "-o", "json"}, kc)
	if err != nil {
		t.Fatalf("Can't exec command in cilium pod %s.", pod.Name)
	}

	response := struct {
		Cilium struct {
			State string // Should be 'Ok'
		}
		Cluster struct {
			CiliumHealth struct {
				State string // Should be 'Ok'
			}
		}
	}{}

	err = json.Unmarshal([]byte(stdout), &response)
	if err != nil {
		t.Fatalf("Can't exec command in cilium pod %s.", pod.Name)
	}

	if response.Cilium.State != "Ok" {
		t.Fatalf("Expected `cilium status -o json` to give 'Ok' under 'cilium.state', got %s.", response.Cilium.State)
	}

	if response.Cluster.CiliumHealth.State != "Ok" {
		t.Fatalf("Expected `cilium status -o json` to give 'Ok' under 'cilium.cluster.ciliumHealth.state', got %s.", response.Cluster.CiliumHealth.State)
	}
}
