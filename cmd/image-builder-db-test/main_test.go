// +build integration

package main

import (
	"context"
	"fmt"
	"os/exec"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v4"
	"github.com/stretchr/testify/require"

	"github.com/osbuild/image-builder/internal/config"
	"github.com/osbuild/image-builder/internal/db"
)

const (
	ANR1 = "000001"
	ANR2 = "000002"
	ANR3 = "000003"

	ORGID1 = "100000"
	ORGID2 = "100001"
	ORGID3 = "100002"

	fortnight = time.Hour * 24 * 14
)

func conf(t *testing.T) *config.ImageBuilderConfig {
	c := config.ImageBuilderConfig{
		ListenAddress:     "unused",
		LogLevel:          "INFO",
		TernMigrationsDir: "/usr/share/image-builder/migrations-tern",
		PGHost:            "localhost",
		PGPort:            "5432",
		PGDatabase:        "imagebuilder",
		PGUser:            "postgres",
		PGPassword:        "foobar",
		PGSSLMode:         "disable",
	}

	err := config.LoadConfigFromEnv(&c)
	require.NoError(t, err)

	return &c
}

func connStr(t *testing.T) string {
	c := conf(t)
	return fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=%s", c.PGUser, c.PGPassword, c.PGHost, c.PGPort, c.PGDatabase, c.PGSSLMode)
}

func migrateTern(t *testing.T) {
	// run tern migration on top of existing db
	c := conf(t)

	cmd := exec.Command("tern", "migrate",
		"-m", c.TernMigrationsDir,
		"--host", c.PGHost, "--port", c.PGPort,
		"--user", c.PGUser, "--password", c.PGPassword,
		"--sslmode", c.PGSSLMode)
	err := cmd.Run()
	require.NoError(t, err)
}

func connect(t *testing.T) *pgx.Conn {
	conn, err := pgx.Connect(context.Background(), connStr(t))
	require.NoError(t, err)
	return conn
}

func tearDown(t *testing.T) {
	conn := connect(t)
	defer conn.Close(context.Background())
	conn.Exec(context.Background(), "drop table clones")
	conn.Exec(context.Background(), "drop table composes")
	conn.Exec(context.Background(), "drop table if exists schema_migrations")
	conn.Exec(context.Background(), "drop table if exists schema_version")
}

func testInsertCompose(t *testing.T) {
	d, err := db.InitDBConnectionPool(connStr(t))
	require.NoError(t, err)

	imageName := "MyImageName"

	migrateTern(t)

	// test
	err = d.InsertCompose(uuid.New(), ANR1, ORGID1, &imageName, []byte("{}"))
	require.NoError(t, err)
	err = d.InsertCompose(uuid.New(), "", ORGID1, &imageName, []byte("{}"))
	require.NoError(t, err)
}

func testGetCompose(t *testing.T) {
	d, err := db.InitDBConnectionPool(connStr(t))
	require.NoError(t, err)

	imageName := "MyImageName"

	err = d.InsertCompose(uuid.New(), ANR1, ORGID1, &imageName, []byte("{}"))
	require.NoError(t, err)
	err = d.InsertCompose(uuid.New(), ANR1, ORGID1, &imageName, []byte("{}"))
	require.NoError(t, err)
	err = d.InsertCompose(uuid.New(), ANR1, ORGID1, &imageName, []byte("{}"))
	require.NoError(t, err)
	err = d.InsertCompose(uuid.New(), ANR1, ORGID1, &imageName, []byte("{}"))
	require.NoError(t, err)

	// test
	// GetComposes works as expected
	composes, count, err := d.GetComposes(ORGID1, fortnight, 100, 0)
	require.NoError(t, err)
	require.Equal(t, 4, count)
	require.Equal(t, 4, len(composes))

	// count returns total in db, ignoring limits
	composes, count, err = d.GetComposes(ORGID1, fortnight, 1, 2)
	require.NoError(t, err)
	require.Equal(t, 4, count)
	require.Equal(t, 1, len(composes))

	// GetCompose works as expected
	compose, err := d.GetCompose(composes[0].Id, ORGID1)
	require.NoError(t, err)
	require.Equal(t, composes[0], *compose)

	// cross-account compose access not allowed
	compose, err = d.GetCompose(composes[0].Id, ORGID2)
	require.Equal(t, db.ComposeNotFoundError, err)
	require.Nil(t, compose)

}

