package manifest

import (
	"io/fs"

	"github.com/opendatahub-io/opendatahub-operator/v2/pkg/feature/builder"
)

type Builder struct {
	manifestLocation fs.FS
	paths            []string
}

// Location sets the root file system from which manifest paths are loaded.
func Location(fsys fs.FS) *Builder {
	return &Builder{manifestLocation: fsys}
}

// Include loads manifests from the provided paths.
func (b *Builder) Include(paths ...string) *Builder {
	b.paths = append(b.paths, paths...)
	return b
}

func (b *Builder) Create() ([]builder.ResourceApplier, error) {
	var manifests []*Manifest
	for _, path := range b.paths {
		currManifests, err := LoadManifests(b.manifestLocation, path)
		if err != nil {
			return nil, err // TODO wrap
		}

		manifests = append(manifests, currManifests...)
	}

	resources := make([]builder.ResourceApplier, 0, len(manifests))
	for _, m := range manifests {
		resources = append(resources, CreateApplier(m))
	}

	return resources, nil
}
