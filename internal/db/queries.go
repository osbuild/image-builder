package db

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
	_ "github.com/jackc/pgx/v5/stdlib"
)

const (
	sqlInsertCompose = `
		INSERT INTO composes(job_id, request, created_at, account_number, org_id, image_name)
		VALUES ($1, $2, CURRENT_TIMESTAMP, $3, $4, $5)`

	sqlGetComposes = `
		SELECT job_id, request, created_at, image_name
		FROM composes
		WHERE org_id=$1 AND CURRENT_TIMESTAMP - created_at <= $2 AND deleted=FALSE
		ORDER BY created_at DESC
		LIMIT $3 OFFSET $4`

	sqlGetCompose = `
		SELECT job_id, request, created_at, image_name
		FROM composes
		WHERE org_id=$1 AND job_id=$2 AND deleted=FALSE`

	sqlGetComposeImageTypePSQL = `
		SELECT req->>'image_type'
		FROM composes,jsonb_array_elements(composes.request->'image_requests') as req
		WHERE org_id=$1 AND job_id = $2`

	sqlCountActiveComposesSince = `
		SELECT COUNT(*)
		FROM composes
		WHERE org_id=$1 AND CURRENT_TIMESTAMP - created_at <= $2 AND deleted = FALSE`

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
		SELECT clones.id, clones.request, clones.created_at
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
		SELECT clones.id, clones.request, clones.created_at
		FROM clones
		WHERE clones.id=$1 AND clones.compose_id in (
			SELECT composes.job_id
			FROM composes
			WHERE composes.org_id=$2)`
)

func (d *dB) InsertCompose(jobId uuid.UUID, accountNumber, orgId string, imageName *string, request json.RawMessage) error {
	ctx := context.Background()
	_, err := d.pool.ExecContext(ctx, sqlInsertCompose, jobId, request, accountNumber, orgId, imageName)
	return err
}

func (d *dB) GetCompose(jobId uuid.UUID, orgId string) (*ComposeEntry, error) {
	ctx := context.Background()
	result := d.pool.QueryRowContext(ctx, sqlGetCompose, orgId, jobId)

	var compose ComposeEntry
	err := result.Scan(&compose.Id, &compose.Request, &compose.CreatedAt, &compose.ImageName)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ComposeNotFoundError
		} else {
			return nil, err
		}
	}
	return &compose, nil
}

func (d *dB) GetComposeImageType(jobId uuid.UUID, orgId string) (string, error) {
	sqlGetComposeImageType := func() string {
		if d.dbType == dbTypePSQL {
			return sqlGetComposeImageTypePSQL
		}
	}

	ctx := context.Background()
	result := d.pool.QueryRowContext(ctx, sqlGetComposeImageType(), orgId, jobId)

	var imageType string
	err := result.Scan(&imageType)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", ComposeNotFoundError
		} else {
			return "", err
		}
	}
	return imageType, nil
}

func (d *dB) GetComposes(orgId string, since time.Duration, limit, offset int) ([]ComposeEntry, int, error) {
	ctx := context.Background()
	result, err := d.pool.QueryContext(ctx, sqlGetComposes, orgId, since, limit, offset)
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
	err = d.pool.QueryRowContext(ctx, sqlCountActiveComposesSince, orgId, since).Scan(&count)
	if err != nil {
		return nil, 0, err
	}

	return composes, count, nil
}

func (d *dB) CountComposesSince(orgId string, duration time.Duration) (int, error) {
	ctx := context.Background()
	var count int
	err := d.pool.QueryRowContext(ctx,
		sqlCountComposesSince,
		orgId, duration).Scan(&count)
	if err != nil {
		return 0, err
	}

	return count, nil
}

func (d *dB) DeleteCompose(jobId uuid.UUID, orgId string) error {
	ctx := context.Background()
	tag, err := d.pool.ExecContext(ctx, sqlDeleteCompose, orgId, jobId)
	if err != nil {
		return err
	}

	ra, err := tag.RowsAffected()
	if err != nil {
		return err
	}

	if ra != 1 {
		return ComposeNotFoundError
	}

	return err
}

func (d *dB) InsertClone(composeId, cloneId uuid.UUID, request json.RawMessage) error {
	ctx := context.Background()
	_, err := d.pool.ExecContext(ctx, sqlInsertClone, cloneId, composeId, request)
	return err
}

func (d *dB) GetClonesForCompose(composeId uuid.UUID, orgId string, limit, offset int) ([]CloneEntry, int, error) {
	ctx := context.Background()
	rows, err := d.pool.QueryContext(ctx, sqlGetClonesForCompose, composeId, orgId, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var clones []CloneEntry
	for rows.Next() {
		var id uuid.UUID
		var request json.RawMessage
		var createdAt time.Time
		err = rows.Scan(&id, &request, &createdAt)
		if err != nil {
			return nil, 0, err
		}
		clones = append(clones, CloneEntry{
			id,
			request,
			createdAt,
		})
	}
	if err = rows.Err(); err != nil {
		return nil, 0, err
	}

	var count int
	err = d.pool.QueryRowContext(ctx, sqlCountClonesForCompose, composeId, orgId).Scan(&count)
	if err != nil {
		return nil, 0, err
	}

	return clones, count, nil

}

func (d *dB) GetClone(id uuid.UUID, orgId string) (*CloneEntry, error) {
	ctx := context.Background()
	var clone CloneEntry
	err := d.pool.QueryRowContext(ctx, sqlGetClone, id, orgId).Scan(&clone.Id, &clone.Request, &clone.CreatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, CloneNotFoundError
		} else {
			return nil, err
		}
	}

	return &clone, nil
}
