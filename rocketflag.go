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
	version      string
	apiUrl       string
	client       *http.Client
	cache        *cache
	defaultTTL   time.Duration
	cacheSeconds *int
	cacheMinutes *int
	configErr    error
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

// WithCacheSeconds enables in-memory response caching with the given default
// TTL in seconds. Cannot be combined with WithCacheMinutes.
func WithCacheSeconds(seconds int) ClientOption {
	return func(c *Client) {
		c.cacheSeconds = &seconds
	}
}

// WithCacheMinutes enables in-memory response caching with the given default
// TTL in minutes. Cannot be combined with WithCacheSeconds.
func WithCacheMinutes(minutes int) ClientOption {
	return func(c *Client) {
		c.cacheMinutes = &minutes
	}
}

// CallOption modifies behaviour of a single GetFlag call.
type CallOption func(*callOptions)

type callOptions struct {
	seconds *int
	minutes *int
}

// WithCallSeconds overrides the cache TTL for a single GetFlag call in
// seconds. A value of 0 disables caching for that call even if a default is
// configured. Cannot be combined with WithCallMinutes.
func WithCallSeconds(seconds int) CallOption {
	return func(o *callOptions) {
		o.seconds = &seconds
	}
}

// WithCallMinutes overrides the cache TTL for a single GetFlag call in
// minutes. A value of 0 disables caching for that call. Cannot be combined
// with WithCallSeconds.
func WithCallMinutes(minutes int) CallOption {
	return func(o *callOptions) {
		o.minutes = &minutes
	}
}

// NewClient creates a new Client with optional configurations. Configuration
// errors (such as conflicting cache options) are deferred and surfaced from
// GetFlag.
func NewClient(opts ...ClientOption) *Client {
	client := &Client{
		version: "v1",
		apiUrl:  "https://api.rocketflag.app",
		client:  http.DefaultClient,
	}

	for _, opt := range opts {
		opt(client)
	}

	if client.cacheSeconds != nil && client.cacheMinutes != nil {
		client.configErr = fmt.Errorf("client cache options cannot specify both WithCacheSeconds and WithCacheMinutes")
		return client
	}
	if client.cacheSeconds != nil {
		client.defaultTTL = time.Duration(*client.cacheSeconds) * time.Second
	} else if client.cacheMinutes != nil {
		client.defaultTTL = time.Duration(*client.cacheMinutes) * time.Minute
	}
	client.cache = newCache()

	return client
}

// GetFlag retrieves a feature flag from the RocketFlag API.
func (c *Client) GetFlag(flagID string, userContext UserContext, opts ...CallOption) (*FlagStatus, error) {
	if c.configErr != nil {
		return nil, c.configErr
	}

	co := callOptions{}
	for _, opt := range opts {
		opt(&co)
	}
	if co.seconds != nil && co.minutes != nil {
		return nil, fmt.Errorf("call cache options cannot specify both WithCallSeconds and WithCallMinutes")
	}

	ttl := c.defaultTTL
	if co.seconds != nil {
		ttl = time.Duration(*co.seconds) * time.Second
	} else if co.minutes != nil {
		ttl = time.Duration(*co.minutes) * time.Minute
	}

	u, err := url.Parse(fmt.Sprintf("%s/%s/flags/%s", c.apiUrl, c.version, flagID))
	if err != nil {
		return nil, fmt.Errorf("error parsing URL: %w", err)
	}

	q := u.Query()
	for k, v := range userContext {
		q.Set(k, fmt.Sprintf("%v", v))
	}
	encoded := q.Encode()
	u.RawQuery = encoded

	cacheActive := c.cache != nil && ttl > 0
	var key string
	if cacheActive {
		key = flagID + "?" + encoded
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
		c.cache.set(key, &flag, ttl)
	}

	return &flag, nil
}

type cacheEntry struct {
	flag      *FlagStatus
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
	return entry.flag, true
}

func (c *cache) set(key string, flag *FlagStatus, ttl time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.entries[key] = cacheEntry{flag: flag, expiresAt: time.Now().Add(ttl)}
}
