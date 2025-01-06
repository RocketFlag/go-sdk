package rocketflag

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"reflect"
	"strings"
	"testing"
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
	flag, err := client.GetFlag("123", UserContext{"cohort": "beta"})

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
	_, err := client.GetFlag("123", nil)

	// Assertions
	if err == nil {
		t.Fatal("Expected an error, got nil")
	}

	if !strings.Contains(err.Error(), "error parsing URL") {
		t.Errorf("Expected error message to contain 'error parsing URL', got: %v", err)
	}
}

func TestGetFlag_ErrorCreatingRequest(t *testing.T) {
	// Create a client with a custom Transport that forces an error in NewRequest.
	client := NewClient()
	client.client = &http.Client{
		Transport: &errorTransport{},
	}

	// Call the function
	_, err := client.GetFlag("123", nil)

	// Assertions
	if err == nil {
		t.Fatal("Expected an error, got nil")
	}
	if !strings.Contains(err.Error(), "error creating request") {
		t.Errorf("Expected error message to contain 'error creating request', got: %v", err)
	}
}

// errorTransport is a custom RoundTripper that forces an error during request creation.
type errorTransport struct{}

func (t *errorTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	// Attempt to create a new request with an invalid URL, which will cause an error.
	_, err := http.NewRequest(req.Method, "\n", req.Body) // Invalid URL
	if err != nil {
		return nil, fmt.Errorf("error creating request: %w", err)
	}
	return nil, errors.New("unexpected: RoundTrip should not reach this point")
}

func TestGetFlag_ErrorMakingRequest(t *testing.T) {
	// Mock error
	mockError := errors.New("mock network error")

	// Create a client with the mock HTTP client that returns an error
	client := NewClient(WithHTTPClient(MockClient(nil, mockError)))

	// Call the function
	_, err := client.GetFlag("123", nil)

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
	_, err := client.GetFlag("123", nil)

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
	_, err := client.GetFlag("123", nil)

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
	userContext := UserContext{"cohort": "beta", "id": 123, "active": true}
	_, err := client.GetFlag("123", userContext)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	// Assertions
	if capturedRequest == nil {
		t.Fatal("Request was not captured")
	}

	expectedQuery := url.Values{}
	for k, v := range userContext {
		expectedQuery.Set(k, fmt.Sprintf("%v", v))
	}
	actualQuery := capturedRequest.URL.Query()

	if !reflect.DeepEqual(actualQuery, expectedQuery) {
		t.Errorf("Expected query: %+v, got: %+v", expectedQuery, actualQuery)
	}
}

// RoundTripFunc is an adapter to allow the use of ordinary functions as RoundTrippers.
type RoundTripFunc func(req *http.Request) (*http.Response, error)

// RoundTrip implements the RoundTripper interface.
func (f RoundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}
