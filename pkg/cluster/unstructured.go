package cluster

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	k8stypes "k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/yaml"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/opendatahub-io/opendatahub-operator/v2/pkg/metadata/annotations"
)

type Applier func(objects []*unstructured.Unstructured) error

func ConvertToUnstructured(resources string) ([]*unstructured.Unstructured, error) {
	splitter := regexp.MustCompile(resourceSeparator)
	objectStrings := splitter.Split(resources, -1)
	objs := make([]*unstructured.Unstructured, 0, len(objectStrings))
	for _, str := range objectStrings {
		if strings.TrimSpace(str) == "" {
			continue
		}
		u := &unstructured.Unstructured{}
		if err := yaml.Unmarshal([]byte(str), u); err != nil {
			return nil, err
		}

		objs = append(objs, u)
	}
	return objs, nil
}

const (
	resourceSeparator = "(?m)^---[ \t]*$"
)

func ApplyResources(ctx context.Context, cli client.Client, objects []*unstructured.Unstructured, metaOptions ...MetaOptions) error {
	for _, object := range objects {
		for _, opt := range metaOptions {
			if err := opt(object); err != nil {
				return err
			}
		}

		name := object.GetName()
		namespace := object.GetNamespace()

		err := cli.Get(ctx, k8stypes.NamespacedName{Name: name, Namespace: namespace}, object.DeepCopy())
		if client.IgnoreNotFound(err) != nil {
			return fmt.Errorf("failed to get object %s/%s: %w", namespace, name, err)
		}

		if err != nil {
			// object does not exist and should be created
			if createErr := cli.Create(ctx, object); client.IgnoreAlreadyExists(createErr) != nil {
				return fmt.Errorf("failed to create object %s/%s: %w", namespace, name, createErr)
			}
		}
		// object exists, check if it is managed
		isManaged, isAnnotated := object.GetAnnotations()[annotations.ManagedByODHOperator]
		if isAnnotated && isManaged == "true" {
			// update the object since we manage it
			if errUpdate := cli.Update(ctx, object); errUpdate != nil {
				return fmt.Errorf("failed to update object %s/%s: %w", namespace, name, errUpdate)
			}
		}
		// object exists and is not manged, skip reconcile allowing users to tweak it
	}
	return nil
}

func PatchResources(ctx context.Context, cli client.Client, patches []*unstructured.Unstructured) error {
	for _, patch := range patches {
		// Convert the individual resource patch to JSON
		patchAsJSON, errJSON := patch.MarshalJSON()
		if errJSON != nil {
			return fmt.Errorf("error converting yaml to json: %w", errJSON)
		}

		if errPatch := cli.Patch(ctx, patch, client.RawPatch(k8stypes.MergePatchType, patchAsJSON)); errPatch != nil {
			return fmt.Errorf("failed patching resource: %w", errPatch)
		}
	}

	return nil
}
