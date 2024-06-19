package manifest

import (
	"bytes"
	"context"
	"fmt"
	"html/template"
	"io"
	"io/fs"
	"path/filepath"
	"strings"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/opendatahub-io/opendatahub-operator/v2/pkg/cluster"
)

type ApplierFunc func(ctx context.Context, cli client.Client, objects []*unstructured.Unstructured, options ...cluster.MetaOptions) error

func Create(fsys fs.FS, path string) *Manifest {
	basePath := filepath.Base(path)
	return &Manifest{
		name:  basePath,
		path:  path,
		patch: isPatch(basePath),
		fsys:  fsys,
	}
}

func LoadManifests(fsys fs.FS, path string) ([]*Manifest, error) {
	var manifests []*Manifest

	err := fs.WalkDir(fsys, path, func(path string, dirEntry fs.DirEntry, errWalk error) error {
		if errWalk != nil {
			return errWalk
		}

		if _, err := dirEntry.Info(); err != nil {
			return err
		}

		if dirEntry.IsDir() {
			return nil
		}

		manifests = append(manifests, Create(fsys, path))

		return nil
	})

	if err != nil {
		return nil, err
	}

	return manifests, nil
}

type Manifest struct {
	name,
	path string
	patch bool
	fsys  fs.FS
}

// Applier wraps an instance of Manifest and provides a way to apply it to the cluster.
type Applier struct {
	ctx         context.Context
	cli         client.Client
	options     []cluster.MetaOptions
	applierFunc ApplierFunc
	manifest    *Manifest
	data        any
}

func CreateApplier(ctx context.Context, cli client.Client, manifest *Manifest, data any, options ...cluster.MetaOptions) *Applier {
	applierFunc := cluster.ApplyResources
	if isPatch(manifest.path) {
		applierFunc = func(ctx context.Context, cli client.Client, objects []*unstructured.Unstructured, _ ...cluster.MetaOptions) error {
			return cluster.PatchResources(ctx, cli, objects)
		}
	}
	return &Applier{
		ctx:         ctx,
		cli:         cli,
		options:     options,
		applierFunc: applierFunc,
		manifest:    manifest,
		data:        data,
	}
}

// Apply processes owned manifest and apply it to a cluster.
func (a *Applier) Apply() error {
	objects, errProcess := a.manifest.Process(a.data)
	if errProcess != nil {
		return errProcess
	}

	return a.applierFunc(a.ctx, a.cli, objects, a.options...)
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

	return cluster.ConvertToUnstructured(resources)
}

func isTemplate(path string) bool {
	return strings.Contains(filepath.Base(path), ".tmpl.")
}

func isPatch(path string) bool {
	return strings.Contains(filepath.Base(path), ".patch")
}