func testCountComposesSince(t *testing.T) {
	d, err := db.InitDBConnectionPool(connStr(t))
	require.NoError(t, err)

	imageName := "MyImageName"

	conn := connect(t)
	defer conn.Close(context.Background())
	insert := "INSERT INTO composes(job_id, request, created_at, account_number, org_id, image_name) VALUES ($1, $2, CURRENT_TIMESTAMP - interval '2 days', $3, $4, $5)"
	_, err = conn.Exec(context.Background(), insert, uuid.New().String(), "{}", ANR3, ORGID3, &imageName)
	insert = "INSERT INTO composes(job_id, request, created_at, account_number, org_id, image_name) VALUES ($1, $2, CURRENT_TIMESTAMP - interval '3 days', $3, $4, $5)"
	_, err = conn.Exec(context.Background(), insert, uuid.New().String(), "{}", ANR3, ORGID3, &imageName)
	insert = "INSERT INTO composes(job_id, request, created_at, account_number, org_id, image_name) VALUES ($1, $2, CURRENT_TIMESTAMP - interval '4 days', $3, $4, $5)"
	_, err = conn.Exec(context.Background(), insert, uuid.New().String(), "{}", ANR3, ORGID3, &imageName)

	// Verify quering since an interval
	count, err := d.CountComposesSince(ORGID3, 24*time.Hour)
	require.NoError(t, err)
	require.Equal(t, 0, count)

	count, err = d.CountComposesSince(ORGID3, 48*time.Hour+time.Second)
	require.NoError(t, err)
	require.Equal(t, 1, count)

	count, err = d.CountComposesSince(ORGID3, 72*time.Hour+time.Second)
	require.NoError(t, err)
	require.Equal(t, 2, count)

	count, err = d.CountComposesSince(ORGID3, 96*time.Hour+time.Second)
	require.NoError(t, err)
	require.Equal(t, 3, count)
}

func testCountGetComposesSince(t *testing.T) {
	d, err := db.InitDBConnectionPool(connStr(t))
	require.NoError(t, err)

	conn := connect(t)
	defer conn.Close(context.Background())

	job1 := uuid.New()
	insert := "INSERT INTO composes(job_id, request, created_at, account_number, org_id) VALUES ($1, $2, CURRENT_TIMESTAMP - interval '2 days', $3, $4)"
	_, err = conn.Exec(context.Background(), insert, job1, "{}", ANR3, ORGID3)

	composes, count, err := d.GetComposes(ORGID3, fortnight, 100, 0)
	require.Equal(t, 1, count)
	require.NoError(t, err)
	require.Equal(t, job1, composes[0].Id)

	job2 := uuid.New()
	insert = "INSERT INTO composes(job_id, request, created_at, account_number, org_id) VALUES ($1, $2, CURRENT_TIMESTAMP - interval '20 days', $3, $4)"
	_, err = conn.Exec(context.Background(), insert, job2, "{}", ANR3, ORGID3)

	// job2 is outside of time range
	composes, count, err = d.GetComposes(ORGID3, fortnight, 100, 0)
	require.Equal(t, 1, count)
	require.NoError(t, err)
	require.Equal(t, job1, composes[0].Id)

	// correct ordering (recent first)
	composes, count, err = d.GetComposes(ORGID3, fortnight*2, 100, 0)
	require.Equal(t, 2, count)
	require.NoError(t, err)
	require.Equal(t, job1, composes[0].Id)
}

func testGetComposeImageType(t *testing.T) {
	d, err := db.InitDBConnectionPool(connStr(t))
	require.NoError(t, err)
	conn := connect(t)
	defer conn.Close(context.Background())

	composeId := uuid.New()
	insert := "INSERT INTO composes(job_id, request, created_at, account_number, org_id) VALUES ($1, $2, CURRENT_TIMESTAMP, $3, $4)"
	_, err = conn.Exec(context.Background(), insert, composeId, `
{
  "customizations": {
  },
  "distribution": "rhel-8",
  "image_requests": [
    {
      "architecture": "x86_64",
      "image_type": "guest-image",
      "upload_request": {
        "type": "aws.s3",
        "options": {
        }
      }
    }
  ]
}
`, ANR1, ORGID1)
	require.NoError(t, err)

	it, err := d.GetComposeImageType(composeId, ORGID1)
	require.NoError(t, err)
	require.Equal(t, "guest-image", it)

	_, err = d.GetComposeImageType(composeId, ORGID2)
	require.Error(t, err)
}

