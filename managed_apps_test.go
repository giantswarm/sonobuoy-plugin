package sonobuoy_plugin

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync"
	"testing"

	"github.com/giantswarm/micrologger"

	"github.com/giantswarm/sonobuoy-plugin/v5/pkg/apputil"
	"github.com/giantswarm/sonobuoy-plugin/v5/pkg/ctrlclient"
)

// Test_ManagedApps test the main giantswarm managed apps install successfully
func Test_ManagedApps(t *testing.T) {
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

	apps := map[string][]apputil.AppConfig{
		"phoenix": {
			apputil.AppConfig{Name: "aws-load-balancer-controller"},
			apputil.AppConfig{Name: "karpenter"},
			apputil.AppConfig{Name: "aws-efs-csi-driver"},
		},
		"cabbage": {
			apputil.AppConfig{Name: "kong-app"},
			// apputil.AppConfig{Name: "ingress-nginx"}, // tested as part of the "ingress" test.
			// apputil.AppConfig{Name: "cloudflared"}, // failed (requires custom value)
		},
		"teddyfriends": {
			apputil.AppConfig{Name: "k8s-initiator-app", Catalog: "giantswarm-playground"}, // PSS failing
		},
		"atlas": {
			apputil.AppConfig{Name: "fluent-logshipping-app"},
			apputil.AppConfig{Name: "keda"},
			apputil.AppConfig{Name: "grafana"},
			apputil.AppConfig{Name: "loki"},
			apputil.AppConfig{Name: "datadog"},
		},
		"shield": {
			apputil.AppConfig{Name: "starboard-exporter"},
		},
		"honeybadger": {
			// apputil.AppConfig{Name: "flux-app" }, // failed PSS
			// apputil.AppConfig{Name: "external-secrets"}, // failed PSS
		},
		"bigmac": {
			apputil.AppConfig{Name: "athena"},
			apputil.AppConfig{Name: "dex-app"},
			apputil.AppConfig{Name: "rbac-bootstrap"},
		},
	}

	failed := make([]string, 0)

	var wg sync.WaitGroup
	var mutex sync.Mutex

	markFailed := func(msg string) {
		mutex.Lock()
		failed = append(failed, msg)
		mutex.Unlock()
	}

	for team, teamApps := range apps {
		for _, appCfg := range teamApps {
			wg.Add(1)
			go func(appCfg apputil.AppConfig, team string) {
				defer wg.Done()
				app, err := apputil.GetApp(clusterID, appCfg)
				if err != nil {
					logger.Debugf(ctx, "[%s] Error getting CR for app %q: %s", team, appCfg.Name, err)
					markFailed(fmt.Sprintf("%s (team %s)", appCfg.Name, team))
					return
				}

				err = apputil.InstallAndWait(ctx, logger, cpCtrlClient, app)
				if err != nil {
					markFailed(fmt.Sprintf("%s (team %s)", app.Name, team))
				}

				// Delete app.
				err = cpCtrlClient.Delete(ctx, app)
				if err != nil && !IsNotFound(err) {
					logger.Debugf(ctx, "[%s] Error deleting app %q: %s", team, app.Name, err)
				}
			}(appCfg, team)
		}
	}

	wg.Wait()

	if len(failed) > 0 {
		logger.Debugf(ctx, fmt.Sprintf("%d apps failed deploying: %s", len(failed), strings.Join(failed, ", ")))
		t.Fatalf("%d apps failed deploying", len(failed))
	}
}
