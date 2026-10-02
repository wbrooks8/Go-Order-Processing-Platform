// Handles the custom metrics for prometheus
package http

import (
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
)

var requestCounter = prometheus.NewCounterVec(
	prometheus.CounterOpts{
		Name: "http_requests_total",
		Help: "Total number of HTTP requests.",
	},
	[]string{"method", "route", "status"},
)

var requestHistogram = prometheus.NewHistogramVec(
	prometheus.HistogramOpts{
		Name: "http_request_duration_seconds",
		Help: "Duration of HTTP requests",
	},
	[]string{"method", "route"},
)

func init() {
	prometheus.MustRegister(requestCounter)
	prometheus.MustRegister(requestHistogram)
}

func requestMetrics(context *gin.Context) {
	currTime := time.Now()
	context.Next()
	route := context.FullPath()
	if route == "" {
		route = "unmatched"
	}

	requestCounter.WithLabelValues(
		context.Request.Method,
		route,
		strconv.Itoa(context.Writer.Status()),
	).Inc()
	elapsedTime := time.Since(currTime)
	requestHistogram.WithLabelValues(
		context.Request.Method,
		route,
	).Observe(elapsedTime.Seconds())
}
