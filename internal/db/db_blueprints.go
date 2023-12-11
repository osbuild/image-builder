package db

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v4"
)

type ComposeEntryWithVersion struct {
	ComposeEntry
	BlueprintId      uuid.UUID
	BlueprintVersion int
}

const (
	sqlInsertBlueprint = `
		INSERT INTO blueprints(id, org_id, account_number, name, description)
		VALUES($1, $2, $3, $4, $5)`

	sqlInsertVersion = `
		INSERT INTO blueprint_versions(id, blueprint_id, version, body)
		VALUES($1, $2, $3, $4)`

	sqlGetBlueprintComposes = `
		SELECT blueprint_versions.version, composes.job_id, composes.request, composes.created_at, composes.image_name, composes.client_id
		FROM composes INNER JOIN blueprint_versions ON composes.blueprint_version_id = blueprint_versions.id
		WHERE composes.org_id = $1
		  	AND blueprint_versions.blueprint_id = $2
			AND CURRENT_TIMESTAMP - composes.created_at <= $3
			AND ($4::text[] is NULL OR composes.request->'image_requests'->0->>'image_type' <> ALL($4))
			AND composes.deleted = FALSE
		ORDER BY composes.created_at DESC
		LIMIT $5 OFFSET $6`

	sqlCountActiveBlueprintComposesSince = `
		SELECT COUNT(*)
		FROM composes INNER JOIN blueprint_versions ON composes.blueprint_version_id = blueprint_versions.id
		WHERE org_id=$1 AND blueprint_versions.blueprint_id = $2 AND CURRENT_TIMESTAMP - composes.created_at <= $3 AND composes.deleted = FALSE
		AND ($4::text[] is NULL OR composes.request->'image_requests'->0->>'image_type' <> ALL($4))`

	sqlGetBlueprint = `
		SELECT blueprints.id, blueprint_versions.id, blueprints.name, blueprints.description, blueprint_versions.version, blueprint_versions.body
		FROM blueprints INNER JOIN blueprint_versions ON blueprint_versions.blueprint_id = blueprints.id
		WHERE blueprints.id = $1 AND blueprints.org_id = $2 AND blueprints.account_number = $3    
		ORDER BY blueprint_versions.created_at DESC LIMIT 1`

	sqlDeleteBlueprint = `DELETE FROM blueprints WHERE id = $1 AND org_id = $2 AND account_number = $3`
)

func (db *dB) GetBlueprintComposes(orgId string, blueprintId uuid.UUID, since time.Duration, limit, offset int, ignoreImageTypes []string) ([]ComposeEntryWithVersion, int, error) {
	ctx := context.Background()
	conn, err := db.Pool.Acquire(ctx)
	if err != nil {
		return nil, 0, err
	}
	defer conn.Release()
	result, err := conn.Query(ctx, sqlGetBlueprintComposes, orgId, blueprintId, since, ignoreImageTypes, limit, offset)
	defer result.Close()

	if err != nil {
		return nil, 0, err
	}

	var composes []ComposeEntryWithVersion
	for result.Next() {
		var version int
		var jobId uuid.UUID
		var request json.RawMessage
		var createdAt time.Time
		var imageName *string
		var clientId *string
		err = result.Scan(&version, &jobId, &request, &createdAt, &imageName, &clientId)
		if err != nil {
			return nil, 0, err
		}
		composes = append(composes, ComposeEntryWithVersion{
			ComposeEntry{
				jobId,
				request,
				createdAt,
				imageName,
				clientId,
			},
			blueprintId,
			version,
		})
	}
	if err = result.Err(); err != nil {
		return nil, 0, err
	}

	var count int
	err = conn.QueryRow(ctx, sqlCountActiveBlueprintComposesSince, orgId, blueprintId, since, ignoreImageTypes).Scan(&count)
	if err != nil {
		return nil, 0, err
	}

	return composes, count, nil
}

func (db *dB) InsertBlueprint(id uuid.UUID, versionId uuid.UUID, orgID, accountNumber, name, description string, body json.RawMessage) error {
	ctx := context.Background()
	conn, err := db.Pool.Acquire(ctx)
	if err != nil {
		return err
	}
	defer conn.Release()

	err = db.withTransaction(ctx, func(tx pgx.Tx) error {
		tag, txErr := tx.Exec(ctx, sqlInsertBlueprint, id, orgID, accountNumber, name, description)
		if tag.RowsAffected() != 1 {
			return fmt.Errorf("%w, expected 1, returned %d", AffectedRowsMismatchError, tag.RowsAffected())
		}
		if txErr != nil {
			return txErr
		}

		tag, txErr = tx.Exec(ctx, sqlInsertVersion, versionId, id, 1, body)
		if tag.RowsAffected() != 1 {
			return fmt.Errorf("%w, expected 1, returned %d", AffectedRowsMismatchError, tag.RowsAffected())
		}
		if txErr != nil {
			return txErr
		}
		return nil
	})
	return err
}

func (db *dB) GetBlueprint(id uuid.UUID, orgID, accountNumber string) (*BlueprintEntry, error) {
	ctx := context.Background()
	conn, err := db.Pool.Acquire(ctx)
	if err != nil {
		return nil, err
	}
	defer conn.Release()

	var result BlueprintEntry
	row := conn.QueryRow(ctx, sqlGetBlueprint, id, orgID, accountNumber)
	err = row.Scan(&result.Id, &result.VersionId, &result.Name, &result.Description, &result.Version, &result.Body)
	if err != nil {
		return nil, err
	}

	return &result, err
}

func (db *dB) DeleteBlueprint(id uuid.UUID, orgID, accountNumber string) error {
	ctx := context.Background()
	conn, err := db.Pool.Acquire(ctx)
	if err != nil {
		return err
	}
	defer conn.Release()

	tag, err := conn.Exec(ctx, sqlDeleteBlueprint, id, orgID, accountNumber)
	if tag.RowsAffected() != 1 {
		return fmt.Errorf("%w, expected 1, returned %d", AffectedRowsMismatchError, tag.RowsAffected())
	}
	return err
}
