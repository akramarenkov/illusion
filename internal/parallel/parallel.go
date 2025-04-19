package parallel

import (
	"context"
	"runtime"
	"sync"

	"github.com/testcontainers/testcontainers-go"
)

type Prototype interface {
	Request() testcontainers.GenericContainerRequest
	Set(created testcontainers.Container)
}

type Running interface {
	Get() testcontainers.Container
}

func Run[Type Prototype](ctx context.Context, runnable []Type) error {
	parallelization := min(runtime.NumCPU(), len(runnable))

	ids := make(chan int)
	errs := make(chan error)
	breaker := make(chan struct{})

	var wg sync.WaitGroup

	go func() {
		defer close(ids)

		for id := range runnable {
			select {
			case <-breaker:
				return
			case ids <- id:
			}
		}
	}()

	wg.Add(parallelization)

	for range parallelization {
		go func() {
			defer wg.Done()

			for id := range ids {
				container, err := testcontainers.GenericContainer(
					ctx,
					runnable[id].Request(),
				)

				// Container can be created but not running, in which case an error
				// will also be received. However, the container must be removed
				runnable[id].Set(container)

				errs <- err
			}
		}()
	}

	go func() {
		wg.Wait()
		close(errs)
	}()

	for err := range errs {
		if err != nil {
			close(breaker)

			for e := range errs {
				_ = e
			}

			return err
		}
	}

	return nil
}

func Terminate[Type Running](terminable []Type, opts ...testcontainers.TerminateOption) error {
	parallelization := min(runtime.NumCPU(), len(terminable))

	ids := make(chan int)
	errs := make(chan error)
	breaker := make(chan struct{})

	var wg sync.WaitGroup

	go func() {
		defer close(ids)

		for id := range terminable {
			select {
			case <-breaker:
				return
			case ids <- id:
			}
		}
	}()

	wg.Add(parallelization)

	for range parallelization {
		go func() {
			defer wg.Done()

			for id := range ids {
				errs <- testcontainers.TerminateContainer(terminable[id].Get(), opts...)
			}
		}()
	}

	go func() {
		wg.Wait()
		close(errs)
	}()

	for err := range errs {
		if err != nil {
			close(breaker)

			for e := range errs {
				_ = e
			}

			return err
		}
	}

	return nil
}
