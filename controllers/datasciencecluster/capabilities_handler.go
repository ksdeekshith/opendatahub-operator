package datasciencecluster

import (
	"context"
	"encoding/json"
	"github.com/hashicorp/go-multierror"
	operatorv1 "github.com/openshift/api/operator/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	dsc "github.com/opendatahub-io/opendatahub-operator/v2/apis/datasciencecluster/v1"
	dsci "github.com/opendatahub-io/opendatahub-operator/v2/apis/dscinitialization/v1"
	"github.com/opendatahub-io/opendatahub-operator/v2/pkg/cluster"
	"github.com/opendatahub-io/opendatahub-operator/v2/pkg/metadata/labels"
	"github.com/opendatahub-io/opendatahub-operator/v2/platform/capabilities"
)

func (r *DataScienceClusterReconciler) configurePlatformCapabilities(ctx context.Context, instance *dsc.DataScienceCluster, dsciSpec *dsci.DSCInitializationSpec) error {
	allComponents, err := instance.GetComponents()
	if err != nil {
		return err
	}
	componentCapabilities := make(map[string][]any)
	capabilitiesHandler := make(map[string]capabilities.Handler)

	// for now only these capabilities are supported
	authName := "authorization"
	capabilitiesHandler[authName] = &capabilities.AuthorizationHandler{}

	for _, component := range allComponents {
		component := component
		if authComponent, ok := component.(capabilities.Authorization); ok && component.GetManagementState() == operatorv1.Managed {
			capabilitiesHandler[authName].AddConfigureHook(func(ctx context.Context, cli client.Client) error {
				return capabilities.CreateAuthzRoleBinding(ctx, cli, component.GetComponentName(), authComponent.ProtectedResources(), "get", "list", "watch")
			})
			componentCapabilities[authName] = append(componentCapabilities[authName], authComponent.ProtectedResources())
		}
	}

	var multiErr *multierror.Error
	multiErr = multierror.Append(multiErr, r.saveCapabilitiesSettings(instance, componentCapabilities))

	for i := range capabilitiesHandler {
		defs, capabilityRequested := componentCapabilities[i]
		if capabilityRequested && len(defs) != 0 { // there is at least one component requesting given capability
			multiErr = multierror.Append(multiErr, capabilitiesHandler[i].Configure(ctx, r.Client, dsciSpec))
		}
	}

	return multiErr.ErrorOrNil()
}

func (r *DataScienceClusterReconciler) saveCapabilitiesSettings(instance *dsc.DataScienceCluster, componentCapabilities map[string][]any) error {
	platformSettings := make(map[string]string)
	capabilitiesAsJSON, err := json.Marshal(componentCapabilities)
	if err != nil {
		return err
	}
	platformSettings["capabilities"] = string(capabilitiesAsJSON)

	return cluster.CreateOrUpdateConfigMap(
		r.Client,
		&corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "platform-capabilities",
				Namespace: r.DataScienceCluster.DSCISpec.ApplicationsNamespace,
			},
			Data: platformSettings,
		},
		cluster.OwnedBy(instance, r.Scheme),
		cluster.WithLabels(
			labels.K8SCommon.PartOf, "opendatahub", // TODO revise
			labels.K8SCommon.ManagedBy, "opendatahub-operator", // TODO change to platform-type aware
		),
	)
}
