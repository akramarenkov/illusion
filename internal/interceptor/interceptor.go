// Intercepts requests to the container manager and provide to decide which ones
// to skip. Can only be used in tests.
package interceptor

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"time"

	"github.com/akramarenkov/illusion/internal/env"

	"github.com/akramarenkov/wrecker/httpw"
)

const defaultDockerHost = "unix:///var/run/docker.sock"

// Cleans up interceptor environment.
type Cleanup func()

// Shuts down interceptor.
type Shutdown func(ctx context.Context) error

// Prepares environment for interceptor. Must be called in [TestMain] function. Panics
// when something goes wrong.
//
// [Cleanup] function must be called when the [TestMain] function completes.
func Prepare() Cleanup {
	upstream := os.Getenv("DOCKER_HOST")

	if upstream == "" {
		upstream = defaultDockerHost
	}

	// Using environment variables makes it easy to setup the interceptor for each
	// package individually. It is more difficult to achieve the same behavior when
	// using a global variables
	if err := os.Setenv(env.InterceptorUpstream, upstream); err != nil {
		panic(err)
	}

	tempDir, err := os.MkdirTemp(os.TempDir(), "")
	if err != nil {
		panic(err)
	}

	purge := func() error {
		return os.RemoveAll(tempDir)
	}

	socket := url.URL{
		Scheme: "unix",
		Path:   filepath.Join(tempDir, "interceptor.sock"),
	}

	if err := os.Setenv("DOCKER_HOST", socket.String()); err != nil {
		panic(errors.Join(err, purge()))
	}

	cleanup := func() {
		if err := purge(); err != nil {
			panic(err)
		}
	}

	return cleanup
}

// Runs interceptor. Requests can be interrupted using blockers.
//
// [Shutdown] function must be called when the test function completes if
// [Run] did not return an error.
func Run(blockers ...httpw.Blocker) (Shutdown, error) {
	listen, err := url.Parse(os.Getenv("DOCKER_HOST"))
	if err != nil {
		return nil, err
	}

	upstream, err := url.Parse(os.Getenv(env.InterceptorUpstream))
	if err != nil {
		return nil, err
	}

	if upstream.Scheme == "unix" {
		upstream.Scheme = httpw.UnixSchemeHTTP
	}

	opts := httpw.Opts{
		Network:  "unix",
		Address:  listen.Path,
		Upstream: upstream.String(),
		Blockers: blockers,
		Server: &http.Server{
			ReadTimeout: time.Minute,
			IdleTimeout: time.Minute,
		},
	}

	wrecker, err := httpw.Run(opts)
	if err != nil {
		return nil, err
	}

	shutdown := func(ctx context.Context) error {
		if err := wrecker.Shutdown(ctx); err != nil {
			return err
		}

		if err := <-wrecker.Err(); !errors.Is(err, http.ErrServerClosed) {
			return err
		}

		return nil
	}

	return shutdown, nil
}
