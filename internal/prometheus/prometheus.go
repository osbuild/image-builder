package prometheus

import (
	"errors"
	"regexp"
	"strconv"
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

// TODO deprecate
var (
	composeRequests = promauto.NewCounter(prometheus.CounterOpts{
		Name:      "compose_requests_total",
		Namespace: namespace,
		Subsystem: subsystem,
		Help:      "Total number of compose requests.",
	})
)

// TODO deprecate
var (
	ComposeErrors = promauto.NewCounter(prometheus.CounterOpts{
		Name:      "compose_errors",
		Namespace: namespace,
		Subsystem: subsystem,
		Help:      "Number of internal server errors.",
	})
)

var (
	ReqCounter = promauto.NewCounterVec(prometheus.CounterOpts{
		Name:      "request_count",
		Namespace: namespace,
		Subsystem: subsystem,
		Help:      "total number of http requests",
	}, []string{"method", "path", "code"})
)

func pathLabel(path string) string {
	r := regexp.MustCompile(":(.*)")
	segments := strings.Split(path, "/")
	for i, segment := range segments {
		segments[i] = r.ReplaceAllString(segment, "-")
	}
	return strings.Join(segments, "/")
}

func PrometheusMW(nextHandler echo.HandlerFunc) echo.HandlerFunc {
	return func(ctx echo.Context) error {
		// TODO deprecate
		if strings.HasSuffix(ctx.Path(), "/compose") {
			composeRequests.Inc()
		}

		timer := prometheus.NewTimer(httpDuration.WithLabelValues(ctx.Path()))
		defer timer.ObserveDuration()
		return nextHandler(ctx)
	}
}

func StatusMiddleware(next echo.HandlerFunc) echo.HandlerFunc {
	return func(ctx echo.Context) error {
		// call the next handler to see if
		// an error occurred, see:
		// - https://github.com/labstack/echo/issues/1837#issuecomment-816399630
		// - https://github.com/labstack/echo/discussions/1820#discussioncomment-529428
		err := next(ctx)

		path := pathLabel(ctx.Path())
		method := ctx.Request().Method
		status := ctx.Response().Status

		httpErr := new(echo.HTTPError)
		if errors.As(err, &httpErr) {
			status = httpErr.Code
		}

		ReqCounter.WithLabelValues(
			method,
			path,
			strconv.Itoa(status),
		).Inc()

		return err
	}
}
