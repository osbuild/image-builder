package db

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/sirupsen/logrus"
)

// Used for pgx logging with context information
type dbTracer struct{}

func (dt *dbTracer) TraceQueryStart(ctx context.Context, conn *pgx.Conn, data pgx.TraceQueryStartData) context.Context {
	logrus.WithContext(ctx).Debug(formatSqlLog(data))
	return ctx
}

func (dt *dbTracer) TraceQueryEnd(ctx context.Context, conn *pgx.Conn, data pgx.TraceQueryEndData) {
	// no-op
}

func formatSqlLog(data pgx.TraceQueryStartData) string {
	if logrus.GetLevel() > logrus.DebugLevel {
		return ""
	}

	d := make([]interface{}, len(data.Args))
	copy(d, data.Args)
	for i, v := range d {
		if b, ok := v.([]byte); ok {
			d[i] = ellipsis(string(b), 20)
		} else if j, ok := v.(json.RawMessage); ok {
			d[i] = ellipsis(string(j), 20)
		}
	}

	return fmt.Sprintf("Executing SQL: %s; args: %v", data.SQL, d)
}

func ellipsis(s string, maxLen int) string {
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	if maxLen < 3 {
		maxLen = 3
	}
	return string(runes[0:maxLen-3]) + "..."
}
