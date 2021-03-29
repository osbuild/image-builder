package server

import (
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
	totalRequests = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "image_builder_http_requests_total",
		Help: "Total number of HTTP requests.",
	}, []string{"path"})
)

var (
	serverErrors = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "image_builder_internal_server_errors",
		Help: "Number of internal server errors.",
	}, []string{"path"})
)

func (s *Server) PrometheusMW(nextHandler echo.HandlerFunc) echo.HandlerFunc {
	return func(ctx echo.Context) error {
		totalRequests.WithLabelValues(ctx.Path()).Inc()
		timer := prometheus.NewTimer(httpDuration.WithLabelValues(ctx.Path()))
		timer.ObserveDuration()
		return nextHandler(ctx)
	}
}
