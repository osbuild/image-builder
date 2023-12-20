//go:build integration

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"testing"
	"time"

	v1 "github.com/osbuild/image-builder/internal/v1"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
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

	EMAIL1 = "user1@test.test"

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
	output, err := cmd.CombinedOutput()
	require.NoErrorf(t, err, "tern command failed with non-zero code, combined output: %s", string(output))
}

func connect(t *testing.T) *pgx.Conn {
	conn, err := pgx.Connect(context.Background(), connStr(t))
	require.NoError(t, err)
	return conn
}

func tearDown(t *testing.T) {
	conn := connect(t)
	defer conn.Close(context.Background())
	_, err := conn.Exec(context.Background(), "drop schema public cascade")
	require.NoError(t, err)
	_, err = conn.Exec(context.Background(), "create schema public")
	require.NoError(t, err)
	_, err = conn.Exec(context.Background(), "grant all on schema public to postgres")
	require.NoError(t, err)
	_, err = conn.Exec(context.Background(), "grant all on schema public to public")
	require.NoError(t, err)
}

func testInsertCompose(t *testing.T) {
	d, err := db.InitDBConnectionPool(connStr(t))
	require.NoError(t, err)

	imageName := "MyImageName"
	clientId := "ui"
	blueprintId := uuid.New()
	versionId := uuid.New()

	migrateTern(t)

	err = d.InsertBlueprint(blueprintId, versionId, ORGID1, ANR1, "blueprint", "blueprint desc", []byte("{}"))
	require.NoError(t, err)

	// test
	err = d.InsertCompose(uuid.New(), "", "", ORGID1, &imageName, []byte("{}"), &clientId, &versionId)
	require.NoError(t, err)
	err = d.InsertCompose(uuid.New(), ANR1, EMAIL1, ORGID1, &imageName, []byte("{}"), &clientId, nil)
	require.NoError(t, err)
	err = d.InsertCompose(uuid.New(), "", "", ORGID1, &imageName, []byte("{}"), &clientId, nil)
	require.NoError(t, err)
}

