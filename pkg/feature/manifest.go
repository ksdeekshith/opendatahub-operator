package feature

import (
	"bytes"
	"fmt"
	"html/template"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/ghodss/yaml"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/kustomize/api/krusty"
	"sigs.k8s.io/kustomize/api/resmap"
	"sigs.k8s.io/kustomize/kyaml/filesys"

	"github.com/opendatahub-io/opendatahub-operator/v2/pkg/metadata/annotations"
	"github.com/opendatahub-io/opendatahub-operator/v2/pkg/plugins"
)

type Manifest struct {
	name,
	path string
	patch bool
	fsys  fs.FS
}

// Process allows any arbitrary struct to be passed and used while processing the content of the manifest.
func (m *Manifest) Process(data any) ([]*unstructured.Unstructured, error) {
	manifestFile, err := m.fsys.Open(m.path)
	if err != nil {
		return nil, err
	}

	defer manifestFile.Close()

	content, err := io.ReadAll(manifestFile)
	if err != nil {
		return nil, fmt.Errorf("failed to create file: %w", err)
	}

	resources := string(content)

	if isTemplate(m.path) {
		tmpl, err := template.New(m.name).
			Option("missingkey=error").
			Parse(string(content))
		if err != nil {
			return nil, fmt.Errorf("failed to parse template: %w", err)
		}

		var buffer bytes.Buffer
		if err := tmpl.Execute(&buffer, data); err != nil {
			return nil, fmt.Errorf("failed to execute template: %w", err)
		}

		resources = buffer.String()
	}

	return convertToUnstructuredSlice(resources)
}

func isTemplate(path string) bool {
	return strings.Contains(filepath.Base(path), ".tmpl.")
}

// MarkAsManaged sets all non-patch objects to be managed/reconciled by setting the annotation.
func (m *Manifest) MarkAsManaged(objects []*unstructured.Unstructured) {
	if !m.patch {
		markAsManaged(objects)
	}
}

func markAsManaged(objs []*unstructured.Unstructured) {
	for _, obj := range objs {
		objAnnotations := obj.GetAnnotations()
		if objAnnotations == nil {
			objAnnotations = make(map[string]string)
		}

		objAnnotations[annotations.ManagedByODHOperator] = "true"
		obj.SetAnnotations(objAnnotations)
	}
}

func loadManifestsFrom(fsys fs.FS, path string) ([]*Manifest, error) {
	var manifests []*Manifest
	// check local filesystem for kustomization manifest
	// TODO rework
	// if isKustomizeManifest(path) {
	//	m := CreateKustomizeManifestFrom(path, filesys.MakeFsOnDisk())
	//	manifests = append(manifests, m)
	//	return manifests, nil
	//}

	err := fs.WalkDir(fsys, path, func(path string, dirEntry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}

		_, err := dirEntry.Info()
		if err != nil {
			return err
		}

		if dirEntry.IsDir() {
			return nil
		}

		manifests = append(manifests, CreateManifestFrom(fsys, path))

		return nil
	})

	if err != nil {
		return nil, err
	}

	return manifests, nil
}

func CreateManifestFrom(fsys fs.FS, path string) *Manifest {
	basePath := filepath.Base(path)
	return &Manifest{
		name:  basePath,
		path:  path,
		patch: strings.Contains(basePath, ".patch"),
		fsys:  fsys,
	}
}

const kustomizationFile = "kustomization.yaml"

// kustomizeManifest supports paths to kustomization files / directories containing a kustomization file
// note that it only supports to paths within the mounted files ie: /opt/manifests.
type kustomizeManifest struct {
	name,
	path string
	fsys filesys.FileSystem
}

func (k *kustomizeManifest) Process(data any) ([]*unstructured.Unstructured, error) {
	kustomizer := krusty.MakeKustomizer(krusty.MakeDefaultOptions())
	var resMap resmap.ResMap
	resMap, resErr := kustomizer.Run(k.fsys, k.path)

	if resErr != nil {
		return nil, fmt.Errorf("error during resmap resources: %w", resErr)
	}

	targetNs := getTargetNs(data)
	if targetNs == "" {
		return nil, fmt.Errorf("targetNamespaces not defined")
	}

	nsPlugin := plugins.CreateNamespaceApplierPlugin(targetNs)
	if err := nsPlugin.Transform(resMap); err != nil {
		return nil, err
	}

	componentName := getComponentName(data)
	if componentName != "" {
		labelsPlugin := plugins.CreateAddLabelsPlugin(componentName)
		if err := labelsPlugin.Transform(resMap); err != nil {
			return nil, err
		}
	}

	objs, resErr := getResources(resMap)
	if resErr != nil {
		return nil, resErr
	}
	return objs, nil
}

func getResources(resMap resmap.ResMap) ([]*unstructured.Unstructured, error) {
	resources := make([]*unstructured.Unstructured, 0, resMap.Size())
	for _, res := range resMap.Resources() {
		u := &unstructured.Unstructured{}
		err := yaml.Unmarshal([]byte(res.MustYaml()), u)
		if err != nil {
			return nil, err
		}
		resources = append(resources, u)
	}

	return resources, nil
}

func (k *kustomizeManifest) MarkAsManaged(objects []*unstructured.Unstructured) {
	markAsManaged(objects)
}

func CreateKustomizeManifestFrom(path string, fsys filesys.FileSystem) *kustomizeManifest { //nolint:golint,revive //No need to export kustomizeManifest.
	return &kustomizeManifest{
		name: filepath.Base(path),
		path: path,
		fsys: fsys,
	}
}

// IsKustomizeManifest checks default filesystem for presence of kustomization file at this path.
func IsKustomizeManifest(path string) bool {
	if filepath.Base(path) == kustomizationFile {
		return true
	}
	_, err := os.Stat(filepath.Join(path, kustomizationFile))
	return err == nil
}

func convertToUnstructuredSlice(resources string) ([]*unstructured.Unstructured, error) {
	splitter := regexp.MustCompile(YamlSeparator)
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

// TODO(mvp): the Manifest structure should be revamped, as these are hard assumptions made on what is passed to kustomized
// TODO(mvp): it would be better to compose a Kustomize-based features with plugins target namespace and component name are intended for
// TODO(mvp): this however, makes the whole Process(data any) a bit blurry, as it is only needed for Templates, making it a wrong abstraction for other "types" of Manifests.
func getTargetNs(_ any) string {
	return "opendatahub"
}

func getComponentName(_ any) string {
	return "kserve"
}
