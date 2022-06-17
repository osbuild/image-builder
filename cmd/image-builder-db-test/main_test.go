// +build integration

package main

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v4"
	"github.com/stretchr/testify/require"

	"github.com/osbuild/image-builder/internal/config"
	"github.com/osbuild/image-builder/internal/db"
	"github.com/osbuild/image-builder/internal/logger"
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
		ListenAddress: "unused",
		LogLevel:      "INFO",
		MigrationsDir: "/usr/share/image-builder/migrations",
		PGHost:        "localhost",
		PGPort:        "5432",
		PGDatabase:    "imagebuilder",
		PGUser:        "postgres",
		PGPassword:    "foobar",
		PGSSLMode:     "disable",
	}

	err := config.LoadConfigFromEnv(&c)
	require.NoError(t, err)

	return &c
}

func connStr(t *testing.T) string {
	c := conf(t)
	return fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=%s", c.PGUser, c.PGPassword, c.PGHost, c.PGPort, c.PGDatabase, c.PGSSLMode)
}

func migrateOneStep(t *testing.T) {
	c := conf(t)

	log, err := logger.NewLogger(c.LogLevel, "", "", "", "", "")
	require.NoError(t, err)
	require.NotNil(t, log)

	err = db.MigrateSteps(connStr(t), c.MigrationsDir, 1, log)
	require.NoError(t, err)
}

func migrateUp(t *testing.T) {
	c := conf(t)

	log, err := logger.NewLogger(c.LogLevel, "", "", "", "", "")
	require.NoError(t, err)
	require.NotNil(t, log)

	err = db.Migrate(connStr(t), c.MigrationsDir, log)
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
	conn.Exec(context.Background(), "drop table composes")
	conn.Exec(context.Background(), "drop table schema_migrations")
}

func testMigration(t *testing.T) {
	defer tearDown(t) // tear-down cleanup the database

	migrateOneStep(t) //migrate to step 1

	conn := connect(t)
	defer conn.Close(context.Background())
	insert := "INSERT INTO composes(job_id, request, created_at, account_id, org_id) VALUES ($1, $2, CURRENT_TIMESTAMP, $3, $4)"
	_, err := conn.Exec(context.Background(), insert, uuid.New().String(), "{}", ANR1, ORGID1)
	require.NoError(t, err)

	migrateOneStep(t) // migrate to step 2

	insert = "INSERT INTO composes(job_id, request, created_at, account_number, org_id) VALUES ($1, $2, CURRENT_TIMESTAMP, $3, $4)"
	_, err = conn.Exec(context.Background(), insert, uuid.New().String(), "{}", ANR2, ORGID2)
	require.NoError(t, err)

	// inserting data referring to account_id should fail after migration step 2
	insert = "INSERT INTO composes(job_id, request, created_at, account_id, org_id) VALUES ($1, $2, CURRENT_TIMESTAMP, $3, $4)"
	_, err = conn.Exec(context.Background(), insert, uuid.New().String(), "{}", ANR1, ORGID1)
	require.Error(t, err)

	migrateOneStep(t) // migrate to step 3

	// Verify that after migration step 3 adding a compose request to the db requires a non empty account number.
	insert = "INSERT INTO composes(job_id, request, created_at, account_id, org_id) VALUES ($1, $2, CURRENT_TIMESTAMP, $3, $4)"
	_, err = conn.Exec(context.Background(), insert, uuid.New().String(), "{}", "", ORGID1)
	require.Error(t, err)

	migrateOneStep(t) // migrate to step 4

	imageName := "MyImageName"

	// inserting image_name should succeed
	insert = "INSERT INTO composes(job_id, request, created_at, account_number, org_id, image_name) VALUES ($1, $2, CURRENT_TIMESTAMP, $3, $4, $5)"
	_, err = conn.Exec(context.Background(), insert, uuid.New().String(), "{}", ANR3, ORGID3, &imageName)
	require.NoError(t, err)

	// inserting empty image_name should succeed
	insert = "INSERT INTO composes(job_id, request, created_at, account_number, org_id, image_name) VALUES ($1, $2, CURRENT_TIMESTAMP, $3, $4, $5)"
	_, err = conn.Exec(context.Background(), insert, uuid.New().String(), "{}", ANR3, ORGID3, nil)
	require.NoError(t, err)

	migrateOneStep(t) // migrate to step 5

	// inserting image_name with length > 101 should fail
	imageNameInvalid := strings.Repeat("a", 101)

	insert = "INSERT INTO composes(job_id, request, created_at, account_number, org_id, image_name) VALUES ($1, $2, CURRENT_TIMESTAMP, $3, $4, $5)"
	_, err = conn.Exec(context.Background(), insert, uuid.New().String(), "{}", ANR3, ORGID1, &imageNameInvalid)

	migrateOneStep(t) // migrate to step 6

	// Verify that after migration step 6 adding a compose request to the db requires a non empty org_id.
	insert = "INSERT INTO composes(job_id, request, created_at, account_id, org_id) VALUES ($1, $2, CURRENT_TIMESTAMP, $3, $4)"
	_, err = conn.Exec(context.Background(), insert, uuid.New().String(), "{}", ANR1, "")
	require.Error(t, err)

	// make sure migrating a fully migrated db doesn't error out
	migrateUp(t)

	// Check data inserted at migration step 1 and 2 are still accessible
	d, err := db.InitDBConnectionPool(connStr(t))
	_, count, err := d.GetComposes(ORGID1, fortnight, 100, 0)
	require.NoError(t, err)
	require.Equal(t, 1, count)
	_, count, err = d.GetComposes(ORGID2, fortnight, 100, 0)
	require.NoError(t, err)
	require.Equal(t, 1, count)

	// check that the image names are as expected
	composeEntries, count, err := d.GetComposes(ORGID3, fortnight, 100, 0)
	require.Nil(t, composeEntries[0].ImageName)
	require.Equal(t, imageName, *composeEntries[1].ImageName)
}

