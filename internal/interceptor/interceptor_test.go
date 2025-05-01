package interceptor

import (
	"testing"

	"github.com/akramarenkov/illusion/internal/env"

	"github.com/stretchr/testify/require"
)

func TestMain(m *testing.M) {
	cleanup := Prepare()
	defer cleanup()

	m.Run()
}

func TestRun(t *testing.T) {
	shutdown, err := Run()
	require.NoError(t, err)
	require.NoError(t, shutdown(t.Context()))
}

func TestRunWrongEnvDockerHost(t *testing.T) {
	t.Setenv("DOCKER_HOST", "http://%2F")

	shutdown, err := Run()
	require.Error(t, err)
	require.Nil(t, shutdown)

	t.Setenv("DOCKER_HOST", "http://127.0.0.1/")

	shutdown, err = Run()
	require.Error(t, err)
	require.Nil(t, shutdown)
}

func TestRunWrongEnvInterceptor(t *testing.T) {
	t.Setenv(env.InterceptorUpstream, "unix://%2F/interceptor.sock")

	shutdown, err := Run()
	require.Error(t, err)
	require.Nil(t, shutdown)
}
