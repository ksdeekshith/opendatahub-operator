package modelregistry

import (
	"github.com/opendatahub-io/opendatahub-operator/v2/platform/capabilities"
	"k8s.io/apimachinery/pkg/runtime/schema"
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
