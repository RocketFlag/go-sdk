package rocketflag

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// Mocking the http.RoundTripper interface to control HTTP responses
type MockRoundTripper struct {
	Response *http.Response
	Error    error
}

func (m *MockRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	return m.Response, m.Error
}

// Helper function to create a mock HTTP client
func MockClient(response *http.Response, err error) *http.Client {
	return &http.Client{
		Transport: &MockRoundTripper{
			Response: response,
			Error:    err,
		},
	}
}

func TestGetFlag_Success(t *testing.T) {
	// Expected flag status
	expectedFlag := &FlagStatus{Name: "test-flag", Enabled: true, ID: "123"}
	expectedFlagJSON, _ := json.Marshal(expectedFlag)

	// Mock response
	mockResponse := &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(bytes.NewReader(expectedFlagJSON)),
		Header:     http.Header{"Content-Type": []string{"application/json"}},
	}

	// Create a client with the mock HTTP client
	client := NewClient(WithHTTPClient(MockClient(mockResponse, nil)))

	// Call the function
	flag, err := client.GetFlag(context.Background(), "123", UserContext{"cohort": "beta"})

	// Assertions
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}
	if !reflect.DeepEqual(flag, expectedFlag) {
		t.Errorf("Expected flag: %+v, got: %+v", expectedFlag, flag)
	}
}

func TestGetFlag_ErrorParsingURL(t *testing.T) {
	// Create a client with an invalid URL
	client := NewClient(WithAPIURL(":invalid-url"))

	// Call the function
	_, err := client.GetFlag(context.Background(), "123", nil)

	// Assertions
	if err == nil {
		t.Fatal("Expected an error, got nil")
	}

	if !strings.Contains(err.Error(), "error parsing URL") {
		t.Errorf("Expected error message to contain 'error parsing URL', got: %v", err)
	}
}

func TestGetFlag_ErrorCreatingRequest(t *testing.T) {
	client := NewClient()

	// A nil context is the one input NewRequestWithContext refuses.
	var ctx context.Context
	_, err := client.GetFlag(ctx, "123", nil)

	if err == nil {
		t.Fatal("Expected an error, got nil")
	}
	if !strings.Contains(err.Error(), "error creating request") {
		t.Errorf("Expected error message to contain 'error creating request', got: %v", err)
	}
}

func TestGetFlag_ErrorMakingRequest(t *testing.T) {
	// Mock error
	mockError := errors.New("mock network error")

	// Create a client with the mock HTTP client that returns an error
	client := NewClient(WithHTTPClient(MockClient(nil, mockError)))

	// Call the function
	_, err := client.GetFlag(context.Background(), "123", nil)

	// Assertions
	if err == nil {
		t.Fatal("Expected an error, got nil")
	}
	if !strings.Contains(err.Error(), "error making request") {
		t.Errorf("Expected error message to contain 'error making request', got: %v", err)
	}
}

func TestGetFlag_ServerError(t *testing.T) {
	// Mock response with a 500 status code
	mockResponse := &http.Response{
		StatusCode: http.StatusInternalServerError,
		Body:       io.NopCloser(bytes.NewReader([]byte(""))),
		Status:     "500 Internal Server Error",
	}

	// Create a client with the mock HTTP client
	client := NewClient(WithHTTPClient(MockClient(mockResponse, nil)))

	// Call the function
	_, err := client.GetFlag(context.Background(), "123", nil)

	// Assertions
	if err == nil {
		t.Fatal("Expected an error, got nil")
	}
	if !strings.Contains(err.Error(), "error from server: 500 Internal Server Error") {
		t.Errorf("Expected error message to contain 'error from server: 500 Internal Server Error', got: %v", err)
	}
}

func TestGetFlag_ErrorDecodingResponse(t *testing.T) {
	// Mock response with invalid JSON
	mockResponse := &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(bytes.NewReader([]byte("invalid json"))),
		Header:     http.Header{"Content-Type": []string{"application/json"}},
	}

	// Create a client with the mock HTTP client
	client := NewClient(WithHTTPClient(MockClient(mockResponse, nil)))

	// Call the function
	_, err := client.GetFlag(context.Background(), "123", nil)

	// Assertions
	if err == nil {
		t.Fatal("Expected an error, got nil")
	}
	if !strings.Contains(err.Error(), "error decoding response") {
		t.Errorf("Expected error message to contain 'error decoding response', got: %v", err)
	}
}

