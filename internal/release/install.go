package release

import (
	"context"
	"fmt"
	"time"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

func Install(ctx context.Context, c client.Client, bundle Bundle, gates Gates, interval, groupTimeout time.Duration, report func(group Group, err error)) error {
	previous, err := ReadInventory(ctx, c)
	if err != nil {
		return err
	}
	applier := Applier{Client: c}
	for _, group := range bundle.Groups {
		groupCtx, cancel := groupContext(ctx, groupTimeout)
		err := applier.Apply(groupCtx, group)
		if err == nil {
			err = WaitGroup(groupCtx, c, gates, group, interval)
		}
		cancel()
		report(group, err)
		if err != nil {
			return fmt.Errorf("group %s: %w", group.Name, err)
		}
	}
	current := InventoryOf(bundle.Groups)
	if err := applier.Prune(ctx, previous, current); err != nil {
		return err
	}
	return WriteInventory(ctx, c, current)
}

func groupContext(ctx context.Context, timeout time.Duration) (context.Context, context.CancelFunc) {
	if timeout <= 0 {
		return context.WithCancel(ctx)
	}
	return context.WithTimeout(ctx, timeout)
}
