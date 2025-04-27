package interceptor

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMain(m *testing.M) {
	cleanup, err := Setup()
	if err != nil {
		panic(err)
	}

	defer func() {
		if err := cleanup(); err != nil {
			panic(err)
		}
	}()

	m.Run()
}

func TestRun(t *testing.T) {
	cleanup, err := Run(t.Context(), nil)
	require.NoError(t, err)
	require.NoError(t, cleanup(t.Context()))
}
