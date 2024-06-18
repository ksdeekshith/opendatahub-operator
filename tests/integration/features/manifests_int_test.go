package features_test

import (
	"context"
	"os"
	"path"

	corev1 "k8s.io/api/core/v1"

	dsciv1 "github.com/opendatahub-io/opendatahub-operator/v2/apis/dscinitialization/v1"
	"github.com/opendatahub-io/opendatahub-operator/v2/pkg/cluster"
	"github.com/opendatahub-io/opendatahub-operator/v2/pkg/feature"
	"github.com/opendatahub-io/opendatahub-operator/v2/pkg/feature/provider"
	"github.com/opendatahub-io/opendatahub-operator/v2/tests/envtestutil"
	"github.com/opendatahub-io/opendatahub-operator/v2/tests/integration/features/fixtures"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Manifest sources", func() {

	var (
		objectCleaner *envtestutil.Cleaner
		dsci          *dsciv1.DSCInitialization
		namespace     *corev1.Namespace
	)

	BeforeEach(func() {
		objectCleaner = envtestutil.CreateCleaner(envTestClient, envTest.Config, fixtures.Timeout, fixtures.Interval)
		nsName := envtestutil.AppendRandomNameTo("smcp-ns")

		var err error
		namespace, err = cluster.CreateNamespace(context.Background(), envTestClient, nsName)
		Expect(err).ToNot(HaveOccurred())

		dsci = fixtures.NewDSCInitialization(nsName)
		dsci.Spec.ServiceMesh.ControlPlane.Namespace = namespace.Name
	})

	AfterEach(func() {
		objectCleaner.DeleteAll(namespace)
	})

	It("should be able to process an embedded YAML file", func() {
		// given
		featuresHandler := feature.ClusterFeaturesHandler(dsci, func(registry feature.FeaturesRegistry) error {
			createNamespaceErr := registry.Add(feature.Define("create-namespace").
				UsingConfig(envTest.Config).
				Manifests().
				Location(fixtures.TestEmbeddedFiles).
				Paths(path.Join(fixtures.BaseDir, "namespace.yaml")).
				Done(),
			)

			Expect(createNamespaceErr).ToNot(HaveOccurred())

			return nil
		})

		// when
		Expect(featuresHandler.Apply()).To(Succeed())

		// then
		embeddedNs, err := fixtures.GetNamespace(envTestClient, "embedded-test-ns")
		defer objectCleaner.DeleteAll(embeddedNs)
		Expect(err).ToNot(HaveOccurred())
		Expect(embeddedNs.Name).To(Equal("embedded-test-ns"))
	})

	It("should be able to process an embedded template file", func() {
		// given
		featuresHandler := feature.ClusterFeaturesHandler(dsci, func(registry feature.FeaturesRegistry) error {
			createServiceErr := registry.Add(feature.Define("create-local-gw-svc").
				UsingConfig(envTest.Config).
				Manifests().
				Location(fixtures.TestEmbeddedFiles).
				Paths(path.Join(fixtures.BaseDir, "local-gateway-svc.tmpl.yaml")).
				Done().
				WithData(feature.Entry("ControlPlane", provider.ValueOf(dsci.Spec.ServiceMesh.ControlPlane).Get)),
			)

			Expect(createServiceErr).ToNot(HaveOccurred())

			return nil
		})

		// when
		Expect(featuresHandler.Apply()).To(Succeed())

		// then
		service, err := fixtures.GetService(envTestClient, namespace.Name, "knative-local-gateway")
		Expect(err).ToNot(HaveOccurred())
		Expect(service.Name).To(Equal("knative-local-gateway"))
	})

	const nsYAML = `apiVersion: v1
kind: Namespace
metadata:
  name: real-file-test-ns`

	It("should source manifests from a specified temporary directory within the file system", func() {
		// given
		tempDir := GinkgoT().TempDir()

		Expect(fixtures.CreateFile(tempDir, "namespace.yaml", nsYAML)).To(Succeed())

		featuresHandler := feature.ClusterFeaturesHandler(dsci, func(registry feature.FeaturesRegistry) error {
			createServiceErr := registry.Add(feature.Define("create-namespace").
				UsingConfig(envTest.Config).
				Manifests().
				Location(os.DirFS(tempDir)).
				Paths(path.Join("namespace.yaml")). // must be relative to root DirFS defined above
				Done(),
			)

			Expect(createServiceErr).ToNot(HaveOccurred())

			return nil
		})

		// when
		Expect(featuresHandler.Apply()).To(Succeed())

		// then
		realNs, err := fixtures.GetNamespace(envTestClient, "real-file-test-ns")
		defer objectCleaner.DeleteAll(realNs)
		Expect(err).ToNot(HaveOccurred())
		Expect(realNs.Name).To(Equal("real-file-test-ns"))
	})

	// TODO(mvp): kustomize manifests need to be reworked and have target namespace/plugin passed instead of assuming it is
	// passed as part of Process(data any)
	PIt("should process kustomization manifests directly from the file system", func() {
		// TODO: we create dummy tempdir just to pass in for ManifestSource - messy but temporary?
		tempDir := GinkgoT().TempDir()

		// given
		featuresHandler := feature.ClusterFeaturesHandler(dsci, func(registry feature.FeaturesRegistry) error {
			return registry.Add(feature.Define("create-cfg-map").
				UsingConfig(envTest.Config).
				Manifests().
				Location(os.DirFS(tempDir)).
				Paths(path.Join("fixtures", fixtures.BaseDir, "fake-kust-dir")).
				Done(),
			)
		})

		// when
		Expect(featuresHandler.Apply()).To(Succeed())

		// then
		cfgMap, err := fixtures.GetConfigMap(envTestClient, dsci.Spec.ApplicationsNamespace, "my-configmap")
		Expect(err).ToNot(HaveOccurred())
		Expect(cfgMap.Name).To(Equal("my-configmap"))
		Expect(cfgMap.Data["key"]).To(Equal("value"))
	})
})
