package capabilities

import (
	"context"
	"encoding/json"

	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/opendatahub-io/opendatahub-operator/v2/pkg/cluster"
)

// Consumer

// ProtectedResource defines a custom resource type that the component requires capability for.
type ProtectedResource struct {
	// Schema is the schema of the resource.
	Schema ResourceSchema `json:"schema,omitempty"`
	// WorkloadSelector is a map of labels used to select the workload.
	WorkloadSelector map[string]string `json:"workloadSelector,omitempty"`
	// HostPaths is a list of host paths associated with the resource.
	HostPaths []string `json:"hostPaths,omitempty"`
	// Ports is a list of ports associated with the resource.
	Ports []string `json:"ports,omitempty"`
}

type ResourceSchema struct {
	// GroupVersionKind specifies the group, version, and kind of the resource.
	schema.GroupVersionKind `json:"gvk,omitempty"`
	// Resources is the type of resource being protected, e.g., "pods", "services".
	Resources string `json:"resources,omitempty"`
}

func NewAuthorization(available bool) AuthorizationCapability {
	return AuthorizationCapability{available: available}
}

type Authorization interface {
	Availability
	ProtectedResources(protectedResource ...ProtectedResource)
}

// Producer

var _ Authorization = (*AuthorizationCapability)(nil)

type AuthorizationCapability struct {
	available          bool
	protectedResources []ProtectedResource
}

func (a *AuthorizationCapability) IsAvailable() bool {
	return a.available
}

func (a *AuthorizationCapability) ProtectedResources(protectedResource ...ProtectedResource) {
	a.protectedResources = protectedResource
}

func (a *AuthorizationCapability) AsJSON() ([]byte, error) {
	return json.Marshal(a.protectedResources)
}

var _ Handler = (*AuthorizationCapability)(nil)

func (a *AuthorizationCapability) IsRequired() bool {
	return len(a.protectedResources) > 0
}

// Reconcile ensures Authorization capability and component-specific configuration is wired when needed.
func (a *AuthorizationCapability) Reconcile(ctx context.Context, cli client.Client, metaOptions ...cluster.MetaOptions) error {
	if a.IsRequired() {
		return CreateOrUpdateAuthzBindings(ctx, cli, a.protectedResources, metaOptions...)
	}

	return DeleteAuthzBindings(ctx, cli)
}
