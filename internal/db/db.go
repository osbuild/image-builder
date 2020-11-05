package db

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v4/pgxpool"
)

type dB struct {
	Pool *pgxpool.Pool
}

type ComposeEntry struct {
	Id        uuid.UUID
	Request   json.RawMessage
	CreatedAt time.Time
}

type DB interface {
	InsertCompose(jobId, accountId, orgId string, request json.RawMessage) error
	GetComposes(accountId string, limit, offset int) ([]ComposeEntry, int, error)
}

func InitDBConnectionPool(connStr string) (DB, error) {
	dbConfig, err := pgxpool.ParseConfig(connStr)
	if err != nil {
		return nil, err
	}

	pool, err := pgxpool.ConnectConfig(context.Background(), dbConfig)
	if err != nil {
		return nil, err
	}

	return &dB{pool}, nil
}

func (db *dB) InsertCompose(jobId, accountId, orgId string, request json.RawMessage) error {
	ctx := context.Background()
	conn, err := db.Pool.Acquire(ctx)
	if err != nil {
		return err
	}
	defer conn.Release()

	_, err = conn.Query(ctx, "INSERT INTO composes(job_id, request, created_at, account_id, org_id) VALUES ($1, $2, CURRENT_TIMESTAMP, $3, $4);", jobId, request, accountId, orgId)
	return err
}

func (db *dB) GetComposes(accountId string, limit, offset int) ([]ComposeEntry, int, error) {
	ctx := context.Background()
	conn, err := db.Pool.Acquire(ctx)
	if err != nil {
		return nil, 0, err
	}
	defer conn.Release()

	result, err := conn.Query(ctx, "SELECT job_id, request, created_at FROM composes WHERE account_id=$1 ORDER BY created_at DESC LIMIT $2 OFFSET $3", accountId, limit, offset)
	if err != nil {
		return nil, 0, err
	}

	var composes []ComposeEntry
	for result.Next() {
		var jobId uuid.UUID
		var request json.RawMessage
		var createdAt time.Time
		err = result.Scan(&jobId, &request, &createdAt)
		if err != nil {
			return nil, 0, err
		}
		composes = append(composes, ComposeEntry{
			jobId,
			request,
			createdAt,
		})
	}

	var count int
	err = conn.QueryRow(ctx, "SELECT COUNT(*) FROM composes WHERE account_id=$1", accountId).Scan(&count)
	if err != nil {
		return nil, 0, err
	}

	return composes, count, nil
}
