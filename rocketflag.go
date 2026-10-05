package rocketflag

import (
	"container/list"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"sync"
	"time"
)

// DefaultCacheMaxEntries is the most responses the cache holds unless
// WithCacheMaxEntries sets another limit.
const DefaultCacheMaxEntries = 10000

// FlagStatus represents the status of a feature flag.
type FlagStatus struct {
	Name    string `json:"name"`
	Enabled bool   `json:"enabled"`
	ID      string `json:"id"`
}

// UserContext is the evaluation context for a flag request. Every entry is sent
// to the API as a query parameter:
//
//   - "cohort" is matched against the flag's cohort list.
//   - "env" selects an environment of a group flag.
//   - "targetingKey" is a stable identifier for the user (a user id, not an
//     email where you can avoid it) that makes percentage rollouts sticky.
//   - Any other key is an audience attribute, such as "plan" or "country".
type UserContext map[string]string

// Client is a RocketFlag API client.
type Client struct {
	version    string
	apiUrl     string
	client     *http.Client
	cache      *cache
	defaultTTL time.Duration
	maxEntries int
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

// WithCacheMaxEntries sets the most responses the cache holds. When it is full
// the least recently used entry is evicted. A non-positive value leaves the
// default of DefaultCacheMaxEntries in place.
func WithCacheMaxEntries(n int) ClientOption {
	return func(c *Client) {
		if n > 0 {
			c.maxEntries = n
		}
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
		version:    "v1",
		apiUrl:     "https://api.rocketflag.app",
		client:     http.DefaultClient,
		maxEntries: DefaultCacheMaxEntries,
	}

	for _, opt := range opts {
		opt(client)
	}

	client.cache = newCache(client.maxEntries)

	return client
}

// GetFlag retrieves a feature flag from the RocketFlag API. The request is
// bound to ctx, so cancelling ctx or passing its deadline aborts it.
func (c *Client) GetFlag(ctx context.Context, flagID string, userContext UserContext, opts ...CallOption) (*FlagStatus, error) {
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
		q.Set(k, v)
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

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
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
	key       string
	flag      FlagStatus
	expiresAt time.Time
}

// cache is a TTL cache capped at maxEntries, evicting the least recently used
// entry when full. The front of order is the most recently used entry.
type cache struct {
	mu         sync.Mutex
	maxEntries int
	order      *list.List
	entries    map[string]*list.Element
}

func newCache(maxEntries int) *cache {
	return &cache{
		maxEntries: maxEntries,
		order:      list.New(),
		entries:    make(map[string]*list.Element),
	}
}

func (c *cache) get(key string) (*FlagStatus, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	el, ok := c.entries[key]
	if !ok {
		return nil, false
	}
	entry := el.Value.(*cacheEntry)
	if time.Now().After(entry.expiresAt) {
		c.order.Remove(el)
		delete(c.entries, key)
		return nil, false
	}
	c.order.MoveToFront(el)
	flag := entry.flag
	return &flag, true
}

func (c *cache) set(key string, flag FlagStatus, ttl time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	expiresAt := time.Now().Add(ttl)
	if el, ok := c.entries[key]; ok {
		entry := el.Value.(*cacheEntry)
		entry.flag = flag
		entry.expiresAt = expiresAt
		c.order.MoveToFront(el)
		return
	}
	if c.order.Len() >= c.maxEntries {
		oldest := c.order.Back()
		c.order.Remove(oldest)
		delete(c.entries, oldest.Value.(*cacheEntry).key)
	}
	c.entries[key] = c.order.PushFront(&cacheEntry{key: key, flag: flag, expiresAt: expiresAt})
}
