package kserve

import (
	"path"

	dsciv1 "github.com/opendatahub-io/opendatahub-operator/v2/apis/dscinitialization/v1"
	"github.com/opendatahub-io/opendatahub-operator/v2/pkg/feature"
	"github.com/opendatahub-io/opendatahub-operator/v2/pkg/feature/serverless"
	"github.com/opendatahub-io/opendatahub-operator/v2/pkg/feature/servicemesh"
)

func (k *Kserve) configureServerlessFeatures(dsciSpec *dsciv1.DSCInitializationSpec) feature.FeaturesProvider {
	return func(registry feature.FeaturesRegistry) error {
		servingDeployment := feature.Define("serverless-serving-deployment").
			Manifests().
			Location(Resources.Location).
			Paths(
				path.Join(Resources.InstallDir),
			).
			Done().
			WithData(
				serverless.FeatureData.IngressDomain.Create(&k.Serving).AsAction(),
				serverless.FeatureData.Serving.Create(&k.Serving).AsAction(),
				servicemesh.FeatureData.ControlPlane.Create(dsciSpec).AsAction(),
			).
			PreConditions(
				serverless.EnsureServerlessOperatorInstalled,
				serverless.EnsureServerlessAbsent,
				servicemesh.EnsureServiceMeshInstalled,
				feature.CreateNamespaceIfNotExists(serverless.KnativeServingNamespace),
			).
			PostConditions(
				feature.WaitForPodsToBeReady(serverless.KnativeServingNamespace),
			)

		istioSecretFiltering := feature.Define("serverless-net-istio-secret-filtering").
			Manifests().
			Location(Resources.Location).
			Paths(
				path.Join(Resources.BaseDir, "serving-net-istio-secret-filtering.patch.tmpl.yaml"),
			).
			Done().
			WithData(serverless.FeatureData.Serving.Create(&k.Serving).AsAction()).
			PreConditions(serverless.EnsureServerlessServingDeployed).
			PostConditions(
				feature.WaitForPodsToBeReady(serverless.KnativeServingNamespace),
			)

		servingGateway := feature.Define("serverless-serving-gateways").
			Manifests().
			Location(Resources.Location).
			Paths(
				path.Join(Resources.GatewaysDir),
			).
			Done().
			WithData(
				serverless.FeatureData.IngressDomain.Create(&k.Serving).AsAction(),
				serverless.FeatureData.Certificate.Create(&k.Serving).AsAction(),
				serverless.FeatureData.Serving.Create(&k.Serving).AsAction(),
				servicemesh.FeatureData.ControlPlane.Create(dsciSpec).AsAction(),
			).
			WithResources(serverless.ServingCertificateResource).
			PreConditions(serverless.EnsureServerlessServingDeployed)

		return registry.Add(
			servingDeployment,
			istioSecretFiltering,
			servingGateway,
		)
	}
}
