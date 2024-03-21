package db

import (
	"encoding/json"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/require"
)

func TestFormatSqlSimple(t *testing.T) {
	data := pgx.TraceQueryStartData{
		SQL: "SELECT $1 $2",
		Args: []any{
			"hello",
			13,
		},
	}

	result := formatSqlLog(data)
	require.Equal(t, "Executing SQL: SELECT $1 $2; args: [hello 13]", result)
}

func TestFormatSqlByteSlice(t *testing.T) {
	data := pgx.TraceQueryStartData{
		SQL: "SELECT $1 $2",
		Args: []any{
			[]byte("hello"),
			13,
		},
	}

	result := formatSqlLog(data)
	require.Equal(t, "Executing SQL: SELECT $1 $2; args: [hello 13]", result)
}

func TestFormatSqlRawMessage(t *testing.T) {
	data := pgx.TraceQueryStartData{
		SQL: "SELECT $1 $2",
		Args: []any{
			json.RawMessage("hello"),
			13,
		},
	}

	result := formatSqlLog(data)
	require.Equal(t, "Executing SQL: SELECT $1 $2; args: [hello 13]", result)
}

func TestFormatSqlLongSlice(t *testing.T) {
	data := pgx.TraceQueryStartData{
		SQL: "SELECT $1 $2",
		Args: []any{
			[]byte("123456789012345678901"),
		},
	}

	result := formatSqlLog(data)
	require.Equal(t, "Executing SQL: SELECT $1 $2; args: [12345678901234567...]", result)
}
