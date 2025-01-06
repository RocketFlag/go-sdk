package rocketflag

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
)

// FlagStatus represents the status of a feature flag.
type FlagStatus struct {
	Name    string `json:"name"`
	Enabled bool   `json:"enabled"`
	ID      string `json:"id"`
}

// UserContext allows for passing user-specific context to the API, such as a cohort.
type UserContext map[string]interface{}

// Client is a RocketFlag API client.
type Client struct {
	version string
	apiUrl  string
	client  *http.Client
}

// ClientOption defines a function type that modifies the Client.
type ClientOption func(*Client)

// WithVersion sets the version for the Client.
func WithVersion(version string) ClientOption {
	return func(c *Client) {
		c.version = version
	}
}

// WithAPIURL sets the API URL for the Client.
func WithAPIURL(apiUrl string) ClientOption {
	return func(c *Client) {
		c.apiUrl = apiUrl
	}
}

// WithHTTPClient sets a custom HTTP client for the Client.
func WithHTTPClient(client *http.Client) ClientOption {
	return func(c *Client) {
		c.client = client
	}
}

// NewClient creates a new Client with optional configurations.
func NewClient(opts ...ClientOption) *Client {
	// Default values
	client := &Client{
		version: "v1",
		apiUrl:  "https://api.rocketflag.app",
		client:  http.DefaultClient,
	}

	// Apply functional options
	for _, opt := range opts {
		opt(client)
	}

	return client
}

// GetFlag retrieves a feature flag from the RocketFlag API.
func (c *Client) GetFlag(flagID string, userContext UserContext) (*FlagStatus, error) {

	u, err := url.Parse(fmt.Sprintf("%s/%s/flags/%s", c.apiUrl, c.version, flagID))
	if err != nil {
		return nil, fmt.Errorf("error parsing URL: %w", err)
	}

	q := u.Query()
	for k, v := range userContext {
		q.Set(k, fmt.Sprintf("%v", v))
	}
	u.RawQuery = q.Encode()

	req, err := http.NewRequest(http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("error creating request: %w", err)
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error making request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("error from server: %s", resp.Status)
	}

	var flag FlagStatus
	if err := json.NewDecoder(resp.Body).Decode(&flag); err != nil {
		return nil, fmt.Errorf("error decoding response: %w", err)
	}

	return &flag, nil
}
