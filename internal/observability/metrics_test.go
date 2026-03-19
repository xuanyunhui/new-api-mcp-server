package observability

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus"
)

func TestNewMetrics(t *testing.T) {
	reg := prometheus.NewRegistry()
	m := NewMetrics(reg)
	if m.ToolRequestsTotal == nil {
		t.Error("ToolRequestsTotal is nil")
	}
	if m.ToolRequestDuration == nil {
		t.Error("ToolRequestDuration is nil")
	}
	if m.UpstreamRequestsTotal == nil {
		t.Error("UpstreamRequestsTotal is nil")
	}
	if m.UpstreamRequestDuration == nil {
		t.Error("UpstreamRequestDuration is nil")
	}
}

func TestMetrics_RecordToolRequest(t *testing.T) {
	reg := prometheus.NewRegistry()
	m := NewMetrics(reg)
	m.ToolRequestsTotal.WithLabelValues("list_models", "success").Inc()

	metrics, err := reg.Gather()
	if err != nil {
		t.Fatalf("Gather() error: %v", err)
	}
	found := false
	for _, mf := range metrics {
		if mf.GetName() == "mcp_tool_requests_total" {
			found = true
		}
	}
	if !found {
		t.Error("mcp_tool_requests_total metric not found")
	}
}
