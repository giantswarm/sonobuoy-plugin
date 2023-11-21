package apputil

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	appv1alpha1 "github.com/giantswarm/apiextensions-application/api/v1alpha1"
	"github.com/giantswarm/backoff"
	"github.com/giantswarm/microerror"
	"github.com/giantswarm/micrologger"
	"github.com/google/go-github/v45/github"
	"golang.org/x/oauth2"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type AppConfig struct {
	Catalog    string
	Name       string
	Namespace  string
	ValuesYAML string
}

func InstallAndWait(ctx context.Context, logger micrologger.Logger, ctrlClient client.Client, app *appv1alpha1.App) error {
	err := ctrlClient.Create(ctx, app)
	if err != nil {
		return microerror.Mask(err)
	}

	check := func(ok client.ObjectKey) error {
		app := appv1alpha1.App{}
		err = ctrlClient.Get(ctx, ok, &app)
		if err != nil {
			logger.Debugf(ctx, "Error creating App CR for %q: %s", app.Name, err)
			return microerror.Mask(err)
		}

		switch app.Status.Release.Status {
		case "failed":
			return microerror.Maskf(appFailedError, "App %q with version %q is in failed state: %s", app.Name, app.Spec.Version, app.Status.Release.Reason)
		case "deployed":
			logger.Debugf(ctx, "App %q with version %s deployed correctly.", app.Name, app.Spec.Version)

			// Delete app.
			err = ctrlClient.Delete(ctx, &app)
			if err != nil {
				logger.Debugf(ctx, "Error deleting app %q: %s", app.Name, err)
			}
		default:
			return microerror.Maskf(appNotReadyError, "App %q with version %q is still in state %q.", app.Name, app.Spec.Version, app.Status.Release.Status)
		}
		return nil
	}

	b := backoff.NewConstant(10*time.Minute, 30*time.Second)
	n := backoff.NewNotifier(logger, ctx)
	err = backoff.RetryNotify(func() error {
		return check(client.ObjectKey{Name: app.Name, Namespace: app.Namespace})
	}, b, n)
	if err != nil {
		logger.Debugf(ctx, "Installation of %q app failed", app.Name)
		return microerror.Mask(err)
	}

	return nil
}

func CreateAppConfigCM(ctx context.Context, logger micrologger.Logger, ctrlClient client.Client, clusterID string, appCfg AppConfig) (*v1.ConfigMap, error) {
	cm := v1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      getCmName(appCfg),
			Namespace: clusterID,
			Labels: map[string]string{
				"giantswarm.io/cluster": clusterID,
			},
		},
		Data: map[string]string{
			"values": appCfg.ValuesYAML,
		},
	}

	err := ctrlClient.Create(ctx, &cm)
	if err != nil {
		return nil, microerror.Mask(err)
	}

	return &cm, nil
}

func GetApp(clusterID string, appCfg AppConfig) (*appv1alpha1.App, error) {
	version, err := getLatestGithubRelease("giantswarm", appCfg.Name)
	if err != nil {
		return nil, microerror.Mask(err)
	}

	if appCfg.Catalog == "" {
		appCfg.Catalog = "giantswarm"
	}

	if appCfg.Namespace == "" {
		appCfg.Namespace = appCfg.Name
	}

	var userConfigName string
	var userConfigNamespace string
	if appCfg.ValuesYAML != "" {
		userConfigName = getCmName(appCfg)
		userConfigNamespace = clusterID
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
			UserConfig: appv1alpha1.AppSpecUserConfig{
				ConfigMap: appv1alpha1.AppSpecUserConfigConfigMap{
					Name:      userConfigName,
					Namespace: userConfigNamespace,
				},
			},
			Name:      appCfg.Name,
			Namespace: appCfg.Namespace,
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

func getCmName(appCfg AppConfig) string {
	return fmt.Sprintf("%s-user-values", appCfg.Name)
}
