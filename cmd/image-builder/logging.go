package main

import (
	"context"
	"strings"

	"github.com/labstack/echo/v4"
	"github.com/labstack/gommon/random"
	"github.com/osbuild/image-builder/internal/common"
	"github.com/sirupsen/logrus"
)

type ctxKey int

const (
	requestIdCtx         ctxKey = iota
	insightsRequestIdCtx ctxKey = iota
	requestDataCtx       ctxKey = iota
)

// Use request id from the standard context and add it to the message as a field.
type ctxHook struct {
}

func (h *ctxHook) Levels() []logrus.Level {
	return []logrus.Level{
		logrus.DebugLevel,
		logrus.InfoLevel,
		logrus.WarnLevel,
		logrus.ErrorLevel,
		logrus.FatalLevel,
		logrus.PanicLevel,
	}
}

func (h *ctxHook) Fire(e *logrus.Entry) error {
	if e.Context != nil {
		e.Data["request_id"] = e.Context.Value(requestIdCtx)
		e.Data["insights_id"] = e.Context.Value(insightsRequestIdCtx)
		rd := e.Context.Value(requestDataCtx).(logrus.Fields)
		for k, v := range rd {
			e.Data[k] = v
		}
	}

	return nil
}

// Extract/generate request id and store it in the standard context
func requestIdExtractMiddleware(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		// extract or generate request and insights id
		rid := c.Request().Header.Get("X-Rh-Edge-Request-Id")
		if rid != "" {
			rid = strings.TrimSuffix(rid, "\n")
		} else {
			rid = random.String(12)
		}

		iid := c.Request().Header.Get("X-Rh-Insights-Request-Id")
		if iid != "" {
			iid = strings.TrimSuffix(iid, "\n")
		} else {
			iid = random.String(12)
		}

		// create fields stored with every log statement
		rd := logrus.Fields{
			"method": c.Request().Method,
			"path":   c.Path(),
		}
		for _, key := range c.ParamNames() {
			// protect existing and the most important fields
			if _, ok := rd[key]; !(ok || key == "msg" || key == "level") {
				rd[key] = c.Param(key)
			}
		}

		// store it in a standard context
		ctx := c.Request().Context()
		ctx = context.WithValue(ctx, requestIdCtx, rid)
		ctx = context.WithValue(ctx, insightsRequestIdCtx, iid)
		ctx = context.WithValue(ctx, requestDataCtx, rd)
		c.SetRequest(c.Request().WithContext(ctx))

		// and set echo logger to be context logger
		ctxLogger := logrus.StandardLogger()
		c.SetLogger(&common.EchoLogrusLogger{
			Logger: ctxLogger,
			Ctx:    ctx,
		})

		if !SkipPath(c.Path()) {
			ctxLogger.WithContext(ctx).WithFields(rd).Debugf("Started request")
		}

		return next(c)
	}
}

func SkipPath(path string) bool {
	switch path {
	case "/metrics":
		return true
	case "/status":
		return true
	case "/ready":
		return true
	}

	return false
}
