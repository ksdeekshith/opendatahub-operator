package modelregistry

import (
	dsciv1 "github.com/opendatahub-io/opendatahub-operator/v2/apis/dscinitialization/v1"
	"github.com/opendatahub-io/opendatahub-operator/v2/pkg/feature"
	operatorv1 "github.com/openshift/api/operator/v1"
	"os"
	"path"
)

func (m *ModelRegistry) configureAuth(dscispec *dsciv1.DSCInitializationSpec) error {
	// TODO: replace logic to not check SM management state, but if component requires auth.
	if dscispec.ServiceMesh.ManagementState == operatorv1.Managed && m.GetManagementState() == operatorv1.Managed {
		authInitializer := feature.ComponentFeaturesHandler(m.GetComponentName(), dscispec, m.defineAuthFeatures())
		return authInitializer.Apply()
	}

	return m.removeAuth(dscispec)
}

func (m *ModelRegistry) removeAuth(dscispec *dsciv1.DSCInitializationSpec) error {
	authInitializer := feature.ComponentFeaturesHandler(m.GetComponentName(), dscispec, m.defineAuthFeatures())
	return authInitializer.Delete()
}

func (m *ModelRegistry) defineAuthFeatures() feature.FeaturesProvider {
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
