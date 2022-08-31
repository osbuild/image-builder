package db

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v4"
	"github.com/jackc/pgx/v4/pgxpool"
)

// ComposeNotFoundError occurs when no compose request is found for a user.
var ComposeNotFoundError = errors.New("Compose not found")

type dB struct {
	Pool *pgxpool.Pool
}

type ComposeEntry struct {
	Id        uuid.UUID
	Request   json.RawMessage
	CreatedAt time.Time
	ImageName *string
}

type DB interface {
	InsertCompose(jobId, accountNumber, orgId string, imageName *string, request json.RawMessage) error
	GetComposes(orgId string, since time.Duration, limit, offset int) ([]ComposeEntry, int, error)
	GetCompose(jobId string, orgId string) (*ComposeEntry, error)
	GetComposeImageType(jobId, orgId string) (string, error)
	CountComposesSince(orgId string, duration time.Duration) (int, error)
}

const (
	sqlInsertCompose = `
		INSERT INTO composes(job_id, request, created_at, account_number, org_id, image_name)
		VALUES ($1, $2, CURRENT_TIMESTAMP, $3, $4, $5)`

	sqlGetComposes = `
		SELECT job_id, request, created_at, image_name
		FROM composes WHERE org_id=$1 AND CURRENT_TIMESTAMP - created_at <= $2
		ORDER BY created_at DESC
		LIMIT $3 OFFSET $4`

	sqlGetCompose = `
		SELECT job_id, request, created_at, image_name
		FROM composes
		WHERE org_id=$1 AND job_id=$2`

	sqlGetComposeImageType = `
		SELECT req->>'image_type'
		FROM composes,jsonb_array_elements(composes.request->'image_requests') as req
		WHERE org_id=$1 AND job_id = $2`

	sqlCountComposesSince = `
		SELECT COUNT(*)
		FROM composes
		WHERE org_id=$1 AND CURRENT_TIMESTAMP - created_at <= $2`
)

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

func (db *dB) InsertCompose(jobId, accountNumber, orgId string, imageName *string, request json.RawMessage) error {
	ctx := context.Background()
	conn, err := db.Pool.Acquire(ctx)
	if err != nil {
		return err
	}
	defer conn.Release()

	_, err = conn.Exec(ctx, sqlInsertCompose, jobId, request, accountNumber, orgId, imageName)
	return err
}

func (db *dB) GetCompose(jobId string, orgId string) (*ComposeEntry, error) {
	ctx := context.Background()
	conn, err := db.Pool.Acquire(ctx)
	if err != nil {
		return nil, err
	}
	defer conn.Release()

	result := conn.QueryRow(ctx, sqlGetCompose, orgId, jobId)

	var compose ComposeEntry
	err = result.Scan(&compose.Id, &compose.Request, &compose.CreatedAt, &compose.ImageName)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ComposeNotFoundError
		} else {
			return nil, err
		}
	}

	return &compose, nil
}

func (db *dB) GetComposeImageType(jobId, orgId string) (string, error) {
	ctx := context.Background()
	conn, err := db.Pool.Acquire(ctx)
	if err != nil {
		return "", err
	}
	defer conn.Release()

	result := conn.QueryRow(ctx, sqlGetComposeImageType, orgId, jobId)

	var imageType string
	err = result.Scan(&imageType)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", ComposeNotFoundError
		} else {
			return "", err
		}
	}

	return imageType, nil
}

func (db *dB) GetComposes(orgId string, since time.Duration, limit, offset int) ([]ComposeEntry, int, error) {
	ctx := context.Background()
	conn, err := db.Pool.Acquire(ctx)
	if err != nil {
		return nil, 0, err
	}
	defer conn.Release()

	result, err := conn.Query(ctx, sqlGetComposes, orgId, since, limit, offset)
	if err != nil {
		return nil, 0, err
	}

	var composes []ComposeEntry
	for result.Next() {
		var jobId uuid.UUID
		var request json.RawMessage
		var createdAt time.Time
		var imageName *string
		err = result.Scan(&jobId, &request, &createdAt, &imageName)
		if err != nil {
			return nil, 0, err
		}
		composes = append(composes, ComposeEntry{
			jobId,
			request,
			createdAt,
			imageName,
		})
	}
	if err = result.Err(); err != nil {
		return nil, 0, err
	}

	var count int
	err = conn.QueryRow(ctx, sqlCountComposesSince, orgId, since).Scan(&count)
	if err != nil {
		return nil, 0, err
	}

	return composes, count, nil
}

func (db *dB) CountComposesSince(orgId string, duration time.Duration) (int, error) {
	ctx := context.Background()
	conn, err := db.Pool.Acquire(ctx)
	if err != nil {
		return 0, err
	}
	defer conn.Release()

	var count int
	err = conn.QueryRow(ctx,
		sqlCountComposesSince,
		orgId, duration).Scan(&count)
	if err != nil {
		return 0, err
	}

	return count, nil
}
