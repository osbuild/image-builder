package db

import (
	"database/sql"
	"os"
	"path"
	"path/filepath"
	"strings"

	_ "github.com/mattn/go-sqlite3"
)

// The SQLite db is used by the v1 unit tests
func SQLite(tempDir, migrationsDir string) (DB, error) {
	file, err := os.CreateTemp(tempDir, "file:*.db")
	if err != nil {
		return nil, err
	}

	pool, err := sql.Open("sqlite3", file.Name())
	if err != nil {
		return nil, err
	}
	defer func() {
		if err != nil {
			//#nosec G104
			pool.Close()
		}
	}()

	migrations, err := os.ReadDir(migrationsDir)
	if err != nil {
		return nil, err
	}

	for _, mbase := range migrations {
		if !strings.HasSuffix(mbase.Name(), ".sql") {
			continue
		}

		m := path.Join(migrationsDir, mbase.Name())
		mSql, err := os.ReadFile(filepath.Clean(m))
		if err != nil {
			return nil, err
		}

		_, err = pool.Exec(string(mSql))
		if err != nil {
			return nil, err
		}
	}

	return &dB{pool, dbTypeSQLite}, nil
}
