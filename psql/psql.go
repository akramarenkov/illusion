package psql

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/url"

	"github.com/akramarenkov/illusion/internal/parallel"

	"github.com/google/uuid"
	"github.com/sethvargo/go-password/password"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/network"
	"github.com/testcontainers/testcontainers-go/wait"
)

var (
	ErrGroupNetworkNotCreated = errors.New("network of postgres group was not created")
	ErrGroupNodesNotRunning   = errors.New("nodes of postgres group was not running")
	ErrGroupNotRemoved        = errors.New("postgres group was not removed")
)

const (
	sqlPort    = "5432"
	sqlPortTCP = "5432/tcp"

	defaultDriver             = "postgres"
	defaultPasswordLength     = 16
	defaultPasswordNumDigits  = 4
	defaultPasswordNumSymbols = 4
)

type Cleanup func(ctx context.Context) error

type group struct {
	drivers  []string
	imageTag string

	network *testcontainers.DockerNetwork
	nodes   []*node
}

type node struct {
	container testcontainers.Container
	driver    string
	password  string
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

func Run(ctx context.Context, imageTag string, drivers ...string) ([]url.URL, Cleanup, error) {
	if len(drivers) == 0 {
		drivers = []string{defaultDriver}
	}

	grp := &group{
		drivers:  drivers,
		imageTag: imageTag,
	}

	dsns, err := grp.run(ctx)
	if err != nil {
		if fault := grp.cleanup(ctx); fault != nil {
			return nil, grp.cleanup, errors.Join(err, fault)
		}

		return nil, emptyCleanup, err
	}

	return dsns, grp.cleanup, nil
}

func (grp *group) run(ctx context.Context) ([]url.URL, error) {
	if err := grp.createNetwork(ctx); err != nil {
		return nil, err
	}

	if err := grp.runNodes(ctx); err != nil {
		return nil, err
	}

	dsns := make([]url.URL, len(grp.nodes))

	for id, node := range grp.nodes {
		host, err := node.container.Host(ctx)
		if err != nil {
			return nil, err
		}

		port, err := node.container.MappedPort(ctx, sqlPortTCP)
		if err != nil {
			return nil, err
		}

		dsn := url.URL{
			Scheme:   node.driver,
			User:     url.UserPassword("postgres", node.password),
			Host:     net.JoinHostPort(host, port.Port()),
			Path:     "/",
			RawQuery: url.Values{"sslmode": []string{"disable"}}.Encode(),
		}

		dsns[id] = dsn
	}

	return dsns, nil
}

func (grp *group) cleanup(ctx context.Context) error {
	if err := parallel.Terminate(grp.nodes); err != nil {
		return fmt.Errorf("%w: %w", ErrGroupNotRemoved, err)
	}

	if grp.network != nil {
		if err := grp.network.Remove(ctx); err != nil {
			return fmt.Errorf("%w: %w", ErrGroupNotRemoved, err)
		}

		grp.network = nil
	}

	return nil
}

func (grp *group) createNetwork(ctx context.Context) error {
	netwk, err := network.New(ctx)
	if err != nil {
		return fmt.Errorf("%w: %w", ErrGroupNetworkNotCreated, err)
	}

	grp.network = netwk

	return nil
}

func (grp *group) runNodes(ctx context.Context) error {
	if err := grp.prepareNodeRequests(); err != nil {
		return fmt.Errorf(
			"%w: preparing node requests: %w",
			ErrGroupNodesNotRunning,
			err,
		)
	}

	if err := parallel.Run(ctx, grp.nodes); err != nil {
		return fmt.Errorf("%w: %w", ErrGroupNodesNotRunning, err)
	}

	return nil
}

func (grp *group) prepareNodeRequests() error {
	grp.nodes = make([]*node, len(grp.drivers))

	for id, driver := range grp.drivers {
		hostname, err := prepareHostname()
		if err != nil {
			return err
		}

		pass, err := password.Generate(
			defaultPasswordLength,
			defaultPasswordNumDigits,
			defaultPasswordNumSymbols,
			false,
			false,
		)
		if err != nil {
			return err
		}

		request := testcontainers.GenericContainerRequest{
			ContainerRequest: testcontainers.ContainerRequest{
				Name:     hostname,
				Hostname: hostname,
				Image:    "postgres:" + grp.imageTag,
				ExposedPorts: []string{
					sqlPort,
				},
				Env: map[string]string{
					"POSTGRES_PASSWORD": pass,
				},
				Networks: []string{grp.network.Name},
				WaitingFor: wait.ForAll(
					wait.ForListeningPort(sqlPortTCP),
				),
				Mounts: testcontainers.Mounts(
					testcontainers.VolumeMount("", "/var/lib/postgres"),
				),
			},
			Started: true,
		}

		prepared := &node{
			driver:   driver,
			password: pass,
			req:      request,
		}

		grp.nodes[id] = prepared
	}

	return nil
}

func prepareHostname() (string, error) {
	hostname, err := uuid.NewRandom()
	if err != nil {
		return "", fmt.Errorf("preparing hostname: %w", err)
	}

	return hostname.String(), nil
}

func emptyCleanup(_ context.Context) error {
	return nil
}
