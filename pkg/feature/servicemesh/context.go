package servicemesh

import (
	"context"
	"strings"

	"sigs.k8s.io/controller-runtime/pkg/client"

	dsci "github.com/opendatahub-io/opendatahub-operator/v2/apis/dscinitialization/v1"
	v1 "github.com/opendatahub-io/opendatahub-operator/v2/apis/infrastructure/v1"
	"github.com/opendatahub-io/opendatahub-operator/v2/pkg/feature"
)

// These keys are used in FeatureData struct, as fields of a struct are not accessible in closures which we define for
// creating and fetching the data.
const (
	controlPlaneKey      string = "ControlPlane"
	authKey              string = "Auth"
	authProviderNsKey    string = "AuthNamespace"
	authProviderNameKey  string = "AuthProviderName"
	authExtensionNameKey string = "AuthExtensionName"
)

// FeatureData is a convention to simplify how the data for the Service Mesh features is created and accessed.
// Being a "singleton" it is based on anonymous struct concept.
var FeatureData = struct {
	ControlPlane  feature.ContextDefinition[dsci.DSCInitializationSpec, v1.ControlPlaneSpec]
	Authorization AuthorizationContext
}{
	ControlPlane: feature.ContextDefinition[dsci.DSCInitializationSpec, v1.ControlPlaneSpec]{
		Create: func(source *dsci.DSCInitializationSpec) feature.ContextEntry[v1.ControlPlaneSpec] {
			return feature.ContextEntry[v1.ControlPlaneSpec]{
				Key: controlPlaneKey,
				Value: func(_ context.Context, _ client.Client) (v1.ControlPlaneSpec, error) {
					return source.ServiceMesh.ControlPlane, nil
				},
			}
		},
		From: feature.ExtractEntry[v1.ControlPlaneSpec](controlPlaneKey),
	},
	Authorization: AuthorizationContext{
		Spec:                  authSpec,
		Namespace:             authNs,
		Provider:              authProvider,
		ExtensionProviderName: authExtensionName,
		All: func(source *dsci.DSCInitializationSpec) []feature.Action {
			return []feature.Action{
				authSpec.Create(source).AsAction(),
				authNs.Create(source).AsAction(),
				authProvider.Create(source).AsAction(),
				authExtensionName.Create(source).AsAction(),
			}
		},
	},
}

type AuthorizationContext struct {
	Spec                  feature.ContextDefinition[dsci.DSCInitializationSpec, v1.AuthSpec]
	Namespace             feature.ContextDefinition[dsci.DSCInitializationSpec, string]
	Provider              feature.ContextDefinition[dsci.DSCInitializationSpec, string]
	ExtensionProviderName feature.ContextDefinition[dsci.DSCInitializationSpec, string]
	All                   func(source *dsci.DSCInitializationSpec) []feature.Action
}

var authSpec = feature.ContextDefinition[dsci.DSCInitializationSpec, v1.AuthSpec]{
	Create: func(source *dsci.DSCInitializationSpec) feature.ContextEntry[v1.AuthSpec] {
		return feature.ContextEntry[v1.AuthSpec]{
			Key: authKey,
			Value: func(_ context.Context, _ client.Client) (v1.AuthSpec, error) {
				return source.ServiceMesh.Auth, nil
			},
		}
	},
	From: feature.ExtractEntry[v1.AuthSpec](authKey),
}

var authNs = feature.ContextDefinition[dsci.DSCInitializationSpec, string]{
	Create: func(source *dsci.DSCInitializationSpec) feature.ContextEntry[string] {
		return feature.ContextEntry[string]{
			Key: authProviderNsKey,
			Value: func(_ context.Context, _ client.Client) (string, error) {
				ns := strings.TrimSpace(source.ServiceMesh.Auth.Namespace)
				if len(ns) == 0 {
					ns = source.ApplicationsNamespace + "-auth-provider"
				}

				return ns, nil
			},
		}
	},
	From: feature.ExtractEntry[string](authProviderNsKey),
}

var authProvider = feature.ContextDefinition[dsci.DSCInitializationSpec, string]{
	Create: func(source *dsci.DSCInitializationSpec) feature.ContextEntry[string] {
		return feature.ContextEntry[string]{
			Key: authProviderNameKey,
			Value: func(_ context.Context, _ client.Client) (string, error) {
				return "authorino", nil
			},
		}
	},
	From: feature.ExtractEntry[string](authProviderNameKey),
}

var authExtensionName = feature.ContextDefinition[dsci.DSCInitializationSpec, string]{
	Create: func(source *dsci.DSCInitializationSpec) feature.ContextEntry[string] {
		return feature.ContextEntry[string]{
			Key: authExtensionNameKey,
			Value: func(_ context.Context, _ client.Client) (string, error) {
				return source.ApplicationsNamespace + "-auth-provider", nil
			},
		}
	},
	From: feature.ExtractEntry[string](authExtensionNameKey),
}
