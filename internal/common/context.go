package common

import (
	"context"

	"github.com/sirupsen/logrus"
)

type ctxKey int

const (
	requestIdCtx         ctxKey = iota
	insightsRequestIdCtx ctxKey = iota
	requestDataCtx       ctxKey = iota
)

func WithRequestId(ctx context.Context, requestId string) context.Context {
	return context.WithValue(ctx, requestIdCtx, requestId)
}

func RequestId(ctx context.Context) string {
	rid := ctx.Value(requestIdCtx)
	if rid == nil {
		return ""
	}
	return rid.(string)
}

func WithInsightsRequestId(ctx context.Context, insightsRequestId string) context.Context {
	return context.WithValue(ctx, insightsRequestIdCtx, insightsRequestId)
}

func InsightsRequestId(ctx context.Context) string {
	iid := ctx.Value(insightsRequestIdCtx)
	if iid == nil {
		return ""
	}
	return iid.(string)
}

func WithRequestData(ctx context.Context, data logrus.Fields) context.Context {
	return context.WithValue(ctx, requestDataCtx, data)
}

func RequestData(ctx context.Context) logrus.Fields {
	rd := ctx.Value(requestDataCtx)
	if rd == nil {
		return logrus.Fields{}
	}
	return rd.(logrus.Fields)
}