func testDeleteCompose(t *testing.T) {
	d, err := db.InitDBConnectionPool(connStr(t))
	require.NoError(t, err)
	conn := connect(t)
	defer conn.Close(context.Background())

	composeId := uuid.New()
	insert := "INSERT INTO composes(job_id, request, created_at, account_number, org_id) VALUES ($1, $2, CURRENT_TIMESTAMP, $3, $4)"
	_, err = conn.Exec(context.Background(), insert, composeId, "{}", ANR1, ORGID1)

	err = d.DeleteCompose(composeId, ORGID2)
	require.Equal(t, db.ComposeNotFoundError, err)

	err = d.DeleteCompose(uuid.New(), ORGID1)
	require.Equal(t, db.ComposeNotFoundError, err)

	err = d.DeleteCompose(composeId, ORGID1)
	require.NoError(t, err)

	_, count, err := d.GetComposes(ORGID1, fortnight, 100, 0)
	require.NoError(t, err)
	require.Equal(t, 0, count)

	// delete composes still counts towards quota
	count, err = d.CountComposesSince(ORGID1, fortnight)
	require.NoError(t, err)
	require.Equal(t, 1, count)
}

func testClones(t *testing.T) {
	d, err := db.InitDBConnectionPool(connStr(t))
	require.NoError(t, err)
	conn := connect(t)
	defer conn.Close(context.Background())

	composeId := uuid.New()
	cloneId := uuid.New()
	cloneId2 := uuid.New()

	// fkey constraint on compose id
	require.Error(t, d.InsertClone(composeId, cloneId, []byte(`
{
  "region": "us-east-2"
}
`)))

	require.NoError(t, d.InsertCompose(composeId, ANR1, ORGID1, nil, []byte(`
{
  "customizations": {
  },
  "distribution": "rhel-8",
  "image_requests": [
    {
      "architecture": "x86_64",
      "image_type": "guest-image",
      "upload_request": {
        "type": "aws.s3",
        "options": {
        }
      }
    }
  ]
}`)))

	require.NoError(t, d.InsertClone(composeId, cloneId, []byte(`
{
  "region": "us-east-2"
}
`)))
	require.NoError(t, d.InsertClone(composeId, cloneId2, []byte(`
{
  "region": "eu-central-1"
}
`)))

	clones, count, err := d.GetClonesForCompose(composeId, ORGID2, 100, 0)
	require.NoError(t, err)
	require.Empty(t, clones)
	require.Equal(t, 0, count)

	clones, count, err = d.GetClonesForCompose(composeId, ORGID1, 1, 0)
	require.NoError(t, err)
	require.Len(t, clones, 1)
	require.Equal(t, 2, count)
	require.Equal(t, cloneId2, clones[0].Id)

	clones, count, err = d.GetClonesForCompose(composeId, ORGID1, 100, 0)
	require.NoError(t, err)
	require.Len(t, clones, 2)
	require.Equal(t, 2, count)
	require.Equal(t, cloneId2, clones[0].Id)
	require.Equal(t, cloneId, clones[1].Id)

	entry, err := d.GetClone(cloneId, ORGID2)
	require.ErrorIs(t, err, db.CloneNotFoundError)
	require.Nil(t, entry)

	entry, err = d.GetClone(cloneId, ORGID1)
	require.NoError(t, err)
	require.Equal(t, clones[1], *entry)
}

func TestMain(t *testing.T) {
	fns := []func(*testing.T){
		testInsertCompose,
		testGetCompose,
		testCountComposesSince,
		testGetComposeImageType,
		testDeleteCompose,
		testClones,
	}

	for _, f := range fns {
		migrateTern(t)
		f(t)
		tearDown(t)
	}
}
