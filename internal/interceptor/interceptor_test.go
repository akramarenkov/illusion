package interceptor

import (
	"testing"

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