func testInsertCompose(t *testing.T) {
	d, err := db.InitDBConnectionPool(connStr(t))
	require.NoError(t, err)

	// teardwon
	defer tearDown(t)

	imageName := "MyImageName"

	// setup
	migrateUp(t)

	// test
	err = d.InsertCompose(uuid.New().String(), ANR1, ORGID1, &imageName, []byte("{}"))
	require.NoError(t, err)
	err = d.InsertCompose("toto", ANR1, ORGID1, &imageName, []byte("{}"))
	require.Error(t, err)
	err = d.InsertCompose(uuid.New().String(), "", ORGID1, &imageName, []byte("{}"))
	require.Error(t, err)
}

func testGetCompose(t *testing.T) {
	d, err := db.InitDBConnectionPool(connStr(t))
	require.NoError(t, err)

	imageName := "MyImageName"

	// teardwon
	defer tearDown(t)

	// setup
	migrateUp(t)
	err = d.InsertCompose(uuid.New().String(), ANR1, ORGID1, &imageName, []byte("{}"))
	require.NoError(t, err)
	err = d.InsertCompose(uuid.New().String(), ANR1, ORGID1, &imageName, []byte("{}"))
	require.NoError(t, err)
	err = d.InsertCompose(uuid.New().String(), ANR1, ORGID1, &imageName, []byte("{}"))
	require.NoError(t, err)
	err = d.InsertCompose(uuid.New().String(), ANR1, ORGID1, &imageName, []byte("{}"))
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
	compose, err := d.GetCompose(composes[0].Id.String(), ORGID1)
	require.NoError(t, err)
	require.Equal(t, composes[0], *compose)

	// cross-account compose access not allowed
	compose, err = d.GetCompose(composes[0].Id.String(), ORGID2)
	require.Equal(t, db.ComposeNotFoundError, err)
	require.Nil(t, compose)

}

func testCountComposesSince(t *testing.T) {
	d, err := db.InitDBConnectionPool(connStr(t))
	require.NoError(t, err)

	imageName := "MyImageName"

	// teardwon
	defer tearDown(t)

	// setup
	migrateUp(t)
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

	// teardwon
	defer tearDown(t)

	// setup
	migrateUp(t)
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

func TestMain(t *testing.T) {
	tearDown(t)
	testMigration(t)
	testInsertCompose(t)
	testGetCompose(t)
	testCountComposesSince(t)
}
