package db

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ComposeNotFoundError occurs when no compose request is found for a user.
var ComposeNotFoundError = errors.New("Compose not found")
var CloneNotFoundError = errors.New("Clone not found")
var BlueprintNotFoundError = errors.New("blueprint not found")
var AffectedRowsMismatchError = errors.New("Unexpected affected rows")

type dB struct {
	Pool *pgxpool.Pool
}

type ComposeEntry struct {
	Id        uuid.UUID
	Request   json.RawMessage
	CreatedAt time.Time
	ImageName *string
	ClientId  *string
}

type CloneEntry struct {
	Id        uuid.UUID
	ComposeId uuid.UUID
	Request   json.RawMessage
	CreatedAt time.Time
}

type BlueprintEntry struct {
	Id          uuid.UUID
	VersionId   uuid.UUID
	Version     int
	Body        json.RawMessage
	Name        string
	Description string
}

type BlueprintWithNoBody struct {
	Id             uuid.UUID
	Version        int
	Name           string
	Description    string
	LastModifiedAt time.Time
}

type DB interface {
	InsertCompose(jobId uuid.UUID, accountNumber, email, orgId string, imageName *string, request json.RawMessage, clientId *string, blueprintVersionId *uuid.UUID) error
	GetComposes(orgId string, since time.Duration, limit, offset int, ignoreImageTypes []string) ([]ComposeEntry, int, error)
	GetBlueprintComposes(orgId string, blueprintId uuid.UUID, blueprintVersion *int, since time.Duration, limit, offset int, ignoreImageTypes []string) ([]BlueprintCompose, error)
	GetCompose(jobId uuid.UUID, orgId string) (*ComposeEntry, error)
	GetComposeImageType(jobId uuid.UUID, orgId string) (string, error)
	CountComposesSince(orgId string, duration time.Duration) (int, error)
	CountBlueprintComposesSince(orgId string, blueprintId uuid.UUID, blueprintVersion *int, since time.Duration, ignoreImageTypes []string) (int, error)
	DeleteCompose(jobId uuid.UUID, orgId string) error

	InsertClone(composeId, cloneId uuid.UUID, request json.RawMessage) error
	GetClonesForCompose(composeId uuid.UUID, orgId string, limit, offset int) ([]CloneEntry, int, error)
	GetClone(id uuid.UUID, orgId string) (*CloneEntry, error)

	InsertBlueprint(id uuid.UUID, versionId uuid.UUID, orgID, accountNumber, name, description string, body json.RawMessage) error
	GetBlueprint(id uuid.UUID, orgID, accountNumber string) (*BlueprintEntry, error)
	UpdateBlueprint(id uuid.UUID, blueprintId uuid.UUID, orgId string, name string, description string, body json.RawMessage) error
	GetBlueprints(orgID string, limit, offset int) ([]BlueprintWithNoBody, int, error)
	FindBlueprints(orgID, search string, limit, offset int) ([]BlueprintWithNoBody, int, error)
	DeleteBlueprint(id uuid.UUID, orgID, accountNumber string) error
}

const (
	sqlInsertCompose = `
		INSERT INTO composes(job_id, request, created_at, account_number, email, org_id, image_name, client_id, blueprint_version_id)
		VALUES ($1, $2, CURRENT_TIMESTAMP, $3, $4, $5, $6, $7, $8)`

	sqlGetComposes = `
	        SELECT job_id, request, created_at, image_name, client_id
	        FROM composes
		WHERE org_id = $1
		AND CURRENT_TIMESTAMP - created_at <= $2
		AND ($3::text[] is NULL OR request->'image_requests'->0->>'image_type' <> ALL($3))
		AND deleted = FALSE
		ORDER BY created_at DESC
		LIMIT $4 OFFSET $5`

	sqlGetCompose = `
		SELECT job_id, request, created_at, image_name, client_id
		FROM composes
		WHERE org_id=$1 AND job_id=$2 AND deleted=FALSE`

	sqlGetComposeImageType = `
		SELECT req->>'image_type'
		FROM composes,jsonb_array_elements(composes.request->'image_requests') as req
		WHERE org_id=$1 AND job_id = $2`

	sqlCountActiveComposesSince = `
		SELECT COUNT(*)
		FROM composes
		WHERE org_id=$1 AND CURRENT_TIMESTAMP - created_at <= $2 AND deleted = FALSE
		AND ($3::text[] is NULL OR request->'image_requests'->0->>'image_type' <> ALL($3))`

	sqlCountComposesSince = `
		SELECT COUNT(*)
		FROM composes
		WHERE org_id=$1 AND CURRENT_TIMESTAMP - created_at <= $2`

	sqlDeleteCompose = `
		UPDATE composes
		SET deleted = TRUE
		WHERE org_id=$1 AND job_id=$2
        `

	sqlInsertClone = `
		INSERT INTO clones(id, compose_id, request, created_at)
		VALUES($1, $2, $3, CURRENT_TIMESTAMP)`

	sqlGetClonesForCompose = `
		SELECT clones.id, clones.compose_id, clones.request, clones.created_at
		FROM clones
		WHERE clones.compose_id=$1 AND $1 in (
			SELECT composes.job_id
			FROM composes
			WHERE composes.org_id=$2)
		ORDER BY created_at DESC
		LIMIT $3 OFFSET $4`

	sqlCountClonesForCompose = `
		SELECT COUNT(*)
		FROM clones
		WHERE clones.compose_id=$1 AND $1 in (
			SELECT composes.job_id
			FROM composes
			WHERE composes.org_id=$2)`

	sqlGetClone = `
		SELECT clones.id, clones.compose_id, clones.request, clones.created_at
		FROM clones
		WHERE clones.id=$1 AND clones.compose_id in (
			SELECT composes.job_id
			FROM composes
			WHERE composes.org_id=$2)`
)

