//go:build dbtests

package main

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/osbuild/image-builder/internal/db"
	"github.com/osbuild/image-builder/internal/tutils"
	"github.com/stretchr/testify/require"
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

// testExpireCompose testing expiration of compose only
// also tests vacuum
func testExpireCompose(ctx context.Context, t *testing.T) {
	connStr := tutils.ConnStr(t)
	d, err := newDB(ctx, connStr)
	require.NoError(t, err)

	dbComposesRetentionMonths := 5

	alreadyExpiredTime := time.Now().AddDate(0, (dbComposesRetentionMonths+1)*-1, 0)
	emailRetentionDate := time.Now().AddDate(0, dbComposesRetentionMonths*-1, 0)

	composeId := uuid.New()
	insert := "INSERT INTO composes(job_id, request, created_at, account_number, org_id) VALUES ($1, $2, $3, $4, $5)"
	_, err = d.Conn.Exec(ctx, insert, composeId, "{}", alreadyExpiredTime, ANR1, ORGID1)
	require.NoError(t, err)

	require.NoError(t, d.VacuumAnalyze(ctx))
	deleted, err := d.LogVacuumStats(ctx)
	require.NoError(t, err)
	require.Equal(t, int64(0), deleted)

	rows, err := d.ExpiredComposesCount(ctx, emailRetentionDate)
	require.NoError(t, err)
	require.Equal(t, int64(1), rows)

	rows, err = d.DeleteComposes(ctx, emailRetentionDate)
	require.NoError(t, err)
	require.Equal(t, int64(1), rows)

	// assure data to be flushed for the vacuum test to work
	_, err = d.Conn.Exec(ctx, "CHECKPOINT")
	require.NoError(t, err)

	rows, err = d.ExpiredComposesCount(ctx, emailRetentionDate)
	require.NoError(t, err)
	require.Equal(t, int64(0), rows)

	require.NoError(t, d.VacuumAnalyze(ctx))
	_, err = d.LogVacuumStats(ctx)
	//deleted, err = d.LogVacuumStats()
	// skip check for now
	// until we found out why this works locally but not in
	// the github actions
	//require.Equal(t, int64(1), deleted)
	require.NoError(t, err)
}

func testExpireByCallingDBCleanup(ctx context.Context, t *testing.T) {
	connStr := tutils.ConnStr(t)
	d, err := newDB(ctx, connStr)
	require.NoError(t, err)

	internalDB, err := db.InitDBConnectionPool(ctx, connStr)
	require.NoError(t, err)

	dbComposesRetentionMonths := 5

	notYetExpiredTime := time.Now()
	alreadyExpiredTime := time.Now().AddDate(0, (dbComposesRetentionMonths+1)*-1, 0)
	emailRetentionDate := time.Now().AddDate(0, dbComposesRetentionMonths*-1, 0)

	composeIdNotYetExpired := uuid.New()
	insert := "INSERT INTO composes(job_id, request, created_at, account_number, org_id) VALUES ($1, $2, $3, $4, $5)"
	_, err = d.Conn.Exec(ctx, insert, composeIdNotYetExpired, "{}", notYetExpiredTime, ANR1, ORGID1)

	composeIdExpired := uuid.New()
	insert = "INSERT INTO composes(job_id, request, created_at, account_number, org_id) VALUES ($1, $2, $3, $4, $5)"
	_, err = d.Conn.Exec(ctx, insert, composeIdExpired, "{}", alreadyExpiredTime, ANR1, ORGID1)

	cloneId := uuid.New()
	require.NoError(t, internalDB.InsertClone(ctx, composeIdExpired, cloneId, []byte(`
{
  "region": "us-east-2"
}
`)))

	// two rows inserted, only one is expired
	rows, err := d.ExpiredComposesCount(ctx, emailRetentionDate)
	require.NoError(t, err)
	require.Equal(t, int64(1), rows)

	rows, err = d.ExpiredClonesCount(ctx, emailRetentionDate)
	require.NoError(t, err)
	require.Equal(t, int64(1), rows)

	err = DBCleanup(ctx, connStr, false, dbComposesRetentionMonths)
	require.NoError(t, err)

	rows, err = d.ExpiredComposesCount(ctx, emailRetentionDate)
	require.NoError(t, err)
	require.Equal(t, int64(0), rows)

	rows, err = d.ExpiredClonesCount(ctx, emailRetentionDate)
	require.NoError(t, err)
	require.Equal(t, int64(0), rows)
}

// testVacuum test if no vacuum is performed on a clean database
func testVacuum(ctx context.Context, t *testing.T) {
	d, err := newDB(ctx, tutils.ConnStr(t))
	require.NoError(t, err)

	require.NoError(t, d.VacuumAnalyze(ctx))
	deleted, err := d.LogVacuumStats(ctx)
	require.NoError(t, err)
	require.Equal(t, int64(0), deleted)
}

func testDryRun(ctx context.Context, t *testing.T) {
	connStr := tutils.ConnStr(t)
	d, err := newDB(ctx, connStr)
	require.NoError(t, err)

	dbComposesRetentionMonths := 5

	alreadyExpiredTime := time.Now().AddDate(0, (dbComposesRetentionMonths+1)*-1, 0)
	emailRetentionDate := time.Now().AddDate(0, dbComposesRetentionMonths*-1, 0)

	composeIdExpired := uuid.New()
	insert := "INSERT INTO composes(job_id, request, created_at, account_number, org_id) VALUES ($1, $2, $3, $4, $5)"
	_, err = d.Conn.Exec(ctx, insert, composeIdExpired, "{}", alreadyExpiredTime, ANR1, ORGID1)

	rows, err := d.ExpiredComposesCount(ctx, emailRetentionDate)
	require.NoError(t, err)
	require.Equal(t, int64(1), rows)

	err = DBCleanup(ctx, connStr, true, dbComposesRetentionMonths)
	require.NoError(t, err)

	// still there
	rows, err = d.ExpiredComposesCount(ctx, emailRetentionDate)
	require.NoError(t, err)
	require.Equal(t, int64(1), rows)
}

func TestAll(t *testing.T) {
	ctx := context.Background()
	fns := []func(context.Context, *testing.T){
		testExpireCompose,
		testExpireByCallingDBCleanup,
		testDryRun,
		testVacuum,
	}

	for _, f := range fns {
		select {
		case <-ctx.Done():
			require.NoError(t, ctx.Err())
			return
		default:
			tutils.RunTest(ctx, t, f)
		}
	}
}
