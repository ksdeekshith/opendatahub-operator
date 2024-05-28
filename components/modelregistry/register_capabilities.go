package modelregistry

import (
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/opendatahub-io/opendatahub-operator/v2/platform/capabilities"
)

var _ capabilities.Authorization = (*ModelRegistry)(nil)

func (m *ModelRegistry) ProtectedResources() []capabilities.ProtectedResource {
	return []capabilities.ProtectedResource{
		{
			GroupVersionKind: schema.GroupVersionKind{
				Group:   "modelregistry.opendatahub.io",
				Version: "v1alpha1",
				Kind:    "ModelRegistry",
			},
			Resources: "modelregistries",
			WorkloadSelector: map[string]string{
				"app.kubernetes.io/component": "model-registry",
			},
			HostPaths: []string{"status.URL"},
			Ports:     []string{"8080"},
		},
	}
}

func (m *ModelRegistry) AuthorizationConfigurationHook() capabilities.HookFunc {
	return nil
}
