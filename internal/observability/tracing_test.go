package observability

import (
	"context"
	"testing"
)

func TestSetupTracing_NoEndpoint(t *testing.T) {
	shutdown, err := SetupTracing(context.Background(), "", "test-service")
	if err != nil {
		t.Fatalf("SetupTracing() error: %v", err)
	}
	if shutdown == nil {
		t.Fatal("shutdown function should not be nil")
	}
	if err := shutdown(context.Background()); err != nil {
		t.Errorf("shutdown error: %v", err)
	}
}

func TestSetupTracing_WithEndpoint(t *testing.T) {
	shutdown, err := SetupTracing(context.Background(), "http://localhost:4318", "test-service")
	if err != nil {
		t.Fatalf("SetupTracing() error: %v", err)
	}
	if shutdown == nil {
		t.Fatal("shutdown function should not be nil")
	}
	shutdown(context.Background())
}
