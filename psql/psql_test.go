package psql

import (
	"testing"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/pgx/v5"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/stretchr/testify/require"
)

func TestRun(t *testing.T) {
	dsns, cleanup, err := Run(t.Context(), "17", "postgres", "pgx5")
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
