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
	// GroupVersionKind specifies the group, version, and kind of the resource.
	schema.GroupVersionKind `json:"gvk,omitempty"`
	// WorkloadSelector is a map of labels used to select the workload.
	WorkloadSelector map[string]string `json:"workloadSelector,omitempty"`
	// Resources is the type of resource being protected, e.g., "pods", "services".
	Resources string `json:"resources,omitempty"`
	// HostPaths is a list of host paths associated with the resource.
	HostPaths []string `json:"hostPaths,omitempty"`
	// Ports is a list of ports associated with the resource.
	Ports []string `json:"ports,omitempty"`
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

func (a *AuthorizationCapability) asJSON() ([]byte, error) {
	return json.Marshal(a.protectedResources)
}

var _ Handler = (*AuthorizationCapability)(nil)

func (a *AuthorizationCapability) IsRequired() bool {
	return len(a.protectedResources) > 0
}

// Configure enables the Authorization capability and component-specific configuration through registered hooks.
func (a *AuthorizationCapability) Configure(ctx context.Context, cli client.Client, metaOptions ...cluster.MetaOptions) error {
	if a.IsRequired() {
		return CreateOrUpdateAuthzBindings(ctx, cli, a.protectedResources, metaOptions...)
	}

	return DeleteAuthzBindings(ctx, cli)
}

func (a *AuthorizationCapability) Remove(ctx context.Context, cli client.Client) error {
	return DeleteAuthzBindings(ctx, cli)
}
