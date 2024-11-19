//go:build dbtests

package tutils

import (
	"context"
	"fmt"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/osbuild/image-builder/internal/config"
	"github.com/stretchr/testify/require"
)

func conf(t *testing.T) *config.ImageBuilderConfig {
	c := config.ImageBuilderConfig{
		ListenAddress:     "unused",
		LogLevel:          "INFO",
		TernMigrationsDir: "/usr/share/image-builder/migrations-tern",
		PGHost:            "localhost",
		PGPort:            "5432",
		PGDatabase:        "imagebuilder",
		PGSSLMode:         "disable",
	}

	err := config.LoadConfigFromEnv(&c)
	require.NoError(t, err)

	return &c
}

func ConnStr(t *testing.T) string {
	c := conf(t)
	return fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=%s", c.PGUser, c.PGPassword, c.PGHost, c.PGPort, c.PGDatabase, c.PGSSLMode)
}

func Connect(t *testing.T) *pgx.Conn {
	ctx := context.Background()
	conn, err := pgx.Connect(ctx, ConnStr(t))
	require.NoError(t, err)
	return conn
}

func TearDown(t *testing.T) {
	ctx := context.Background()
	conn := Connect(t)
	defer conn.Close(ctx)
	_, err := conn.Exec(ctx, "drop schema public cascade")
	require.NoError(t, err)
	_, err = conn.Exec(ctx, "create schema public")
	require.NoError(t, err)
	_, err = conn.Exec(ctx, "grant all on schema public to postgres")
	require.NoError(t, err)
	_, err = conn.Exec(ctx, "grant all on schema public to public")
	require.NoError(t, err)
}

func MigrateTern(t *testing.T) {
	// run tern migration on top of existing db
	c := conf(t)

	output, err := callTernMigrate(
		context.Background(),
		TernMigrateOptions{
			MigrationsDir: c.TernMigrationsDir,
			DBName:        c.PGDatabase,
			Hostname:      c.PGHost,
			DBPort:        c.PGPort,
			DBUser:        c.PGUser,
			DBPassword:    c.PGPassword,
			SSLMode:       c.PGSSLMode,
		})
	require.NoErrorf(t, err, "tern command failed with non-zero code, combined output: %s", string(output))
}

func RunTest(t *testing.T, f func(*testing.T)) {
	MigrateTern(t)
	defer TearDown(t)
	f(t)
}
