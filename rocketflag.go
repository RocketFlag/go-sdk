package rocketflag

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"sync"
	"time"
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
	version    string
	apiUrl     string
	client     *http.Client
	cache      *cache
	defaultTTL time.Duration
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

// WithCache enables in-memory response caching with the given default TTL.
// A non-positive duration leaves caching disabled.
func WithCache(ttl time.Duration) ClientOption {
	return func(c *Client) {
		c.defaultTTL = ttl
	}
}

// CallOption modifies behaviour of a single GetFlag call.
type CallOption func(*callOptions)

type callOptions struct {
	ttl    time.Duration
	ttlSet bool
}

// WithCallTTL overrides the cache TTL for a single GetFlag call. A zero or
// negative duration disables caching for that call even if a client default
// is configured.
func WithCallTTL(ttl time.Duration) CallOption {
	return func(o *callOptions) {
		o.ttl = ttl
		o.ttlSet = true
	}
}

// NewClient creates a new Client with optional configurations.
func NewClient(opts ...ClientOption) *Client {
	client := &Client{
		version: "v1",
		apiUrl:  "https://api.rocketflag.app",
		client:  http.DefaultClient,
	}

	for _, opt := range opts {
		opt(client)
	}

	client.cache = newCache()

	return client
}

// GetFlag retrieves a feature flag from the RocketFlag API.
func (c *Client) GetFlag(flagID string, userContext UserContext, opts ...CallOption) (*FlagStatus, error) {
	co := callOptions{}
	for _, opt := range opts {
		opt(&co)
	}

	ttl := c.defaultTTL
	if co.ttlSet {
		ttl = co.ttl
	}

	u, err := url.Parse(fmt.Sprintf("%s/%s/flags/%s", c.apiUrl, c.version, flagID))
	if err != nil {
		return nil, fmt.Errorf("error parsing URL: %w", err)
	}

	q := u.Query()
	for k, v := range userContext {
		q.Set(k, fmt.Sprintf("%v", v))
	}
	u.RawQuery = q.Encode()

	cacheActive := ttl > 0
	var key string
	if cacheActive {
		key = u.String()
		if cached, ok := c.cache.get(key); ok {
			return cached, nil
		}
	}

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

	if cacheActive {
		c.cache.set(key, flag, ttl)
	}

	return &flag, nil
}

type cacheEntry struct {
	flag      FlagStatus
	expiresAt time.Time
}

type cache struct {
	mu      sync.Mutex
	entries map[string]cacheEntry
}

func newCache() *cache {
	return &cache{entries: make(map[string]cacheEntry)}
}

func (c *cache) get(key string) (*FlagStatus, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	entry, ok := c.entries[key]
	if !ok {
		return nil, false
	}
	if time.Now().After(entry.expiresAt) {
		delete(c.entries, key)
		return nil, false
	}
	flag := entry.flag
	return &flag, true
}

func (c *cache) set(key string, flag FlagStatus, ttl time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.entries[key] = cacheEntry{flag: flag, expiresAt: time.Now().Add(ttl)}
}
