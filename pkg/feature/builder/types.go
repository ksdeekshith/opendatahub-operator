package builder

import (
	"context"

	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/opendatahub-io/opendatahub-operator/v2/pkg/cluster"
)

type ResourceApplier interface {
	Apply(ctx context.Context, cli client.Client, data map[string]any, options ...cluster.MetaOptions) error
}

type ResourceBuilder interface {
	Create() ([]ResourceApplier, error)
}

type ResourceBuilderEnricher interface {
	Enrich(builder ResourceBuilder)
}
