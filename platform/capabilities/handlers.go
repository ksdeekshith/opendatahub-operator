package capabilities

import (
	"context"
	"fmt"
	"os"
	"path"

	"sigs.k8s.io/controller-runtime/pkg/client"

	dsci "github.com/opendatahub-io/opendatahub-operator/v2/apis/dscinitialization/v1"
	"github.com/opendatahub-io/opendatahub-operator/v2/pkg/feature"
)

var _ Handler = (*AuthorizationHandler)(nil)

type AuthorizationHandler struct {
	hooksRegistry
}

// Configure enables the Authorization capability and component-specific configuration through registered hooks.
func (a *AuthorizationHandler) Configure(ctx context.Context, cli client.Client, dsciSpec *dsci.DSCInitializationSpec) error {
	for _, hook := range a.hooksRegistry.configHooks {
		if err := hook(ctx, cli); err != nil {
			return err
		}
	}

	// TODO https://issues.redhat.com/browse/RHOAIENG-6016
	// TODO installation path deploy.DeployManifestsFromPath()
	fmt.Println(">>>>> TODO: deploy authz controller")
	authInitializer := feature.ComponentFeaturesHandler("Platform", dsciSpec, defineAuthFeatures())
	return authInitializer.Apply()
}

func (a *AuthorizationHandler) Remove(ctx context.Context, cli client.Client, dsciSpec *dsci.DSCInitializationSpec) error {
	for _, hook := range a.hooksRegistry.configHooks {
		if err := hook(ctx, cli); err != nil {
			return err
		}
	}
	// todo: remove clusterrole + clusterrolebinding - they are not part of the feature currently so not deleted.
	authInitializer := feature.ComponentFeaturesHandler("Platform", dsciSpec, defineAuthFeatures())
	return authInitializer.Delete()
}

type hooksRegistry struct {
	configHooks []HookFunc
	removeHooks []HookFunc
}

func (c *hooksRegistry) AddConfigureHook(hook HookFunc) {
	c.configHooks = append(c.configHooks, hook)
}

func (c *hooksRegistry) AddRemoveHook(hook HookFunc) {
	c.removeHooks = append(c.removeHooks, hook)
}

func defineAuthFeatures() feature.FeaturesProvider {
	return func(handler *feature.FeaturesHandler) error {
		createControllerErr := feature.CreateFeature("deploy-odh-platform").
			For(handler).
			ManifestSource(os.DirFS(".")). // tmp, unused
			Manifests(
				path.Join("/opt/manifests/platform/default"),
			).
			Load()

		if createControllerErr != nil {
			return createControllerErr
		}

		return nil
	}
}
