package feature

import (
	"bytes"
	"fmt"
	"html/template"
	"io"
	"io/fs"
	"path/filepath"
	"strings"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
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

	return ConvertToUnstructured(resources)
}

func isTemplate(path string) bool {
	return strings.Contains(filepath.Base(path), ".tmpl.")
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

func loadManifestsFrom(fsys fs.FS, path string) ([]*Manifest, error) {
	var manifests []*Manifest

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