func testGetCompose(t *testing.T) {
	d, err := db.InitDBConnectionPool(connStr(t))
	require.NoError(t, err)

	imageName := "MyImageName"
	clientId := "ui"

	err = d.InsertCompose(uuid.New(), ANR1, EMAIL1, ORGID1, &imageName, []byte("{}"), &clientId, nil)
	require.NoError(t, err)
	err = d.InsertCompose(uuid.New(), ANR1, EMAIL1, ORGID1, &imageName, []byte("{}"), &clientId, nil)
	require.NoError(t, err)
	err = d.InsertCompose(uuid.New(), ANR1, EMAIL1, ORGID1, &imageName, []byte("{}"), &clientId, nil)
	require.NoError(t, err)
	err = d.InsertCompose(uuid.New(), ANR1, EMAIL1, ORGID1, &imageName, []byte("{}"), &clientId, nil)
	require.NoError(t, err)

	// test
	// GetComposes works as expected
	composes, count, err := d.GetComposes(ORGID1, fortnight, 100, 0, []string{})
	require.NoError(t, err)
	require.Equal(t, 4, count)
	require.Equal(t, 4, len(composes))

	// count returns total in db, ignoring limits
	composes, count, err = d.GetComposes(ORGID1, fortnight, 1, 2, []string{})
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

	composes, count, err := d.GetComposes(ORGID3, fortnight, 100, 0, []string{})
	require.Equal(t, 1, count)
	require.NoError(t, err)
	require.Equal(t, job1, composes[0].Id)

	job2 := uuid.New()
	insert = "INSERT INTO composes(job_id, request, created_at, account_number, org_id) VALUES ($1, $2, CURRENT_TIMESTAMP - interval '20 days', $3, $4)"
	_, err = conn.Exec(context.Background(), insert, job2, "{}", ANR3, ORGID3)

	// job2 is outside of time range
	composes, count, err = d.GetComposes(ORGID3, fortnight, 100, 0, []string{})
	require.Equal(t, 1, count)
	require.NoError(t, err)
	require.Equal(t, job1, composes[0].Id)

	// correct ordering (recent first)
	composes, count, err = d.GetComposes(ORGID3, fortnight*2, 100, 0, []string{})
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

	_, count, err := d.GetComposes(ORGID1, fortnight, 100, 0, []string{})
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

	require.NoError(t, d.InsertCompose(composeId, ANR1, EMAIL1, ORGID1, nil, []byte(`
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
}`), nil, nil))

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

func testBlueprints(t *testing.T) {
	d, err := db.InitDBConnectionPool(connStr(t))
	require.NoError(t, err)
	conn := connect(t)
	defer conn.Close(context.Background())

	b1 := v1.BlueprintV1{
		Version:     1,
		Name:        "name",
		Description: "desc",
	}
	body, err := json.Marshal(b1)
	require.NoError(t, err)

	id := uuid.New()
	versionId := uuid.New()
	err = d.InsertBlueprint(id, versionId, ORGID1, ANR1, "name", "desc", body)
	require.NoError(t, err)

	entry, err := d.GetBlueprint(id, ORGID1, ANR1)
	require.NoError(t, err)
	b2, err := v1.BlueprintFromEntry(entry)
	require.NoError(t, err)
	require.Equal(t, b1, b2)

	updated := v1.BlueprintV1{
		Version:     2,
		Name:        "new name",
		Description: "new desc",
	}
	bodyUpdated1, err := json.Marshal(updated)
	require.NoError(t, err)

	newVersionId := uuid.New()
	err = d.UpdateBlueprint(newVersionId, entry.Id, ORGID1, "new name", "new desc", bodyUpdated1)
	require.NoError(t, err)
	entryUpdated, err := d.GetBlueprint(entry.Id, ORGID1, ANR1)
	require.NoError(t, err)
	bodyUpdated2, err := v1.BlueprintFromEntry(entryUpdated)
	require.NoError(t, err)
	require.Equal(t, updated, bodyUpdated2)
	require.Equal(t, updated.Version, entryUpdated.Version)
	require.Equal(t, "new name", entryUpdated.Name)
	require.Equal(t, "new desc", entryUpdated.Description)

	require.NotEqual(t, b1, bodyUpdated2)

	newBlueprintVersion := v1.BlueprintV1{
		Version:     3,
		Name:        "name should not be changed",
		Description: "desc should not be changed",
	}
	newBlueprintVersionBody, err := json.Marshal(newBlueprintVersion)
	require.NoError(t, err)
	newBlueprintId := uuid.New()
	err = d.UpdateBlueprint(newBlueprintId, entry.Id, ORGID2, "name", "desc", newBlueprintVersionBody)
	require.Error(t, err)
	entryAfterInvalidUpdate, err := d.GetBlueprint(entry.Id, ORGID1, ANR1)
	require.NoError(t, err)
	bodyNotUpdated, err := v1.BlueprintFromEntry(entryAfterInvalidUpdate)
	require.NoError(t, err)
	require.Equal(t, updated, bodyNotUpdated)
	require.Equal(t, updated.Version, bodyNotUpdated.Version)
	require.Equal(t, "new name", bodyNotUpdated.Name)
	require.Equal(t, "new desc", bodyNotUpdated.Description)
	require.NotEqual(t, b1, bodyNotUpdated)

	newestBlueprintVersionId := uuid.New()
	newestBlueprintId := uuid.New()
	newestBlueprintName := "new name"
	err = d.InsertBlueprint(newestBlueprintId, newestBlueprintVersionId, ORGID1, ANR1, newestBlueprintName, "desc", body)
	entries, _, err := d.GetBlueprints(ORGID1, 100, 0)
	require.NoError(t, err)
	require.Equal(t, entries[0].Name, newestBlueprintName)
	require.Equal(t, entries[1].Version, updated.Version)

	err = d.InsertBlueprint(uuid.New(), uuid.New(), ORGID1, ANR1, "unique name", "unique desc", body)
	entries, count, err := d.FindBlueprints(ORGID1, "unique", 100, 0)
	require.NoError(t, err)
	require.Equal(t, 1, count)
	require.Equal(t, "unique name", entries[0].Name)

	entries, count, err = d.FindBlueprints(ORGID1, "unique desc", 100, 0)
	require.NoError(t, err)
	require.Equal(t, 1, count)
	require.Equal(t, "unique desc", entries[0].Description)

	err = d.DeleteBlueprint(id, ORGID1, ANR1)
	require.NoError(t, err)
}

func runTest(t *testing.T, f func(*testing.T)) {
	migrateTern(t)
	defer tearDown(t)
	f(t)
}

func TestAll(t *testing.T) {
	fns := []func(*testing.T){
		testInsertCompose,
		testGetCompose,
		testCountComposesSince,
		testGetComposeImageType,
		testDeleteCompose,
		testClones,
		testBlueprints,
	}

	for _, f := range fns {
		runTest(t, f)
	}
}
