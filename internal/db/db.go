package db

import (
	"database/sql"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
	_ "github.com/jackc/pgx/v5/stdlib"
)

var ComposeNotFoundError = errors.New("Compose not found")
var CloneNotFoundError = errors.New("Clone not found")

var UnsupportedQuery = errors.New("Unsupported query")

type DB interface {
	InsertCompose(jobId uuid.UUID, accountNumber, orgId string, imageName *string, request json.RawMessage) error
	GetComposes(orgId string, since time.Duration, limit, offset int) ([]ComposeEntry, int, error)
	GetCompose(jobId uuid.UUID, orgId string) (*ComposeEntry, error)
	GetComposeImageType(jobId uuid.UUID, orgId string) (string, error)
	CountComposesSince(orgId string, duration time.Duration) (int, error)
	DeleteCompose(jobId uuid.UUID, orgId string) error

	InsertClone(composeId, cloneId uuid.UUID, request json.RawMessage) error
	GetClonesForCompose(composeId uuid.UUID, orgId string, limit, offset int) ([]CloneEntry, int, error)
	GetClone(id uuid.UUID, orgId string) (*CloneEntry, error)

	Close() error
}

type ComposeEntry struct {
	Id        uuid.UUID
	Request   json.RawMessage
	CreatedAt time.Time
	ImageName *string
}

type CloneEntry struct {
	Id        uuid.UUID
	Request   json.RawMessage
	CreatedAt time.Time
}

type DBType string

const (
	dbTypeSQLite DBType = "sqlite"
	dbTypePSQL   DBType = "psql"
)

// private
type dB struct {
	pool   *sql.DB
	dbType DBType
}

func (d *dB) Close() error {
	return d.pool.Close()
}
