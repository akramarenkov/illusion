package imitation

import (
	"context"
	"errors"
	"time"

	"github.com/testcontainers/testcontainers-go"
)

var ErrTerminationFailed = errors.New("termination failed")

type Node struct {
	IsTerminationFailed bool
	TerminationDuration time.Duration

	Req testcontainers.GenericContainerRequest
	testcontainers.Container
}

func (node *Node) Get() testcontainers.Container {
	return node
}

func (node *Node) Set(container testcontainers.Container) {
	node.Container = container
}

func (node *Node) Request() testcontainers.GenericContainerRequest {
	return node.Req
}

func (node *Node) Terminate(ctx context.Context, opts ...testcontainers.TerminateOption) error {
	if node.IsTerminationFailed {
		return ErrTerminationFailed
	}

	if node.Container == nil {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(node.TerminationDuration):
		}

		return nil
	}

	start := time.Now()

	if err := node.Container.Terminate(ctx, opts...); err != nil {
		return err
	}

	time.Sleep(node.TerminationDuration - time.Since(start))

	return nil
}
