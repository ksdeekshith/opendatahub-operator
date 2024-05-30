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

	featurev1 "github.com/opendatahub-io/opendatahub-operator/v2/apis/features/v1"
	"github.com/opendatahub-io/opendatahub-operator/v2/pkg/metadata/annotations"
	"github.com/opendatahub-io/opendatahub-operator/v2/pkg/plugins"
)

const kustomizationFile = "kustomization.yaml"

type Manifest interface {
	// Process allows any arbitrary struct to be passed and used while processing the content of the manifest.
	Process(data any) ([]*unstructured.Unstructured, error)
	// MarkAsManaged sets all non-patch objects to be managed/reconciled by setting the annotation.
	MarkAsManaged(objects []*unstructured.Unstructured)
}

type rawManifest struct {
	name,
	path string
	patch bool
	fsys  fs.FS
}

var _ Manifest = (*rawManifest)(nil)

func (b *rawManifest) Process(_ any) ([]*unstructured.Unstructured, error) {
	manifestFile, err := b.fsys.Open(b.path)
	if err != nil {
		return nil, err
	}
	defer manifestFile.Close()

	content, err := io.ReadAll(manifestFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}
	resources := string(content)

	return convertToUnstructuredSlice(resources)
}

func (b *rawManifest) MarkAsManaged(objects []*unstructured.Unstructured) {
	if !b.patch {
		markAsManaged(objects)
	}
}

var _ Manifest = (*templateManifest)(nil)

type templateManifest struct {
	name,
	path string
	patch bool
	fsys  fs.FS
}

func (t *templateManifest) Process(data any) ([]*unstructured.Unstructured, error) {
	manifestFile, err := t.fsys.Open(t.path)
	if err != nil {
		return nil, err
	}
	defer manifestFile.Close()

	content, err := io.ReadAll(manifestFile)
	if err != nil {
		return nil, fmt.Errorf("failed to create file: %w", err)
	}

	tmpl, err := template.New(t.name).
		Option("missingkey=error").
		Funcs(template.FuncMap{"ReplaceChar": ReplaceChar}).
		Parse(string(content))
	if err != nil {
		return nil, fmt.Errorf("failed to parse template: %w", err)
	}

	var buffer bytes.Buffer
	if err := tmpl.Execute(&buffer, data); err != nil {
		return nil, fmt.Errorf("failed to execute template: %w", err)
	}

	resources := buffer.String()

	return convertToUnstructuredSlice(resources)
}

func (t *templateManifest) MarkAsManaged(objects []*unstructured.Unstructured) {
	if !t.patch {
		markAsManaged(objects)
	}
}

var _ Manifest = (*kustomizeManifest)(nil)

// kustomizeManifest supports paths to kustomization files / directories containing a kustomization file
// note that it only supports to paths within the mounted files ie: /opt/manifests.
type kustomizeManifest struct {
	name,
	path string // path is to the directory containing a kustomization.yaml file within it or path to kust file itself
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

	if err := plugins.ApplyNamespacePlugin(targetNs, resMap); err != nil {
		return nil, err
	}

	componentName := getComponentName(data)
	if componentName != "" {
		if err := plugins.ApplyAddLabelsPlugin(componentName, resMap); err != nil {
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

func loadManifestsFrom(fsys fs.FS, path string) ([]Manifest, error) {
	var manifests []Manifest
	// check local filesystem for kustomization manifest
	if isKustomizeManifest(path) {
		m := CreateKustomizeManifestFrom(path, filesys.MakeFsOnDisk())
		manifests = append(manifests, m)
		return manifests, nil
	}

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
		if isTemplateManifest(path) {
			manifests = append(manifests, CreateTemplateManifestFrom(fsys, path))
		} else {
			manifests = append(manifests, CreateRawManifestFrom(fsys, path))
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	return manifests, nil
}

func CreateRawManifestFrom(fsys fs.FS, path string) *rawManifest { //nolint:golint,revive //No need to export rawManifest.
	basePath := filepath.Base(path)

	return &rawManifest{
		name:  basePath,
		path:  path,
		patch: strings.Contains(basePath, ".patch"),
		fsys:  fsys,
	}
}

func CreateTemplateManifestFrom(fsys fs.FS, path string) *templateManifest { //nolint:golint,revive //No need to export templateManifest.
	basePath := filepath.Base(path)

	return &templateManifest{
		name:  basePath,
		path:  path,
		patch: strings.Contains(basePath, ".patch."),
		fsys:  fsys,
	}
}

func CreateKustomizeManifestFrom(path string, fsys filesys.FileSystem) *kustomizeManifest { //nolint:golint,revive //No need to export kustomizeManifest.
	return &kustomizeManifest{
		name: filepath.Base(path),
		path: path,
		fsys: fsys,
	}
}

// isKustomizeManifest checks default filesystem for presence of kustomization file at this path.
func isKustomizeManifest(path string) bool {
	if filepath.Base(path) == kustomizationFile {
		return true
	}
	_, err := os.Stat(filepath.Join(path, kustomizationFile))
	return err == nil
}

func isTemplateManifest(path string) bool {
	return strings.Contains(filepath.Base(path), ".tmpl.")
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

func getTargetNs(data any) string {
	if spec, ok := data.(*Spec); ok {
		return spec.TargetNamespace
	}
	return ""
}

func getComponentName(data any) string {
	if featSpec, ok := data.(*Spec); ok {
		source := featSpec.Source
		if source == nil {
			return ""
		}
		if source.Type == featurev1.ComponentType {
			return source.Name
		}
		return ""
	}
	return ""
}
