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

func (nd *Node) Get() testcontainers.Container {
	return nd
}

func (nd *Node) Set(container testcontainers.Container) {
	nd.Container = container
}

func (nd *Node) Request() testcontainers.GenericContainerRequest {
	return nd.Req
}

func (nd *Node) Terminate(ctx context.Context, opts ...testcontainers.TerminateOption) error {
	if nd.IsTerminationFailed {
		return ErrTerminationFailed
	}

	if nd.Container == nil {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(nd.TerminationDuration):
		}

		return nil
	}

	start := time.Now()

	if err := nd.Container.Terminate(ctx, opts...); err != nil {
		return err
	}

	time.Sleep(nd.TerminationDuration - time.Since(start))

	return nil
}
