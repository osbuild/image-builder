package main

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/sirupsen/logrus"
)

const (
	sqlDeleteComposes = `
                DELETE FROM composes
                WHERE created_at < $1`
	sqlExpiredClonesCount = `
                SELECT COUNT(*) FROM clones
                WHERE compose_id in (
                    SELECT job_id
                    FROM composes
                    WHERE created_at < $1
                )`
	sqlExpiredComposesCount = `
                SELECT COUNT(*) FROM composes
                WHERE created_at < $1`
	sqlVacuumAnalyze = `
                VACUUM ANALYZE`
	sqlVacuumStats = `
                SELECT relname, pg_size_pretty(pg_total_relation_size(relid)),
                    n_tup_ins, n_tup_upd, n_tup_del, n_live_tup, n_dead_tup,
                    vacuum_count, autovacuum_count, analyze_count, autoanalyze_count,
                    last_vacuum, last_autovacuum, last_analyze, last_autoanalyze
                 FROM pg_stat_user_tables`
)

type maintenanceDB struct {
	Conn *pgx.Conn
}

func newDB(ctx context.Context, dbURL string) (maintenanceDB, error) {
	conn, err := pgx.Connect(ctx, dbURL)
	if err != nil {
		return maintenanceDB{}, err
	}

	return maintenanceDB{
		conn,
	}, nil
}

func (d *maintenanceDB) Close() error {
	return d.Conn.Close(context.Background())
}

func (d *maintenanceDB) DeleteComposes(ctx context.Context, emailRetentionDate time.Time) (int64, error) {
	tag, err := d.Conn.Exec(ctx, sqlDeleteComposes, emailRetentionDate)
	if err != nil {
		return tag.RowsAffected(), fmt.Errorf("Error deleting composes: %v", err)
	}
	return tag.RowsAffected(), nil
}

func (d *maintenanceDB) ExpiredClonesCount(ctx context.Context, emailRetentionDate time.Time) (int64, error) {
	var count int64
	err := d.Conn.QueryRow(ctx, sqlExpiredClonesCount, emailRetentionDate).Scan(&count)
	if err != nil {
		return 0, err
	}
	return count, nil
}

func (d *maintenanceDB) ExpiredComposesCount(ctx context.Context, emailRetentionDate time.Time) (int64, error) {
	var count int64
	err := d.Conn.QueryRow(ctx, sqlExpiredComposesCount, emailRetentionDate).Scan(&count)
	if err != nil {
		return 0, err
	}
	return count, nil
}

func (d *maintenanceDB) VacuumAnalyze(ctx context.Context) error {
	_, err := d.Conn.Exec(ctx, sqlVacuumAnalyze)
	if err != nil {
		return fmt.Errorf("Error running VACUUM ANALYZE: %v", err)
	}
	return nil
}

func (d *maintenanceDB) LogVacuumStats(ctx context.Context) (int64, error) {
	rows, err := d.Conn.Query(ctx, sqlVacuumStats)
	if err != nil {
		return int64(0), fmt.Errorf("Error querying vacuum stats: %v", err)
	}
	defer rows.Close()

	deleted := int64(0)

	for rows.Next() {
		select {
		case <-ctx.Done():
			err = ctx.Err()
			logrus.Errorf("Context cancelled LogVacuumStats: %v", err)
			return int64(0), err
		default:
			var relName, relSize string
			var ins, upd, del, live, dead, vc, avc, ac, aac int64
			var lvc, lavc, lan, laan *time.Time

			err = rows.Scan(&relName, &relSize, &ins, &upd, &del, &live, &dead,
				&vc, &avc, &ac, &aac,
				&lvc, &lavc, &lan, &laan)
			if err != nil {
				return int64(0), err
			}

			logrus.WithFields(logrus.Fields{
				"table_name":        relName,
				"table_size":        relSize,
				"tuples_inserted":   ins,
				"tuples_updated":    upd,
				"tuples_deleted":    del,
				"tuples_live":       live,
				"tuples_dead":       dead,
				"vacuum_count":      vc,
				"autovacuum_count":  avc,
				"last_vacuum":       lvc,
				"last_autovacuum":   lavc,
				"analyze_count":     ac,
				"autoanalyze_count": aac,
				"last_analyze":      lan,
				"last_autoanalyze":  laan,
			}).Info("Vacuum and analyze stats for table")
		}
	}
	if rows.Err() != nil {
		return int64(0), rows.Err()
	}
	return deleted, nil

}

func DBCleanup(ctx context.Context, dbURL string, dryRun bool, ComposesRetentionMonths int) error {
	db, err := newDB(ctx, dbURL)
	if err != nil {
		return err
	}

	_, err = db.LogVacuumStats(ctx)
	if err != nil {
		logrus.Errorf("Error running vacuum stats: %v", err)
	}

	var rowsClones int64
	var rows int64

	emailRetentionDate := time.Now().AddDate(0, ComposesRetentionMonths*-1, 0)

	for {
		select {
		case <-ctx.Done():
			err = ctx.Err()
			logrus.Errorf("Context cancelled DBCleanup: %v", err)
			return err
		default:
			// continue execution outside of select
			// so `break` works as expected
		}
		if dryRun {
			rowsClones, err = db.ExpiredClonesCount(ctx, emailRetentionDate)
			if err != nil {
				logrus.Warningf("Error querying expired clones: %v", err)
			}

			rows, err = db.ExpiredComposesCount(ctx, emailRetentionDate)
			if err != nil {
				logrus.Warningf("Error querying expired composes: %v", err)
			}
			logrus.Infof("Dryrun, expired composes count: %d (affecting %d clones)", rows, rowsClones)
			break
		}

		rows, err = db.DeleteComposes(ctx, emailRetentionDate)
		if err != nil {
			logrus.Errorf("Error deleting composes: %v, %d rows affected", rows, err)
			return err
		}

		err = db.VacuumAnalyze(ctx)
		if err != nil {
			logrus.Errorf("Error running vacuum analyze: %v", err)
			return err
		}

		if rows == 0 {
			break
		}

		logrus.Infof("Deleted results for %d", rows)
	}

	_, err = db.LogVacuumStats(ctx)
	if err != nil {
		logrus.Errorf("Error running vacuum stats: %v", err)
	}

	return nil
}
