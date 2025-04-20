package parallel

import (
	"runtime"
	"testing"
	"time"

	"github.com/akramarenkov/illusion/internal/imitation"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
)

func TestRun(t *testing.T) {
	nodes := prepareNodes(2 * runtime.NumCPU())

	assert.NoError(t, Run(t.Context(), nodes))
	require.NoError(t, Terminate(nodes))
}

func TestRunFailed(t *testing.T) {
	nodes := prepareNodes(2 * runtime.NumCPU())

	nodes[runtime.NumCPU()-1] = &imitation.Node{
		Req: testcontainers.GenericContainerRequest{
			ContainerRequest: testcontainers.ContainerRequest{
				Image: "alpine:latest",
				Cmd: []string{
					"sleep 60",
				},
			},
			Started: true,
		},
	}

	// Container can be created but not running, in which case an error will also be
	// received
	assert.Error(t, Run(t.Context(), nodes)) //nolint:testifylint // However, the container must be removed
	require.NoError(t, Terminate(nodes))
}

func TestTerminate(t *testing.T) {
	nodes := make([]*imitation.Node, max(2*runtime.NumCPU(), 1024))

	for id := range nodes {
		nodes[id] = &imitation.Node{}
	}

	require.NoError(t, Terminate(nodes))
}

func TestTerminateFailed(t *testing.T) {
	nodes := make([]*imitation.Node, max(2*runtime.NumCPU(), 1024))

	for id := range nodes {
		nodes[id] = &imitation.Node{
			TerminationDuration: time.Second,
		}
	}

	nodes[runtime.NumCPU()+1].IsTerminationFailed = true

	require.Error(t, Terminate(nodes))
}

func BenchmarkTerminate(b *testing.B) {
	nodes := prepareNodes(b.N)

	b.ResetTimer()

	require.NoError(b, Terminate(nodes))
}

func prepareNodes(quantity int) []*imitation.Node {
	nodes := make([]*imitation.Node, quantity)

	for id := range nodes {
		nodes[id] = &imitation.Node{
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
	}

	return nodes
}
