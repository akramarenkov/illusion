package imitation

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
)

func TestNode(t *testing.T) {
	node := Node{
		Req: testcontainers.GenericContainerRequest{
			ContainerRequest: testcontainers.ContainerRequest{
				Image: "alpine:latest",
				Cmd: []string{
					"sh",
					"-c",
					"sleep 60",
				},
			},
			Started: true,
		},
	}

	// Container can be created but not running, in which case an error will also be
	// received
	container, err := testcontainers.GenericContainer(t.Context(), node.Request())
	assert.NoError(t, err) //nolint:testifylint // However, the container must be removed
	require.NotNil(t, container)

	node.Set(container)

	require.NoError(t, testcontainers.TerminateContainer(node.Get()))
	require.Error(t, node.Get().Terminate(t.Context()))
	require.NoError(t, testcontainers.TerminateContainer(node.Get()))
}

func TestNodePseudoTerminate(t *testing.T) {
	var node Node

	start := time.Now()

	require.NoError(t, node.Terminate(t.Context()))
	require.Less(t, time.Since(start), 50*time.Millisecond)
}

func TestNodePseudoTerminateDuration(t *testing.T) {
	const expectedDuration = time.Second

	node := Node{
		TerminationDuration: expectedDuration,
	}

	start := time.Now()

	require.NoError(t, node.Terminate(t.Context()))
	require.InEpsilon(t, expectedDuration, time.Since(start), 0.1)
}

func TestNodePseudoTerminateFailed(t *testing.T) {
	node := Node{
		IsTerminationFailed: true,
		TerminationDuration: time.Second,
	}

	start := time.Now()

	require.Error(t, node.Terminate(t.Context()))
	require.Less(t, time.Since(start), 50*time.Millisecond)
}

func TestNodePseudoTerminateContextCancel(t *testing.T) {
	const expectedDuration = time.Second

	node := Node{
		TerminationDuration: 10 * expectedDuration,
	}

	ctx, cancel := context.WithTimeout(t.Context(), expectedDuration)
	defer cancel()

	start := time.Now()

	require.Error(t, node.Terminate(ctx))
	require.InEpsilon(t, expectedDuration, time.Since(start), 0.1)
}
