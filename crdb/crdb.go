package crdb

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/url"
	"time"

	"github.com/akramarenkov/illusion/internal/parallel"

	"github.com/google/uuid"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/network"
	"github.com/testcontainers/testcontainers-go/wait"
	"github.com/tidwall/gjson"
)

var (
	ErrClusterNetworkNotCreated = errors.New("cluster network was not created")
	ErrClusterNodesNotRunning   = errors.New("cluster nodes was not running")
	ErrClusterNotInitialized    = errors.New("cluster was not initialized")
	ErrClusterNotRemoved        = errors.New("cluster was not removed")
	ErrNodesQuantityNegative    = errors.New("nodes quantity is negative")
	ErrNodesQuantityZero        = errors.New("nodes quantity is zero")
)

const (
	advertisePort = "26357"
	httpPort      = "8080"
	httpPortTCP   = "8080/tcp"
	sqlPort       = "26257"
	sqlPortTCP    = "26257/tcp"
)

type Cleanup func(ctx context.Context) error

type node struct {
	container testcontainers.Container
	req       testcontainers.GenericContainerRequest
}

func (n *node) Get() testcontainers.Container {
	return n.container
}

func (n *node) Set(created testcontainers.Container) {
	n.container = created
}

func (n *node) Request() testcontainers.GenericContainerRequest {
	return n.req
}

type cluster struct {
	imageTag      string
	nodesQuantity int

	network *testcontainers.DockerNetwork
	nodes   []*node
}

func RunCluster(ctx context.Context, imageTag string, nodesQuantity int) ([]url.URL, Cleanup, error) {
	if nodesQuantity < 0 {
		return nil, nil, ErrNodesQuantityNegative
	}

	if nodesQuantity == 0 {
		return nil, nil, ErrNodesQuantityZero
	}

	clt := &cluster{
		imageTag:      imageTag,
		nodesQuantity: nodesQuantity,
	}

	dsns, err := clt.run(ctx)
	if err != nil {
		return nil, nil, errors.Join(err, clt.cleanup(ctx))
	}

	return dsns, clt.cleanup, nil
}

func (clt *cluster) run(ctx context.Context) ([]url.URL, error) {
	if err := clt.createNetwork(ctx); err != nil {
		return nil, err
	}

	if err := clt.runNodes(ctx); err != nil {
		return nil, err
	}

	if err := clt.initialize(ctx); err != nil {
		return nil, err
	}

	dsns := make([]url.URL, len(clt.nodes))

	for id, node := range clt.nodes {
		host, err := node.container.Host(ctx)
		if err != nil {
			return nil, err
		}

		port, err := node.container.MappedPort(ctx, sqlPortTCP)
		if err != nil {
			return nil, err
		}

		dsn := url.URL{
			Scheme:   "cockroach",
			User:     url.User("root"),
			Host:     net.JoinHostPort(host, port.Port()),
			Path:     "/",
			RawQuery: url.Values{"sslmode": []string{"disable"}}.Encode(),
		}

		dsns[id] = dsn
	}

	return dsns, nil
}

func (clt *cluster) cleanup(ctx context.Context) error {
	if err := parallel.Terminate(clt.nodes); err != nil {
		return fmt.Errorf("%w: %w", ErrClusterNotRemoved, err)
	}

	if clt.network != nil {
		if err := clt.network.Remove(ctx); err != nil {
			return fmt.Errorf("%w: %w", ErrClusterNotRemoved, err)
		}

		clt.network = nil
	}

	return nil
}

func (clt *cluster) createNetwork(ctx context.Context) error {
	netwk, err := network.New(ctx)
	if err != nil {
		return fmt.Errorf("%w: %w", ErrClusterNetworkNotCreated, err)
	}

	clt.network = netwk

	return nil
}

func (clt *cluster) runNodes(ctx context.Context) error {
	if err := clt.prepareNodeRequests(); err != nil {
		return fmt.Errorf(
			"%w: preparing node requests: %w",
			ErrClusterNodesNotRunning,
			err,
		)
	}

	if err := parallel.Run(ctx, clt.nodes); err != nil {
		return fmt.Errorf("%w: %w", ErrClusterNodesNotRunning, err)
	}

	return nil
}