func TestGetFlag_UserContext(t *testing.T) {
	// Expected flag status
	expectedFlag := &FlagStatus{Name: "test-flag", Enabled: true, ID: "123"}
	expectedFlagJSON, _ := json.Marshal(expectedFlag)

	// Mock response
	mockResponse := &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(bytes.NewReader(expectedFlagJSON)),
		Header:     http.Header{"Content-Type": []string{"application/json"}},
	}

	// Create a RoundTripFunc that captures the request for assertions
	var capturedRequest *http.Request
	rtFunc := func(req *http.Request) (*http.Response, error) {
		capturedRequest = req
		return mockResponse, nil
	}

	// Create a client with the mock HTTP client using the RoundTripFunc
	client := NewClient(WithHTTPClient(MockClient(mockResponse, nil)))
	client.client = &http.Client{
		Transport: RoundTripFunc(rtFunc),
	}

	// Call the function with user context
	userContext := UserContext{"cohort": "beta", "targetingKey": "user-42", "plan": "pro", "country": "AU"}
	_, err := client.GetFlag(context.Background(), "123", userContext)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	// Assertions
	if capturedRequest == nil {
		t.Fatal("Request was not captured")
	}

	expectedQuery := url.Values{}
	for k, v := range userContext {
		expectedQuery.Set(k, v)
	}
	actualQuery := capturedRequest.URL.Query()

	if !reflect.DeepEqual(actualQuery, expectedQuery) {
		t.Errorf("Expected query: %+v, got: %+v", expectedQuery, actualQuery)
	}
}

// countingTransport tracks how many requests pass through it and returns the
// supplied response (fresh body each time so multiple reads work).
type countingTransport struct {
	count int32
	body  []byte
}

func (t *countingTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	atomic.AddInt32(&t.count, 1)
	return &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(bytes.NewReader(t.body)),
		Header:     http.Header{"Content-Type": []string{"application/json"}},
	}, nil
}

func newCountingClient(t *testing.T, flag *FlagStatus, opts ...ClientOption) (*Client, *countingTransport) {
	t.Helper()
	body, err := json.Marshal(flag)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	transport := &countingTransport{body: body}
	httpClient := &http.Client{Transport: transport}
	allOpts := append([]ClientOption{WithHTTPClient(httpClient)}, opts...)
	return NewClient(allOpts...), transport
}

func TestGetFlag_CacheHit(t *testing.T) {
	flag := &FlagStatus{Name: "f", Enabled: true, ID: "123"}
	client, transport := newCountingClient(t, flag, WithCache(time.Minute))

	for i := 0; i < 3; i++ {
		got, err := client.GetFlag(context.Background(), "123", UserContext{"cohort": "beta"})
		if err != nil {
			t.Fatalf("call %d: %v", i, err)
		}
		if !reflect.DeepEqual(got, flag) {
			t.Fatalf("call %d: got %+v, want %+v", i, got, flag)
		}
	}

	if n := atomic.LoadInt32(&transport.count); n != 1 {
		t.Errorf("expected 1 HTTP request, got %d", n)
	}
}

func TestGetFlag_CacheExpiry(t *testing.T) {
	flag := &FlagStatus{Name: "f", Enabled: true, ID: "123"}
	client, transport := newCountingClient(t, flag, WithCache(10*time.Millisecond))

	if _, err := client.GetFlag(context.Background(), "123", nil); err != nil {
		t.Fatal(err)
	}
	time.Sleep(25 * time.Millisecond)
	if _, err := client.GetFlag(context.Background(), "123", nil); err != nil {
		t.Fatal(err)
	}

	if n := atomic.LoadInt32(&transport.count); n != 2 {
		t.Errorf("expected 2 HTTP requests after expiry, got %d", n)
	}
}

func TestGetFlag_CacheKeyIncludesUserContext(t *testing.T) {
	flag := &FlagStatus{Name: "f", Enabled: true, ID: "123"}
	client, transport := newCountingClient(t, flag, WithCache(time.Minute))

	if _, err := client.GetFlag(context.Background(), "123", UserContext{"cohort": "alpha"}); err != nil {
		t.Fatal(err)
	}
	if _, err := client.GetFlag(context.Background(), "123", UserContext{"cohort": "beta"}); err != nil {
		t.Fatal(err)
	}

	if n := atomic.LoadInt32(&transport.count); n != 2 {
		t.Errorf("expected 2 HTTP requests for differing contexts, got %d", n)
	}
}

