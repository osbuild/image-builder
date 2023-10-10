package db

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
)

const (
	sqlInsertBlueprint = `
		INSERT INTO blueprints(id, org_id, account_number, name, description, body_version, body)
		VALUES($1, $2, $3, $4, $5, $6, $7)`

	sqlGetBlueprint = `
		SELECT name, description, body_version, body
		FROM blueprints WHERE id = $1 AND org_id = $2 AND account_number = $3`

	sqlDeleteBlueprint = `DELETE FROM blueprints WHERE id = $1 AND org_id = $2 AND account_number = $3`
)

func (db *dB) InsertBlueprint(id uuid.UUID, orgID, accountNumber, name, description string, bodyVersion int, body json.RawMessage) error {
	ctx := context.Background()
	conn, err := db.Pool.Acquire(ctx)
	if err != nil {
		return err
	}
	defer conn.Release()

	tag, err := conn.Exec(ctx, sqlInsertBlueprint, id, orgID, accountNumber, name, description, bodyVersion, body)
	if tag.RowsAffected() != 1 {
		return fmt.Errorf("%w, expected 1, returned %d", AffectedRowsMismatchError, tag.RowsAffected())
	}
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
	err = row.Scan(&result.Name, &result.Description, &result.Version, &result.Body)
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
