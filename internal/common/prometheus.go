package common

import (
	"strings"

	"github.com/labstack/echo/v4"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	httpDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name: "image_builder_http_duration_seconds",
		Help: "Duration of HTTP requests.",
	}, []string{"path"})
)

var (
	composeRequests = promauto.NewCounter(prometheus.CounterOpts{
		Name: "image_builder_compose_requests_total",
		Help: "Total number of compose requests.",
	})
)

var (
	ComposeErrors = promauto.NewCounter(prometheus.CounterOpts{
		Name: "image_builder_compose_errors",
		Help: "Number of internal server errors.",
	})
)

func PrometheusMW(nextHandler echo.HandlerFunc) echo.HandlerFunc {
	return func(ctx echo.Context) error {
		if strings.HasSuffix(ctx.Path(), "/compose") {
			composeRequests.Inc()
		}

		timer := prometheus.NewTimer(httpDuration.WithLabelValues(ctx.Path()))
		defer timer.ObserveDuration()
		return nextHandler(ctx)
	}
}
