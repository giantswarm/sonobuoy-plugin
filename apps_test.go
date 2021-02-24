package sonobuoy_plugin

import (
	"context"
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
	capiv1alpha3 "sigs.k8s.io/cluster-api/api/v1alpha3"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/giantswarm/sonobuoy-plugin/v5/pkg/ctrlclient"
)

// Test_Apps test the default release apps are installed and deployed successfully.
func Test_Apps(t *testing.T) {
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

	logger.Debugf(ctx, "Testing that all apps declared in the Release CR are deployed.")

	// Get Cluster.
	logger.Debugf(ctx, "Getting cluster CR.")
	var cluster capiv1alpha3.Cluster
	{
		clusters := &capiv1alpha3.ClusterList{}
		err = cpCtrlClient.List(ctx, clusters, client.MatchingLabels{label.Cluster: clusterID})
		if err != nil {
			t.Fatal(err)
		}

		if len(clusters.Items) != 1 {
			t.Fatalf("Expected 1 Cluster CR with %s = %s, %d found.", label.Cluster, clusterID, len(clusters.Items))
		}

		cluster = clusters.Items[0]
		logger.Debugf(ctx, "Got cluster CR.")
	}

	// Get release.
	release := &v1alpha1.Release{}
	{
		logger.Debugf(ctx, "Getting release CR.")
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
		logger.Debugf(ctx, "Got release CR.")
	}

	o := func() error {
		appList := &appv1alpha1.AppList{}
		err = cpCtrlClient.List(ctx, appList, client.InNamespace(clusterID))
		if err != nil {
			t.Fatal(err)
		}

		existingApps := map[string]appv1alpha1.App{}
		for _, app := range appList.Items {
			existingApps[app.Name] = app
		}

		for _, app := range release.Spec.Apps {
			logger.Debugf(ctx, "Checking app %s with version %s.", app.Name, app.Version)

			deployedApp, ok := existingApps[app.Name]
			if !ok {
				return microerror.Maskf(appNotReadyError, "App %#q was not found on the namespace %#q.", app.Name, clusterID)
			}

			if deployedApp.Status.Version != app.Version {
				return microerror.Maskf(appNotReadyError, "App %s not updated yet (version is %s, expected %s)", app.Name, deployedApp.Status.Version, app.Version)
			}

			switch deployedApp.Status.Release.Status {
			case "failed":
				t.Fatalf("App %s is in failed state", app.Name)
			case "deployed":
				logger.Debugf(ctx, "App %s with version %s deployed correctly.", app.Name, app.Version)
				continue
			default:
				return microerror.Maskf(appNotReadyError, "App %s with version %s is still in state %s.", app.Name, app.Version, deployedApp.Status.Release.Status)
				// Have to wait.
			}
		}

		return nil
	}

	b := backoff.NewConstant(backoff.MediumMaxWait, 1*time.Minute)
	n := backoff.NewNotifier(logger, ctx)
	err = backoff.RetryNotify(o, b, n)
	if err != nil {
		t.Fatalf("Error waiting for apps to be deployed.")
	}

}