func TestGetFlag_PerCallTTLDisables(t *testing.T) {
	flag := &FlagStatus{Name: "f", Enabled: true, ID: "123"}
	client, transport := newCountingClient(t, flag, WithCache(time.Minute))

	if _, err := client.GetFlag(context.Background(), "123", nil, WithCallTTL(0)); err != nil {
		t.Fatal(err)
	}
	if _, err := client.GetFlag(context.Background(), "123", nil, WithCallTTL(0)); err != nil {
		t.Fatal(err)
	}

	if n := atomic.LoadInt32(&transport.count); n != 2 {
		t.Errorf("expected 2 HTTP requests when per-call TTL disables cache, got %d", n)
	}
}

func TestGetFlag_PerCallTTLEnables(t *testing.T) {
	flag := &FlagStatus{Name: "f", Enabled: true, ID: "123"}
	client, transport := newCountingClient(t, flag)

	if _, err := client.GetFlag(context.Background(), "123", nil, WithCallTTL(time.Minute)); err != nil {
		t.Fatal(err)
	}
	if _, err := client.GetFlag(context.Background(), "123", nil, WithCallTTL(time.Minute)); err != nil {
		t.Fatal(err)
	}

	if n := atomic.LoadInt32(&transport.count); n != 1 {
		t.Errorf("expected 1 HTTP request when per-call TTL enables cache, got %d", n)
	}
}

func TestGetFlag_NoCacheByDefault(t *testing.T) {
	flag := &FlagStatus{Name: "f", Enabled: true, ID: "123"}
	client, transport := newCountingClient(t, flag)

	for i := 0; i < 3; i++ {
		if _, err := client.GetFlag(context.Background(), "123", nil); err != nil {
			t.Fatal(err)
		}
	}

	if n := atomic.LoadInt32(&transport.count); n != 3 {
		t.Errorf("expected 3 HTTP requests without caching, got %d", n)
	}
}

func TestGetFlag_CachedResultIsolatedFromCallerMutation(t *testing.T) {
	flag := &FlagStatus{Name: "original", Enabled: true, ID: "123"}
	client, transport := newCountingClient(t, flag, WithCache(time.Minute))

	first, err := client.GetFlag(context.Background(), "123", nil)
	if err != nil {
		t.Fatal(err)
	}
	first.Name = "mutated"

	second, err := client.GetFlag(context.Background(), "123", nil)
	if err != nil {
		t.Fatal(err)
	}
	if second.Name != "original" {
		t.Errorf("cached value was mutated by caller: got %q, want %q", second.Name, "original")
	}
	if n := atomic.LoadInt32(&transport.count); n != 1 {
		t.Errorf("expected 1 HTTP request, got %d", n)
	}
}

func TestGetFlag_PassesContextToRequest(t *testing.T) {
	type ctxKey struct{}
	var got any
	client := NewClient(WithHTTPClient(&http.Client{Transport: RoundTripFunc(func(req *http.Request) (*http.Response, error) {
		got = req.Context().Value(ctxKey{})
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(`{"name":"f","enabled":true,"id":"123"}`)),
		}, nil
	})}))

	ctx := context.WithValue(context.Background(), ctxKey{}, "trace-1")
	if _, err := client.GetFlag(ctx, "123", nil); err != nil {
		t.Fatal(err)
	}
	if got != "trace-1" {
		t.Errorf("expected the request to carry the caller's context, got value %v", got)
	}
}

func TestGetFlag_CancelledContext(t *testing.T) {
	var hits int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&hits, 1)
		fmt.Fprint(w, `{"name":"f","enabled":true,"id":"123"}`)
	}))
	defer server.Close()
	client := NewClient(WithAPIURL(server.URL))

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := client.GetFlag(ctx, "123", nil)

	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got: %v", err)
	}
	if n := atomic.LoadInt32(&hits); n != 0 {
		t.Errorf("expected no request to reach the server, got %d", n)
	}
}

