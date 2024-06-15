package capabilities

import (
	"context"
	"fmt"

	rbacv1 "k8s.io/api/rbac/v1"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/opendatahub-io/opendatahub-operator/v2/pkg/cluster"
)

const roleName = "platform-protected-resources-watcher"

// CreateOrUpdateAuthzRoleBinding defines roles which allow platform authorization component to handle protected resources.
// TODO: ? Since we have all in one registry, component split does not make sense anymore nor it's actually needed
// TODO(mvp) tests.
func CreateOrUpdateAuthzRoleBinding(ctx context.Context, cli client.Client, protectedResources []ProtectedResource, metaOptions ...cluster.MetaOptions) error {
	clusterRoleBinding, err := createAuthzRoleBinding(ctx, cli, protectedResources, roleName, metaOptions...)
	if err != nil {
		return err
	}

	if err := cli.Get(ctx, client.ObjectKey{Name: clusterRoleBinding.Name, Namespace: clusterRoleBinding.Namespace}, clusterRoleBinding); err != nil {
		if apierrs.IsNotFound(err) {
			if err := cli.Create(ctx, clusterRoleBinding); err != nil {
				return fmt.Errorf("failed creating cluster role binding: %w", err)
			}
		} else {
			return err
		}
	} else {
		if err := cli.Update(ctx, clusterRoleBinding); err != nil {
			return fmt.Errorf("failed updating cluster role binding: %w", err)
		}
	}

	return nil
}

// TryDeleteAuthzRoleBinding attempts to remove created authz role/bindings but does not fail if these are not existing in the cluster.
// TODO: Don't create the RoleBinding first then delete it...
func TryDeleteAuthzRoleBinding(ctx context.Context, cli client.Client, protectedResources []ProtectedResource) error {
	clusterRoleBinding, err := createAuthzRoleBinding(ctx, cli, protectedResources, roleName)
	if err != nil {
		return err
	}

	if err := cli.Get(ctx, client.ObjectKey{Name: clusterRoleBinding.Name, Namespace: clusterRoleBinding.Namespace}, clusterRoleBinding); client.IgnoreNotFound(err) != nil {
		return err
	}

	return client.IgnoreNotFound(cli.Delete(ctx, clusterRoleBinding))
}

func createAuthzRoleBinding(ctx context.Context, cli client.Client, protectedResources []ProtectedResource, roleName string, metaOptions ...cluster.MetaOptions) (*rbacv1.ClusterRoleBinding, error) {
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
			Verbs:     []string{"get", "list", "watch"},
		},
	}

	if _, roleErr := cluster.CreateClusterRole(ctx, cli, roleName, rules, metaOptions...); roleErr != nil {
		return nil, fmt.Errorf("failed creating cluster roles: %w", roleErr)
	}

	// TODO(mvp) should not be hardcoding to "opendatahub", find clean way to pass namespace
	clusterRoleBinding := &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "opendatahub-" + roleName,
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
			Name:     roleName,
		},
	}
	return clusterRoleBinding, nil
}
