package sonobuoy_plugin

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	appv1alpha1 "github.com/giantswarm/apiextensions-application/api/v1alpha1"
	"github.com/giantswarm/backoff"
	"github.com/giantswarm/microerror"
	"github.com/giantswarm/micrologger"
	"github.com/google/go-github/v45/github"
	"golang.org/x/oauth2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/giantswarm/sonobuoy-plugin/v5/pkg/ctrlclient"
)

type appConfig struct {
	Name    string
	Catalog string
}

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

	apps := map[string][]appConfig{
		"phoenix": {
			appConfig{Name: "aws-load-balancer-controller"},
			appConfig{Name: "karpenter"},
			appConfig{Name: "aws-efs-csi-driver"},
		},
		"cabbage": {
			appConfig{Name: "kong-app"},
			appConfig{Name: "ingress-nginx"},
			//appConfig{Name: "cloudflared"}, // failed (requires custom value)
		},
		//"teddyfriends": {
		//	appConfig{Name: "k8s-initiator-app", Catalog: "giantswarm-playground"}, // PSS failing
		//	appConfig{Name: "opencost", Catalog: "giantswarm-playground"}, // removed by nick from the catalog https://github.com/giantswarm/giantswarm-playground-catalog/commit/33354c6cd8c2f7dd223dcb1e772abd0b669065fa
		//},
		"atlas": {
			appConfig{Name: "fluent-logshipping-app"},
			appConfig{Name: "keda"},
			appConfig{Name: "promtail"},
			appConfig{Name: "grafana"},
			appConfig{Name: "loki"},
			appConfig{Name: "datadog"},
		},
		"shield": {
			appConfig{Name: "jiralert"},
			appConfig{Name: "starboard-exporter"},
			appConfig{Name: "trivy"},
			appConfig{Name: "trivy-operator"},
			appConfig{Name: "falco"},
			appConfig{Name: "exception-recommender"},
		},
		//"honeybadger": {
		//	appConfig{Name: "flux-app" }, // failed PSS
		// 	appConfig{Name: "external-secrets"}, // failed PSS
		//},
		"bigmac": {
			appConfig{Name: "athena"},
			appConfig{Name: "dex-app"},
			appConfig{Name: "rbac-bootstrap"},
		},
	}

	failed := []string{}

	for team, teamApps := range apps {
		for _, appCfg := range teamApps {
			app, err := getApp(clusterID, appCfg)
			if err != nil {
				logger.Debugf(ctx, "[%s] Error getting CR for app %q: %s", team, appCfg.Name, err)
				failed = append(failed, fmt.Sprintf("%s (team %s)", team, appCfg.Name))
				continue
			}
			err = cpCtrlClient.Create(ctx, app)
			if err != nil {
				logger.Debugf(ctx, "[%s] Error creating App CR for %q: %s", team, app.Name, err)
				failed = append(failed, fmt.Sprintf("%s (team %s)", team, app.Name))
				continue
			}

			check := func(appName string) error {
				app := appv1alpha1.App{}
				err = cpCtrlClient.Get(ctx, client.ObjectKey{Namespace: clusterID, Name: appName}, &app)
				if err != nil {
					return microerror.Mask(err)
				}

				switch app.Status.Release.Status {
				case "failed":
					return microerror.Maskf(appFailedError, "[%s] App %q with version %q is in failed state.", team, app.Name, app.Spec.Version)
				case "deployed":
					logger.Debugf(ctx, "[%s] App %q with version %s deployed correctly.", team, app.Name, app.Spec.Version)

					// Delete app.
					err = cpCtrlClient.Delete(ctx, &app)
					if err != nil {
						logger.Debugf(ctx, "[%s] Error deleting app %q: %s", team, app.Name, err)
					}
				default:
					return microerror.Maskf(appNotReadyError, "[%s] App %q with version %q is still in state %q.", team, app.Name, app.Spec.Version, app.Status.Release.Status)
				}
				return nil
			}

			b := backoff.NewConstant(5*time.Minute, 30*time.Second)
			n := backoff.NewNotifier(logger, ctx)
			err = backoff.RetryNotify(func() error {
				return check(app.Name)
			}, b, n)
			if err != nil {
				failed = append(failed, fmt.Sprintf("%s (team %s)", app.Name, team))
				logger.Debugf(ctx, "[%s] Installation of %q app failed", team, app.Name)
			}

			// Delete app.
			err = cpCtrlClient.Delete(ctx, app)
			if err != nil && !IsNotFound(err) {
				logger.Debugf(ctx, "[%s] Error deleting app %q: %s", team, app.Name, err)
			}
		}
	}

	if len(failed) > 0 {
		logger.Debugf(ctx, strings.Join(failed, ", "))
		t.Fatalf("%d apps failed deploying", len(failed))
	}
}

func getApp(clusterID string, appCfg appConfig) (*appv1alpha1.App, error) {
	version, err := getLatestGithubRelease("giantswarm", appCfg.Name)
	if err != nil {
		return nil, microerror.Mask(err)
	}

	if appCfg.Catalog == "" {
		appCfg.Catalog = "giantswarm"
	}

	return &appv1alpha1.App{
		ObjectMeta: metav1.ObjectMeta{
			Name:      appCfg.Name,
			Namespace: clusterID,
			Labels: map[string]string{
				"giantswarm.io/cluster": clusterID,
			},
		},
		Spec: appv1alpha1.AppSpec{
			Catalog: appCfg.Catalog,
			ExtraConfigs: []appv1alpha1.AppExtraConfig{
				{
					Kind:      "configMap",
					Name:      "psp-removal-patch",
					Namespace: clusterID,
					Priority:  150,
				},
			},
			Name:      appCfg.Name,
			Namespace: appCfg.Name,
			Version:   version,
		},
	}, nil
}

func getLatestGithubRelease(owner string, name string) (string, error) {
	var tc *http.Client

	token := os.Getenv("OPSCTL_GITHUB_TOKEN")
	if token != "" {
		ctx := context.Background()
		ts := oauth2.StaticTokenSource(
			&oauth2.Token{AccessToken: token},
		)
		tc = oauth2.NewClient(ctx, ts)
	}

	cl := github.NewClient(tc)

	candidateNames := []string{name}
	if strings.HasSuffix(name, "-app") {
		candidateNames = append(candidateNames, strings.TrimSuffix(name, "-app"))
	} else {
		candidateNames = append(candidateNames, fmt.Sprintf("%s-app", name))
	}

	version := ""
	var latestErr error

	for _, n := range candidateNames {
		release, _, err := cl.Repositories.GetLatestRelease(context.Background(), owner, n)
		if IsGithubNotFound(err) {
			// try with next candidate
			latestErr = err
			continue
		} else if err != nil {
			return "", microerror.Mask(err)
		}

		version = *release.Name
		break
	}

	if version == "" {
		return "", microerror.Mask(latestErr)
	}

	return version, nil
}
