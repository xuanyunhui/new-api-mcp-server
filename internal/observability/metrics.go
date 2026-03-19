package observability

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type Metrics struct {
	ToolRequestsTotal       *prometheus.CounterVec
	ToolRequestDuration     *prometheus.HistogramVec
	UpstreamRequestsTotal   *prometheus.CounterVec
	UpstreamRequestDuration *prometheus.HistogramVec
}

func NewMetrics(reg prometheus.Registerer) *Metrics {
	m := &Metrics{
		ToolRequestsTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "mcp_tool_requests_total",
				Help: "Total number of MCP tool requests",
			},
			[]string{"tool", "status"},
		),
		ToolRequestDuration: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "mcp_tool_request_duration_seconds",
				Help:    "Duration of MCP tool requests in seconds",
				Buckets: prometheus.DefBuckets,
			},
			[]string{"tool"},
		),
		UpstreamRequestsTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "mcp_upstream_requests_total",
				Help: "Total number of upstream API requests",
			},
			[]string{"method", "path", "status_code"},
		),
		UpstreamRequestDuration: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "mcp_upstream_request_duration_seconds",
				Help:    "Duration of upstream API requests in seconds",
				Buckets: prometheus.DefBuckets,
			},
			[]string{"method", "path"},
		),
	}

	reg.MustRegister(
		m.ToolRequestsTotal,
		m.ToolRequestDuration,
		m.UpstreamRequestsTotal,
		m.UpstreamRequestDuration,
	)

	if gathererReg, ok := reg.(*prometheus.Registry); ok {
		gathererReg.MustRegister(collectors.NewGoCollector())
	}

	return m
}

func Handler(reg *prometheus.Registry) http.Handler {
	return promhttp.HandlerFor(reg, promhttp.HandlerOpts{})
}
