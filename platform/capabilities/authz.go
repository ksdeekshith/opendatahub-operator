package capabilities

import (
	"context"
	"fmt"

	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
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
	schema.GroupVersionKind `json:"gvk,omitempty"`
	WorkloadSelector        map[string]string `json:"workloadSelector,omitempty"`
	HostPaths               []string          `json:"hostPaths,omitempty"`
	Ports                   []string          `json:"ports,omitempty"`
}

// CreateAuthzRoleBinding defines roles which allow platform authorization component to handle protected resources.
// TODO Remove counterpart.
func CreateAuthzRoleBinding(ctx context.Context, cli client.Client, componentName string, protectedResources []ProtectedResource, verbs ...string) error {
	apiGroups := make([]string, 0)
	resources := make([]string, 0)

	for _, resource := range protectedResources {
		apiGroups = append(apiGroups, resource.GroupVersionKind.Group)
		resources = append(resources, resource.GroupVersionKind.Kind)
	}

	clusterRole := &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name: componentName + "-role",
		},
		Rules: []rbacv1.PolicyRule{
			{
				APIGroups: apiGroups,
				Resources: resources,
				Verbs:     verbs,
			},
		},
	}

	if err := cli.Get(ctx, client.ObjectKey{Name: clusterRole.Name, Namespace: clusterRole.Namespace}, clusterRole); client.IgnoreNotFound(err) != nil {
		return err
	}

	if err := cli.Create(ctx, clusterRole); err != nil {
		return fmt.Errorf("failed creating cluster role for %s: %w", componentName, err)
	}

	clusterRoleBinding := &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      componentName + "-authz-capability",
			Namespace: "opendatahub",
		},
		Subjects: []rbacv1.Subject{
			{
				Kind: rbacv1.ServiceAccountKind,
				Name: "odh-platform-ctrl",
			},
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "ClusterRole",
			Name:     componentName + "-watchers",
		},
	}

	if err := cli.Get(ctx, client.ObjectKey{Name: clusterRoleBinding.Name, Namespace: clusterRoleBinding.Namespace}, clusterRoleBinding); client.IgnoreNotFound(err) != nil {
		return err
	}

	if err := cli.Create(ctx, clusterRoleBinding); err != nil {
		return fmt.Errorf("failed creating cluster role binding for %s: %w", componentName, err)
	}

	return nil
}
