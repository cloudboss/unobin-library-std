package net

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/cloudboss/unobin/pkg/runtime"
)

// HTTPAction issues an HTTP request and captures the response.
type HTTPAction struct {
	URL     string
	Method  *string
	Headers *map[string]string
	Body    *string
	Timeout *time.Duration
}

// HTTPActionOutput is the captured response. The action returns an error only
// when the request can't be built or the transport fails, not on HTTP
// error status codes. HTTP status codes are returned as data in Status.
type HTTPActionOutput struct {
	Status     int
	StatusText string
	Headers    map[string][]string
	Body       string
	Duration   time.Duration
}

// Run issues the request. A nil Method uses net/http's GET default.
// Timeout applies to the whole request including reading the response body.
func (a *HTTPAction) Run(ctx context.Context, _ runtime.NoConfig) (*HTTPActionOutput, error) {
	if a.URL == "" {
		return nil, errors.New("url is required")
	}
	var body io.Reader
	if a.Body != nil && *a.Body != "" {
		body = strings.NewReader(*a.Body)
	}
	method := ""
	if a.Method != nil {
		method = *a.Method
	}
	req, err := http.NewRequestWithContext(ctx, method, a.URL, body)
	if err != nil {
		return nil, err
	}
	if a.Headers != nil {
		for k, v := range *a.Headers {
			req.Header.Set(k, v)
		}
	}

	client := &http.Client{}
	if a.Timeout != nil && *a.Timeout > 0 {
		client.Timeout = *a.Timeout
	}

	start := time.Now()
	resp, err := client.Do(req)
	duration := time.Since(start)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	return &HTTPActionOutput{
		Status:     resp.StatusCode,
		StatusText: resp.Status,
		Headers:    resp.Header,
		Body:       string(bodyBytes),
		Duration:   duration,
	}, nil
}
