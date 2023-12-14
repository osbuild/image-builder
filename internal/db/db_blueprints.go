package db

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

const (
	sqlInsertBlueprint = `
		INSERT INTO blueprints(id, org_id, account_number, name, description)
		VALUES($1, $2, $3, $4, $5)`

	sqlInsertVersion = `
		INSERT INTO blueprint_versions(id, blueprint_id, version, body)
		VALUES($1, $2, $3, $4)`

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
)

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
