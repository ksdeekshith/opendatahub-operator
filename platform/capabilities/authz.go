package capabilities

import (
	"context"
	"fmt"

	rbacv1 "k8s.io/api/rbac/v1"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/opendatahub-io/opendatahub-operator/v2/pkg/cluster"
)

// Authorization is a contract for components that require authorization capability.
type Authorization interface {
	// ProtectedResources defines custom resource type that the component requires authorization for.
	ProtectedResources() []ProtectedResource
	// AuthorizationConfigurationHook defines a function that can be used to configure the authorization capability per component.
	AuthorizationConfigurationHook() HookFunc
}

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

// CreateAuthzRoleBinding defines roles which allow platform authorization component to handle protected resources.
// TODO Remove counterpart.
func CreateAuthzRoleBinding(ctx context.Context, cli client.Client, componentName string, protectedResources []ProtectedResource, verbs ...string) error {
	name := componentName + "-watchers"

	apiGroups := make([]string, 0)
	resources := make([]string, 0)
	for _, resource := range protectedResources {
		apiGroups = append(apiGroups, resource.GroupVersionKind.Group)
		resources = append(resources, resource.Resources)
	}

	rules := []rbacv1.PolicyRule{
		{
			APIGroups: apiGroups,
			Resources: resources,
			Verbs:     verbs,
		},
	}

	if _, roleErr := cluster.CreateClusterRole(ctx, cli, name, rules); roleErr != nil {
		return fmt.Errorf("failed creating cluster role for %s: %w", componentName, roleErr)
	}

	// todo: should not be hardcoding to "opendatahub", find clean way to pass namespace
	clusterRoleBinding := &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      componentName + "-authz-capability",
			Namespace: "opendatahub",
		},
		Subjects: []rbacv1.Subject{
			{
				Kind:      rbacv1.ServiceAccountKind,
				Name:      "odh-platform-manager",
				Namespace: "opendatahub",
			},
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "ClusterRole",
			Name:     componentName + "-watchers",
		},
	}

	if err := cli.Get(ctx, client.ObjectKey{Name: clusterRoleBinding.Name, Namespace: clusterRoleBinding.Namespace}, clusterRoleBinding); err != nil {
		if apierrs.IsNotFound(err) {
			if err := cli.Create(ctx, clusterRoleBinding); err != nil {
				return fmt.Errorf("failed creating cluster role binding for %s: %w", componentName, err)
			}
		} else {
			return err
		}
	} else {
		if err := cli.Update(ctx, clusterRoleBinding); err != nil {
			return fmt.Errorf("failed updating cluster role binding for %s: %w", componentName, err)
		}
	}

	return nil
}
