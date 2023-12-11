package db

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type BlueprintCompose struct {
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
			AND ($3::integer IS NULL OR blueprint_versions.version = $3)
			AND CURRENT_TIMESTAMP - composes.created_at <= $4
			AND ($5::text[] is NULL OR composes.request->'image_requests'->0->>'image_type' <> ALL($5))
			AND composes.deleted = FALSE
		ORDER BY composes.created_at DESC
		LIMIT $6 OFFSET $7`

	sqlCountActiveBlueprintComposesSince = `
		SELECT COUNT(*)
		FROM composes INNER JOIN blueprint_versions ON composes.blueprint_version_id = blueprint_versions.id
		WHERE org_id=$1
			AND blueprint_versions.blueprint_id = $2
		  	AND ($3::integer IS NULL OR blueprint_versions.version = $3)
		  	AND CURRENT_TIMESTAMP - composes.created_at <= $4
		  	AND composes.deleted = FALSE
		AND ($5::text[] is NULL OR composes.request->'image_requests'->0->>'image_type' <> ALL($5))`

	sqlGetBlueprint = `
		SELECT blueprints.id, blueprint_versions.id, blueprints.name, blueprints.description, blueprint_versions.version, blueprint_versions.body
		FROM blueprints INNER JOIN blueprint_versions ON blueprint_versions.blueprint_id = blueprints.id
		WHERE blueprints.id = $1 AND blueprints.org_id = $2 AND blueprints.account_number = $3    
		ORDER BY blueprint_versions.created_at DESC LIMIT 1`

	sqlUpdateBlueprint = `
		UPDATE blueprints
		SET name = $3, description = $4
		WHERE id = $1 AND org_id = $2`

	sqlUpdateBlueprintVersion = `
		INSERT INTO blueprint_versions (id, blueprint_id, version, body)
		SELECT 
			$1,
			$2,
			MAX(version) + 1, 
			$3
		FROM 
			blueprint_versions
		WHERE 
			blueprint_id = $2
        AND EXISTS (
            SELECT 1
            FROM blueprints
            WHERE id = $2
            AND org_id = $4
        );`

	sqlDeleteBlueprint = `DELETE FROM blueprints WHERE id = $1 AND org_id = $2 AND account_number = $3`

	sqlGetBlueprints = `
		SELECT blueprints.id, blueprints.name, blueprints.description, MAX(blueprint_versions.version) as version, MAX(blueprint_versions.created_at) as last_modified_at
		FROM blueprints INNER JOIN blueprint_versions ON blueprint_versions.blueprint_id = blueprints.id
		WHERE blueprints.org_id = $1
		GROUP BY blueprints.id
		ORDER BY last_modified_at DESC
		LIMIT $2 OFFSET $3`

	sqlFindBlueprints = `
		SELECT blueprints.id, blueprints.name, blueprints.description, MAX(blueprint_versions.version) as version, MAX(blueprint_versions.created_at) as last_modified_at
		FROM blueprints INNER JOIN blueprint_versions ON blueprint_versions.blueprint_id = blueprints.id
		WHERE blueprints.org_id = $1 AND (blueprints.name ILIKE $4 OR blueprints.description ILIKE $4)
		GROUP BY blueprints.id
		ORDER BY last_modified_at DESC
		LIMIT $2 OFFSET $3`

	sqlCountFilteredBlueprints = `
		SELECT COUNT(*)
		FROM blueprints
		WHERE blueprints.org_id = $1 AND (blueprints.name ILIKE $2 OR blueprints.description ILIKE $2)`

	sqlGetBlueprintsCount = `
		SELECT COUNT(*)
		FROM blueprints
		WHERE blueprints.org_id = $1`
)

func (db *dB) CountBlueprintComposesSince(orgId string, blueprintId uuid.UUID, blueprintVersion *int, since time.Duration, ignoreImageTypes []string) (int, error) {
	ctx := context.Background()
	conn, err := db.Pool.Acquire(ctx)
	if err != nil {
		return 0, err
	}
	defer conn.Release()

	var count int
	err = conn.QueryRow(ctx, sqlCountActiveBlueprintComposesSince, orgId, blueprintId, blueprintVersion, since, ignoreImageTypes).Scan(&count)
	if err != nil {
		return 0, err
	}
	return count, nil
}

func (db *dB) GetBlueprintComposes(orgId string, blueprintId uuid.UUID, blueprintVersion *int, since time.Duration, limit, offset int, ignoreImageTypes []string) ([]BlueprintCompose, error) {
	ctx := context.Background()
	conn, err := db.Pool.Acquire(ctx)
	if err != nil {
		return nil, err
	}
	defer conn.Release()

	result, err := conn.Query(ctx, sqlGetBlueprintComposes, orgId, blueprintId, blueprintVersion, since, ignoreImageTypes, limit, offset)
	if err != nil {
		return nil, err
	}
	defer result.Close()

	var composes []BlueprintCompose
	for result.Next() {
		entry := BlueprintCompose{
			BlueprintId: blueprintId,
		}
		err = result.Scan(&entry.BlueprintVersion, &entry.Id, &entry.Request, &entry.CreatedAt, &entry.ImageName, &entry.ClientId)
		if err != nil {
			return nil, err
		}
		composes = append(composes, entry)
	}
	if err = result.Err(); err != nil {
		return nil, err
	}

	return composes, nil
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
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, BlueprintNotFoundError
		}
		return nil, err
	}

	return &result, err
}

func (db *dB) UpdateBlueprint(id uuid.UUID, blueprintId uuid.UUID, orgId string, name string, description string, body json.RawMessage) error {
	ctx := context.Background()
	conn, err := db.Pool.Acquire(ctx)
	if err != nil {
		return err
	}
	defer conn.Release()

	err = db.withTransaction(ctx, func(tx pgx.Tx) error {
		tag, txErr := tx.Exec(ctx, sqlUpdateBlueprint, blueprintId, orgId, name, description)
		if tag.RowsAffected() != 1 {
			return fmt.Errorf("blueprint not found: %w, expected 1, returned %d", AffectedRowsMismatchError, tag.RowsAffected())
		}
		if txErr != nil {
			return txErr
		}

		tag, txErr = tx.Exec(ctx, sqlUpdateBlueprintVersion, id, blueprintId, body, orgId)
		if tag.RowsAffected() != 1 {
			return fmt.Errorf("blueprint version not found: %w, expected 1, returned %d", AffectedRowsMismatchError, tag.RowsAffected())
		}
		if txErr != nil {
			return txErr
		}
		return nil
	})

	return err
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

func (db *dB) FindBlueprints(orgID, search string, limit, offset int) ([]BlueprintWithNoBody, int, error) {
	ctx := context.Background()
	conn, err := db.Pool.Acquire(ctx)
	if err != nil {
		return nil, 0, err
	}
	defer conn.Release()
	searchQuery := fmt.Sprintf("%s%s%s", "%", search, "%")
	rows, err := conn.Query(ctx, sqlFindBlueprints, orgID, limit, offset, searchQuery)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var blueprints []BlueprintWithNoBody
	for rows.Next() {
		var blueprint BlueprintWithNoBody
		err = rows.Scan(&blueprint.Id, &blueprint.Name, &blueprint.Description, &blueprint.Version, &blueprint.LastModifiedAt)
		if err != nil {
			return nil, 0, err
		}
		blueprints = append(blueprints, blueprint)
	}
	var count int
	err = conn.QueryRow(ctx, sqlCountFilteredBlueprints, orgID, searchQuery).Scan(&count)
	if err != nil {
		return nil, 0, err
	}

	return blueprints, count, nil
}

func (db *dB) GetBlueprints(orgID string, limit, offset int) ([]BlueprintWithNoBody, int, error) {
	ctx := context.Background()
	conn, err := db.Pool.Acquire(ctx)
	if err != nil {
		return nil, 0, err
	}
	defer conn.Release()

	rows, err := conn.Query(ctx, sqlGetBlueprints, orgID, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var blueprints []BlueprintWithNoBody
	for rows.Next() {
		var blueprint BlueprintWithNoBody
		err = rows.Scan(&blueprint.Id, &blueprint.Name, &blueprint.Description, &blueprint.Version, &blueprint.LastModifiedAt)
		if err != nil {
			return nil, 0, err
		}
		blueprints = append(blueprints, blueprint)
	}
	var count int
	err = conn.QueryRow(ctx, sqlGetBlueprintsCount, orgID).Scan(&count)
	if err != nil {
		return nil, 0, err
	}

	return blueprints, count, nil
}
