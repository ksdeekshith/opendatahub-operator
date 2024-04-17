package capabilities

import (
	"context"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

// HookFunc is a function that can be used to configure the capability hook per component.
type HookFunc func(ctx context.Context, cli client.Client) error

// Handler is an interface that defines the capability management steps for given capability.
type Handler interface {
	AddConfigureHook(HookFunc)
	Configure(ctx context.Context, cli client.Client) error
	AddRemoveHook(HookFunc)
	Remove(ctx context.Context, cli client.Client) error
}
