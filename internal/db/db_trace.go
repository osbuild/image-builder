package db

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/sirupsen/logrus"
)

// Used for pgx logging with context information
type dbTracer struct{}

func (dt *dbTracer) TraceQueryStart(ctx context.Context, conn *pgx.Conn, data pgx.TraceQueryStartData) context.Context {
	logrus.WithContext(ctx).Debugf("Executing SQL with args %v: %s", data.Args, data.SQL)
	return ctx
}

func (dt *dbTracer) TraceQueryEnd(ctx context.Context, conn *pgx.Conn, data pgx.TraceQueryEndData) {
	// no-op
}