func (clt *cluster) prepareNodeRequests() error {
	hostnames, err := prepareHostnames(clt.nodesQuantity)
	if err != nil {
		return err
	}

	join := prepareJoin(hostnames)

	clt.nodes = make([]*node, clt.nodesQuantity)

	for id, hostname := range hostnames {
		advertiseAddr := net.JoinHostPort(hostname, advertisePort)
		httpAddr := net.JoinHostPort(hostname, httpPort)
		sqlAddr := net.JoinHostPort(hostname, sqlPort)

		request := testcontainers.GenericContainerRequest{
			ContainerRequest: testcontainers.ContainerRequest{
				Name:     hostname,
				Hostname: hostname,
				Image:    "cockroachdb/cockroach:" + clt.imageTag,
				ExposedPorts: []string{
					httpPort,
					sqlPort,
				},
				Networks: []string{clt.network.Name},
				WaitingFor: wait.ForAll(
					wait.ForListeningPort(httpPortTCP),
					wait.ForListeningPort(sqlPortTCP),
				),
				Mounts: testcontainers.Mounts(
					testcontainers.VolumeMount("", "/cockroach/cockroach-data"),
				),
				Cmd: []string{
					"start",
					"--advertise-addr",
					advertiseAddr,
					"--http-addr",
					httpAddr,
					"--listen-addr",
					advertiseAddr,
					"--sql-addr",
					sqlAddr,
					"--insecure",
					"--join",
					join,
				},
			},
			Started: true,
		}

		prepared := &node{
			req: request,
		}

		clt.nodes[id] = prepared
	}

	return nil
}

func (clt *cluster) initialize(ctx context.Context) error {
	node := clt.nodes[0]

	info, err := node.container.Inspect(ctx)
	if err != nil {
		return fmt.Errorf("%w: inspecting node: %w", ErrClusterNotInitialized, err)
	}

	code, _, err := node.container.Exec(
		ctx,
		[]string{
			"cockroach",
			"init",
			"--host",
			info.Config.Hostname,
			"--port",
			advertisePort,
			"--insecure",
		},
	)
	if err != nil {
		return fmt.Errorf("%w: init: %w", ErrClusterNotInitialized, err)
	}

	if code != 0 {
		return fmt.Errorf("%w: init exit code: %d", ErrClusterNotInitialized, code)
	}

	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("%w: status: %w", ErrClusterNotInitialized, ctx.Err())
		case <-ticker.C:
			code, reader, err := node.container.Exec(
				ctx,
				[]string{
					"cockroach",
					"node",
					"status",
					"--host",
					info.Config.Hostname,
					"--port",
					sqlPort,
					"--insecure",
					"--format",
					"ndjson",
				},
			)
			if err != nil {
				return fmt.Errorf("%w: status: %w", ErrClusterNotInitialized, err)
			}

			if code != 0 {
				return fmt.Errorf(
					"%w: status exit code: %d",
					ErrClusterNotInitialized,
					code,
				)
			}

			output, err := io.ReadAll(reader)
			if err != nil {
				return fmt.Errorf("%w: read output: %w", ErrClusterNotInitialized, err)
			}

			joined := gjson.GetBytes(output, "..#(is_available=true)#").
				Get("#(is_live=true)#").
				Get("#")

			if joined.Int() == int64(clt.nodesQuantity) {
				return nil
			}
		}
	}
}

func prepareHostnames(quantity int) ([]string, error) {
	hostnames := make([]string, quantity)

	for id := range hostnames {
		hostname, err := uuid.NewRandom()
		if err != nil {
			return nil, fmt.Errorf("preparing hostname: %w", err)
		}

		hostnames[id] = hostname.String()
	}

	return hostnames, nil
}

func prepareJoin(hostnames []string) string {
	var join string

	for id, hostname := range hostnames {
		if id != 0 {
			join += ","
		}

		advertiseAddr := net.JoinHostPort(hostname, advertisePort)

		join += advertiseAddr
	}

	return join
}