func TestGetFlag_ContextDeadline(t *testing.T) {
	release := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-r.Context().Done():
		case <-release:
		}
	}))
	defer server.Close()
	defer close(release)
	client := NewClient(WithAPIURL(server.URL))

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	start := time.Now()
	_, err := client.GetFlag(ctx, "123", nil)

	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected context.DeadlineExceeded, got: %v", err)
	}
	if elapsed := time.Since(start); elapsed > 2*time.Second {
		t.Errorf("expected GetFlag to return at the deadline, took %v", elapsed)
	}
}

func TestGetFlag_CacheEvictsLeastRecentlyUsed(t *testing.T) {
	flag := &FlagStatus{Name: "f", Enabled: true, ID: "123"}
	client, transport := newCountingClient(t, flag, WithCache(time.Minute), WithCacheMaxEntries(2))
	get := func(key string) {
		t.Helper()
		if _, err := client.GetFlag(context.Background(), "123", UserContext{"targetingKey": key}); err != nil {
			t.Fatal(err)
		}
	}
	requests := func() int32 { return atomic.LoadInt32(&transport.count) }

	get("a")
	get("b")
	get("a") // hit, so "b" is now least recently used
	if n := requests(); n != 2 {
		t.Fatalf("expected 2 HTTP requests, got %d", n)
	}

	get("c") // evicts "b"
	get("a")
	if n := requests(); n != 3 {
		t.Fatalf("expected \"a\" to survive eviction, got %d HTTP requests", n)
	}

	get("b")
	if n := requests(); n != 4 {
		t.Fatalf("expected \"b\" to have been evicted, got %d HTTP requests", n)
	}
}

func TestGetFlag_CacheCapDefaultsTo10000(t *testing.T) {
	flag := &FlagStatus{Name: "f", Enabled: true, ID: "123"}
	for _, opts := range [][]ClientOption{
		{WithCache(time.Minute)},
		{WithCache(time.Minute), WithCacheMaxEntries(0)},
		{WithCache(time.Minute), WithCacheMaxEntries(-5)},
	} {
		client, transport := newCountingClient(t, flag, opts...)
		for i := 0; i <= DefaultCacheMaxEntries; i++ {
			if _, err := client.GetFlag(context.Background(), "123", UserContext{"targetingKey": fmt.Sprintf("user-%d", i)}); err != nil {
				t.Fatal(err)
			}
		}
		if _, err := client.GetFlag(context.Background(), "123", UserContext{"targetingKey": "user-10000"}); err != nil {
			t.Fatal(err)
		}
		if n := atomic.LoadInt32(&transport.count); n != DefaultCacheMaxEntries+1 {
			t.Fatalf("expected the newest entry to be cached, got %d HTTP requests", n)
		}
		if _, err := client.GetFlag(context.Background(), "123", UserContext{"targetingKey": "user-0"}); err != nil {
			t.Fatal(err)
		}
		if n := atomic.LoadInt32(&transport.count); n != DefaultCacheMaxEntries+2 {
			t.Fatalf("expected the oldest entry to be evicted, got %d HTTP requests", n)
		}
		if n := len(client.cache.entries); n != DefaultCacheMaxEntries {
			t.Errorf("expected %d cache entries, got %d", DefaultCacheMaxEntries, n)
		}
	}
}

func TestGetFlag_CacheConcurrentUse(t *testing.T) {
	flag := &FlagStatus{Name: "f", Enabled: true, ID: "123"}
	client, _ := newCountingClient(t, flag, WithCache(time.Minute), WithCacheMaxEntries(8))

	var wg sync.WaitGroup
	for g := 0; g < 8; g++ {
		wg.Add(1)
		go func(g int) {
			defer wg.Done()
			for i := 0; i < 200; i++ {
				key := fmt.Sprintf("user-%d", (g*i)%20)
				if _, err := client.GetFlag(context.Background(), "123", UserContext{"targetingKey": key}); err != nil {
					t.Error(err)
					return
				}
			}
		}(g)
	}
	wg.Wait()

	if n := len(client.cache.entries); n > 8 {
		t.Errorf("expected at most 8 cache entries, got %d", n)
	}
	if client.cache.order.Len() != len(client.cache.entries) {
		t.Errorf("cache list and map disagree: %d vs %d", client.cache.order.Len(), len(client.cache.entries))
	}
}

// RoundTripFunc is an adapter to allow the use of ordinary functions as RoundTrippers.
type RoundTripFunc func(req *http.Request) (*http.Response, error)

// RoundTrip implements the RoundTripper interface.
func (f RoundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}
