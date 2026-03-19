package client

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

type Source int

const (
	SourceRelay Source = iota
	SourceAPI
)

var tracer = otel.Tracer("client")

type Client struct {
	baseURL    string
	relayKey   string
	systemKey  string
	httpClient *http.Client
}

func New(baseURL, relayKey, systemKey string, timeout time.Duration) *Client {
	return &Client{
		baseURL:   baseURL,
		relayKey:  relayKey,
		systemKey: systemKey,
		httpClient: &http.Client{
			Timeout: timeout,
		},
	}
}

func (c *Client) Do(ctx context.Context, source Source, method, path string, queryParams map[string]string, headerParams map[string]string, body []byte) (*http.Response, error) {
	ctx, span := tracer.Start(ctx, "upstream.request",
		trace.WithAttributes(
			attribute.String("http.method", method),
			attribute.String("http.path", path),
		),
	)
	defer span.End()

	url := c.baseURL + path
	var reqBody *bytes.Reader
	if body != nil {
		reqBody = bytes.NewReader(body)
	} else {
		reqBody = bytes.NewReader(nil)
	}

	req, err := http.NewRequestWithContext(ctx, method, url, reqBody)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	key := c.relayKey
	if source == SourceAPI {
		key = c.systemKey
	}
	if key != "" {
		req.Header.Set("Authorization", "Bearer "+key)
	}

	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	// Apply custom header parameters (skip Authorization to prevent override)
	for k, v := range headerParams {
		if !strings.EqualFold(k, "Authorization") {
			req.Header.Set(k, v)
		}
	}

	if len(queryParams) > 0 {
		q := req.URL.Query()
		for k, v := range queryParams {
			q.Set(k, v)
		}
		req.URL.RawQuery = q.Encode()
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		span.RecordError(err)
		return nil, fmt.Errorf("upstream request: %w", err)
	}

	span.SetAttributes(attribute.Int("http.status_code", resp.StatusCode))

	return resp, nil
}
