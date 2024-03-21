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
	translated := make([]string, len(data.Args))
	for k, v := range data.Args {
		switch value := v.(type) {
		case json.RawMessage:
			translated[k] = string(value)
		case []byte:
			translated[k] = string(value)
		default:
			translated[k] = fmt.Sprintf("%v", value)
		}
	}
	logrus.WithContext(ctx).Debugf("Executing SQL with args %v: %s", translated, data.SQL)
	return ctx
}

func (dt *dbTracer) TraceQueryEnd(ctx context.Context, conn *pgx.Conn, data pgx.TraceQueryEndData) {
	// no-op
}
