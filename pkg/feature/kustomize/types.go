package kustomize

import (
	"context"
	"fmt"
	"path/filepath"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/kustomize/api/krusty"
	"sigs.k8s.io/kustomize/api/resmap"
	"sigs.k8s.io/kustomize/kyaml/filesys"
	"sigs.k8s.io/yaml"

	"github.com/opendatahub-io/opendatahub-operator/v2/pkg/cluster"
)

func Create(fsys filesys.FileSystem, path string, plugins ...resmap.Transformer) *Kustomization {
	return &Kustomization{
		name:    filepath.Base(path),
		path:    path,
		fsys:    fsys,
		plugins: plugins,
	}
}

// Kustomization supports paths to kustomization files / directories containing a kustomization file
// note that it only supports to paths within the mounted files ie: /opt/manifests.
type Kustomization struct {
	name,
	path string
	fsys    filesys.FileSystem
	plugins []resmap.Transformer
}

func (k *Kustomization) Process() ([]*unstructured.Unstructured, error) {
	kustomizer := krusty.MakeKustomizer(krusty.MakeDefaultOptions())

	resMap, errRes := kustomizer.Run(k.fsys, k.path)
	if errRes != nil {
		return nil, fmt.Errorf("error during resmap resources: %w", errRes)
	}

	for _, plugin := range k.plugins {
		if err := plugin.Transform(resMap); err != nil {
			return nil, err
		}
	}

	return convertToUnstructuredObjects(resMap)
}

// Applier wraps an instance of Manifest and provides a way to apply it to the cluster.
type Applier struct {
	kustomization *Kustomization
}

func CreateApplier(manifest *Kustomization) *Applier {
	return &Applier{
		kustomization: manifest,
	}
}

// Apply processes owned manifest and apply it to a cluster.
func (a Applier) Apply(ctx context.Context, cli client.Client, _ map[string]any, options ...cluster.MetaOptions) error {
	objects, errProcess := a.kustomization.Process()
	if errProcess != nil {
		return errProcess
	}

	return cluster.ApplyResources(ctx, cli, objects, options...)
}

func convertToUnstructuredObjects(resMap resmap.ResMap) ([]*unstructured.Unstructured, error) {
	resources := make([]*unstructured.Unstructured, 0, resMap.Size())
	for _, res := range resMap.Resources() {
		u := &unstructured.Unstructured{}
		asYAML, errToYAML := res.AsYAML()
		if errToYAML != nil {
			return nil, errToYAML
		}
		if errUnmarshal := yaml.Unmarshal(asYAML, u); errUnmarshal != nil {
			return nil, errUnmarshal
		}
		resources = append(resources, u)
	}

	return resources, nil
}
