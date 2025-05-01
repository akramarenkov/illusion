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

type Cleanup func(ctx context.Context) error

func Setup() (func() error, error) {
	// Using environment variables makes it easy to setup the interceptor for each
	// package individually. It is more difficult to achieve the same behavior when
	// using a global variable
	if err := os.Setenv(env.InterceptorUpstream, os.Getenv("DOCKER_HOST")); err != nil {
		return nil, err
	}

	tempDir, err := os.MkdirTemp(os.TempDir(), "")
	if err != nil {
		return nil, err
	}

	cleanup := func() error {
		return os.RemoveAll(tempDir)
	}

	socket := url.URL{
		Scheme: "unix",
		Path:   filepath.Join(tempDir, "interceptor.sock"),
	}

	if err := os.Setenv("DOCKER_HOST", socket.String()); err != nil {
		return nil, errors.Join(err, cleanup())
	}

	return cleanup, nil
}

func Run(deciders ...httpw.Decider) (Cleanup, error) {
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
		Deciders: deciders,
		Server: &http.Server{
			ReadTimeout: time.Minute,
			IdleTimeout: time.Minute,
		},
	}

	wrecker, err := httpw.Run(opts)
	if err != nil {
		return nil, err
	}

	cleanup := func(ctx context.Context) error {
		if err := wrecker.Shutdown(ctx); err != nil {
			return err
		}

		if !errors.Is(<-wrecker.Err(), http.ErrServerClosed) {
			return err
		}

		return nil
	}

	return cleanup, nil
}
