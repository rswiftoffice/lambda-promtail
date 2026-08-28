package main

import (
	"context"
	"net/http"
	"net/url"
	"testing"
	"time"

	"github.com/grafana/dskit/backoff"
	"github.com/grafana/loki/v3/pkg/logproto"
)

// recordingRoundTripper records whether any HTTP request was issued.
type recordingRoundTripper struct{ called bool }

func (r *recordingRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	r.called = true
	return &http.Response{StatusCode: 200, Body: http.NoBody, Header: make(http.Header)}, nil
}

// TestSendToPromtail_EmptyBatchSkipsHTTP verifies that an empty batch never
// issues an HTTP push. Sending an empty PushRequest makes Loki return a
// non-retryable 422 that DLQs the whole SQS message. The guard in
// sendToPromtail must short-circuit before calling the HTTP client.
//
// This test FAILS if the `if entriesCount == 0 { return nil }` guard is removed.
func TestSendToPromtail_EmptyBatchSkipsHTTP(t *testing.T) {
	// writeAddress is a package global read by send(); set a dummy value.
	prev := writeAddress
	writeAddress, _ = url.Parse("http://localhost:3100/loki/api/v1/push")
	defer func() { writeAddress = prev }()

	rt := &recordingRoundTripper{}
	c := &promtailClient{
		config: &promtailClientConfig{
			backoff: &backoff.Config{MinBackoff: time.Millisecond, MaxBackoff: time.Millisecond, MaxRetries: 1},
			http:    &httpClientConfig{timeout: time.Second},
		},
		http: &http.Client{Transport: rt},
		log:  NewLogger("error"),
	}

	// Empty batch → zero entries.
	b := &batch{streams: map[string]*logproto.Stream{}, processor: &LokiStages{}}

	if err := c.sendToPromtail(context.Background(), b); err != nil {
		t.Fatalf("empty batch should return nil, got: %v", err)
	}
	if rt.called {
		t.Fatal("empty batch must NOT issue an HTTP request (would trigger Loki 422 → DLQ)")
	}
}

// TestEmptyBatchReportsZeroEntries is a supporting precondition check: an empty
// batch must report zero entries so the guard above can detect it.
func TestEmptyBatchReportsZeroEntries(t *testing.T) {
	b := &batch{streams: map[string]*logproto.Stream{}, processor: &LokiStages{}}
	if _, cnt := b.createPushRequest(); cnt != 0 {
		t.Fatalf("expected 0 entries from createPushRequest, got %d", cnt)
	}
	if _, entries, err := b.encode(); err != nil {
		t.Fatal(err)
	} else if entries != 0 {
		t.Fatalf("expected 0 entries from encode, got %d", entries)
	}
}