func InitDBConnectionPool(connStr string) (DB, error) {
	dbConfig, err := pgxpool.ParseConfig(connStr)
	if err != nil {
		return nil, err
	}

	pool, err := pgxpool.NewWithConfig(context.Background(), dbConfig)
	if err != nil {
		return nil, err
	}

	return &dB{pool}, nil
}

func (db *dB) InsertCompose(jobId uuid.UUID, accountNumber, email, orgId string, imageName *string, request json.RawMessage, clientId *string, blueprintVersionId *uuid.UUID) error {
	ctx := context.Background()
	conn, err := db.Pool.Acquire(ctx)
	if err != nil {
		return err
	}
	defer conn.Release()

	_, err = conn.Exec(ctx, sqlInsertCompose, jobId, request, accountNumber, email, orgId, imageName, clientId, blueprintVersionId)
	return err
}

func (db *dB) GetCompose(jobId uuid.UUID, orgId string) (*ComposeEntry, error) {
	ctx := context.Background()
	conn, err := db.Pool.Acquire(ctx)
	if err != nil {
		return nil, err
	}
	defer conn.Release()

	result := conn.QueryRow(ctx, sqlGetCompose, orgId, jobId)

	var compose ComposeEntry
	err = result.Scan(&compose.Id, &compose.Request, &compose.CreatedAt, &compose.ImageName, &compose.ClientId)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ComposeNotFoundError
		} else {
			return nil, err
		}
	}

	return &compose, nil
}

func (db *dB) GetComposeImageType(jobId uuid.UUID, orgId string) (string, error) {
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

func (db *dB) GetComposes(orgId string, since time.Duration, limit, offset int, ignoreImageTypes []string) ([]ComposeEntry, int, error) {
	ctx := context.Background()
	conn, err := db.Pool.Acquire(ctx)
	if err != nil {
		return nil, 0, err
	}
	defer conn.Release()
	result, err := conn.Query(ctx, sqlGetComposes, orgId, since, ignoreImageTypes, limit, offset)
	if err != nil {
		return nil, 0, err
	}

	defer result.Close()

	var composes []ComposeEntry
	for result.Next() {
		var jobId uuid.UUID
		var request json.RawMessage
		var createdAt time.Time
		var imageName *string
		var clientId *string
		err = result.Scan(&jobId, &request, &createdAt, &imageName, &clientId)
		if err != nil {
			return nil, 0, err
		}
		composes = append(composes, ComposeEntry{
			jobId,
			request,
			createdAt,
			imageName,
			clientId,
		})
	}
	if err = result.Err(); err != nil {
		return nil, 0, err
	}

	var count int
	err = conn.QueryRow(ctx, sqlCountActiveComposesSince, orgId, since, ignoreImageTypes).Scan(&count)
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

func (db *dB) DeleteCompose(jobId uuid.UUID, orgId string) error {
	ctx := context.Background()
	conn, err := db.Pool.Acquire(ctx)
	if err != nil {
		return err
	}
	defer conn.Release()

	tag, err := conn.Exec(ctx, sqlDeleteCompose, orgId, jobId)
	if tag.RowsAffected() != 1 {
		return ComposeNotFoundError
	}

	return err
}

func (db *dB) InsertClone(composeId, cloneId uuid.UUID, request json.RawMessage) error {
	ctx := context.Background()
	conn, err := db.Pool.Acquire(ctx)
	if err != nil {
		return err
	}
	defer conn.Release()

	_, err = conn.Exec(ctx, sqlInsertClone, cloneId, composeId, request)
	return err
}

func (db *dB) GetClonesForCompose(composeId uuid.UUID, orgId string, limit, offset int) ([]CloneEntry, int, error) {
	ctx := context.Background()
	conn, err := db.Pool.Acquire(ctx)
	if err != nil {
		return nil, 0, err
	}
	defer conn.Release()

	rows, err := conn.Query(ctx, sqlGetClonesForCompose, composeId, orgId, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var clones []CloneEntry
	for rows.Next() {
		var id uuid.UUID
		var composeID uuid.UUID
		var request json.RawMessage
		var createdAt time.Time
		err = rows.Scan(&id, &composeID, &request, &createdAt)
		if err != nil {
			return nil, 0, err
		}
		clones = append(clones, CloneEntry{
			id,
			composeID,
			request,
			createdAt,
		})
	}
	if err = rows.Err(); err != nil {
		return nil, 0, err
	}

	var count int
	err = conn.QueryRow(ctx, sqlCountClonesForCompose, composeId, orgId).Scan(&count)
	if err != nil {
		return nil, 0, err
	}

	return clones, count, nil

}

func (db *dB) GetClone(id uuid.UUID, orgId string) (*CloneEntry, error) {
	ctx := context.Background()
	conn, err := db.Pool.Acquire(ctx)
	if err != nil {
		return nil, err
	}
	defer conn.Release()

	var clone CloneEntry
	err = conn.QueryRow(ctx, sqlGetClone, id, orgId).Scan(&clone.Id, &clone.ComposeId, &clone.Request, &clone.CreatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, CloneNotFoundError
		} else {
			return nil, err
		}
	}

	return &clone, nil
}
