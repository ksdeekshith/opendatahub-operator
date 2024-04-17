package capabilities

import (
	"context"
	"fmt"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ Handler = (*AuthorizationHandler)(nil)

type AuthorizationHandler struct {
	hooksRegistry
}

// Configure enables the Authorization capability and component-specific configuration through registered hooks.
func (a *AuthorizationHandler) Configure(ctx context.Context, cli client.Client) error {
	for _, hook := range a.hooksRegistry.configHooks {
		if err := hook(ctx, cli); err != nil {
			return err
		}
	}

	// TODO https://issues.redhat.com/browse/RHOAIENG-6016
	// TODO installation path deploy.DeployManifestsFromPath()
	fmt.Println(">>>>> TODO: deploy authz controller")
	return nil
}

func (a *AuthorizationHandler) Remove(ctx context.Context, cli client.Client) error {
	for _, hook := range a.hooksRegistry.configHooks {
		if err := hook(ctx, cli); err != nil {
			return err
		}
	}

	// TODO implement removal/teardown
	return nil
}

type hooksRegistry struct {
	configHooks []HookFunc
	removeHooks []HookFunc
}

func (c *hooksRegistry) AddConfigureHook(hook HookFunc) {
	c.configHooks = append(c.configHooks, hook)
}

func (c *hooksRegistry) AddRemoveHook(hook HookFunc) {
	c.removeHooks = append(c.removeHooks, hook)
}
