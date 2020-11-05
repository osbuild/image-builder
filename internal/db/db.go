package db

import (
	"context"
	"fmt"

	"github.com/jackc/pgtype"
	pgtypeuuid "github.com/jackc/pgtype/ext/gofrs-uuid"
	"github.com/jackc/pgx/v4"
	"github.com/jackc/pgx/v4/pgxpool"
)

type DB struct {
	Pool *pgxpool.Pool
}

// Run a local container: sudo podman run -p5432:5432 --name imagebuilder -ePOSTGRES_PASSWORD=foobar -d postgres
func InitDBConnectionPool(connStr string) (*DB, error) {
	dbConfig, err := pgxpool.ParseConfig(connStr)
	if err != nil {
		return nil, err
	}
	dbConfig.AfterConnect = func(ctx context.Context, conn *pgx.Conn) error {
		conn.ConnInfo().RegisterDataType(pgtype.DataType{
			Value: &pgtypeuuid.UUID{},
			Name:  "uuid",
			OID:   pgtype.UUIDOID,
		})
		return nil
	}

	pool, err := pgxpool.ConnectConfig(context.Background(), dbConfig)
	if err != nil {
		return nil, err
	}

	return &DB{pool}, nil
}

func encodeUUID(src [16]byte) string {
	return fmt.Sprintf("%x-%x-%x-%x-%x", src[0:4], src[4:6], src[6:8], src[8:10], src[10:16])
}

func (db *DB) InsertImage(uuid string) error {
	conn, err := db.Pool.Acquire(context.Background())
	if err != nil {
		return err
	}
	defer conn.Release()

	_, err = conn.Query(context.Background(), "INSERT INTO images(Job) VALUES ($1);", uuid)
	return err
}

func (db *DB) GetImages() ([]string, error) {
	conn, err := db.Pool.Acquire(context.Background())
	if err != nil {
		return nil, err
	}
	defer conn.Release()

	result, err := conn.Query(context.Background(), "SELECT Job FROM images;")
	if err != nil {
		return nil, err
	}

	var jobs []string
	var res pgtype.UUID
	for result.Next() {
		err = result.Scan(&res)
		if err != nil {
			return nil, err
		}
		jobs = append(jobs, encodeUUID(res.Bytes))
	}
	return jobs, nil
}
