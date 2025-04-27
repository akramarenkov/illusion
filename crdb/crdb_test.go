package crdb

import (
	"context"
	"testing"
	"time"

	"github.com/akramarenkov/illusion/internal/interceptor"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/cockroachdb"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/stretchr/testify/require"
)

func TestMain(m *testing.M) {
	cleanup, err := interceptor.Setup()
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

func TestRunCluster(t *testing.T) {
	cln, err := interceptor.Run(t.Context(), nil)
	require.NoError(t, err)

	defer func() {
		require.NoError(t, cln(t.Context()))
	}()

	dsns, cleanup, err := RunCluster(t.Context(), "latest-v25.1", 3)
	defer func() {
		require.NoError(t, cleanup(t.Context()))
	}()

	defer func() {
		require.NoError(t, cleanup(t.Context()))
	}()

	require.NoError(t, err)

	for _, dsn := range dsns {
		migrations, err := migrate.New("file://testdata/migrations", dsn.String())
		require.NoError(t, err)
		require.NoError(t, migrations.Up())
		require.NoError(t, migrations.Down())
	}
}

func TestRunClusterWrongNodesQuantity(t *testing.T) {
	cln, err := interceptor.Run(t.Context(), nil)
	require.NoError(t, err)

	defer func() {
		require.NoError(t, cln(t.Context()))
	}()

	dsns, cleanup, err := RunCluster(t.Context(), "latest-v25.1", -1)
	defer func() {
		require.NoError(t, cleanup(t.Context()))
	}()

	require.Error(t, err)
	require.Nil(t, dsns)

	dsns, cleanup, err = RunCluster(t.Context(), "latest-v25.1", 0)
	defer func() {
		require.NoError(t, cleanup(t.Context()))
	}()

	require.Error(t, err)
	require.Nil(t, dsns)
}

func TestRunClusterWrongTag(t *testing.T) {
	cln, err := interceptor.Run(t.Context(), nil)
	require.NoError(t, err)

	defer func() {
		require.NoError(t, cln(t.Context()))
	}()

	dsns, cleanup, err := RunCluster(t.Context(), "63bc8ecd", 3)
	defer func() {
		require.NoError(t, cleanup(t.Context()))
	}()

	defer func() {
		require.NoError(t, cleanup(t.Context()))
	}()

	require.Error(t, err)
	require.Nil(t, dsns)
}

func TestRunClusterContextCancel(t *testing.T) {
	cln, err := interceptor.Run(t.Context(), nil)
	require.NoError(t, err)

	defer func() {
		require.NoError(t, cln(t.Context()))
	}()

	ctx, cancel := context.WithTimeout(t.Context(), time.Millisecond)
	defer cancel()

	dsns, cleanup, err := RunCluster(ctx, "63bc8ecd", 3)
	defer func() {
		require.NoError(t, cleanup(t.Context()))
	}()

	defer func() {
		require.NoError(t, cleanup(t.Context()))
	}()

	require.Error(t, err)
	require.Nil(t, dsns)
}

func TestPrepareJoin(t *testing.T) {
	hostnames := []string{
		"14862e3d-5ed7-454c-8aa6-0a1b471e959f",
		"c5015fdb-58c0-426a-a71d-205ff26b5f8a",
		"005a670e-84d4-444a-8a63-51899bc9e045",
		"9ec8a69c-8bc7-4517-ba87-eb1f43344224",
		"5dfaa319-7387-4da7-ae23-708f28108f65",
		"4a68fa2b-225d-412d-9067-9b4b65571f5d",
	}

	require.Equal(t,
		"14862e3d-5ed7-454c-8aa6-0a1b471e959f:26357,"+
			"c5015fdb-58c0-426a-a71d-205ff26b5f8a:26357,"+
			"005a670e-84d4-444a-8a63-51899bc9e045:26357,"+
			"9ec8a69c-8bc7-4517-ba87-eb1f43344224:26357,"+
			"5dfaa319-7387-4da7-ae23-708f28108f65:26357,"+
			"4a68fa2b-225d-412d-9067-9b4b65571f5d:26357",
		prepareJoin(hostnames),
	)
}
