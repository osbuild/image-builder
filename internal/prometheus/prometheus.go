package prometheus

import (
	"strings"

	"github.com/labstack/echo/v4"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

const (
	namespace = "image_builder"
	subsystem = "crc"
)

var (
	httpDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:      "http_duration_seconds",
		Namespace: namespace,
		Subsystem: subsystem,
		Help:      "Duration of HTTP requests.",
		Buckets:   []float64{.025, .05, .075, .1, .2, .5, .75, 1, 1.5, 2, 3, 4, 5, 6, 8, 10, 12, 14, 16, 20},
	}, []string{"path"})
)

var (
	composeRequests = promauto.NewCounter(prometheus.CounterOpts{
		Name:      "compose_requests_total",
		Namespace: namespace,
		Subsystem: subsystem,
		Help:      "Total number of compose requests.",
	})
)

var (
	ComposeErrors = promauto.NewCounter(prometheus.CounterOpts{
		Name:      "compose_errors",
		Namespace: namespace,
		Subsystem: subsystem,
		Help:      "Number of internal server errors.",
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
