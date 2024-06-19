package feature

import (
	"fmt"
	"path/filepath"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/kustomize/api/krusty"
	"sigs.k8s.io/kustomize/api/resmap"
	"sigs.k8s.io/kustomize/kyaml/filesys"
	"sigs.k8s.io/yaml"
)

func CreateKustomizeManifest(fsys filesys.FileSystem, path string, plugins ...resmap.Transformer) *KustomizeManifest {
	return &KustomizeManifest{
		name:    filepath.Base(path),
		path:    path,
		fsys:    fsys,
		plugins: plugins,
	}
}

// KustomizeManifest supports paths to kustomization files / directories containing a kustomization file
// note that it only supports to paths within the mounted files ie: /opt/manifests.
type KustomizeManifest struct {
	name,
	path string
	fsys    filesys.FileSystem
	plugins []resmap.Transformer
}

func (k *KustomizeManifest) Process() ([]*unstructured.Unstructured, error) {
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

	return ConvertToUnstructuredObjects(resMap)
}

func ConvertToUnstructuredObjects(resMap resmap.ResMap) ([]*unstructured.Unstructured, error) {
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
