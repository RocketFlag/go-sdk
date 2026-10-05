# RocketFlag Go SDK

This SDK provides a convenient way to interact with the RocketFlag API from your Go applications.

## Installation

```bash
go get github.com/rocketflag/go-sdk/v2
```

Upgrading from v1? See [Migrating from v1](#migrating-from-v1).

## Basic Usage

```go
package main

import (
	"context"
	"fmt"
	"log"

	rocketflag "github.com/rocketflag/go-sdk/v2"
)

func main() {
	rf := rocketflag.NewClient()

	flag, err := rf.GetFlag(context.Background(), "flag-id", nil)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("Flag:", flag)
}
```

`GetFlag` binds the request to the context you pass, so cancelling it or
passing its deadline aborts the request. In a request handler, pass the
handler's context (`r.Context()`); elsewhere, set a timeout:

```go
ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
defer cancel()

flag, err := rf.GetFlag(ctx, "flag-id", nil)
```

## Including a Cohort

If you want to use a cohort and only enable flags for certain users, you'll need to setup the accepted cohorts in the console. Once done,
you can pass the cohort like so with the SDK:

```go
flag, err := rf.GetFlag(ctx, "flag-id", rocketflag.UserContext{"cohort": "user@example.com"})
```

## Sticky rollouts and audiences

Pass a `targetingKey` to make percentage rollouts sticky, and any other keys as
audience attributes:

```go
flag, err := rf.GetFlag(ctx, "flag-id", rocketflag.UserContext{
	"targetingKey": user.ID,
	"plan":         "pro",
	"country":      "AU",
})
```

- **`targetingKey`**: a stable identifier for the user. The same key always
  gets the same answer from a percentage rollout, in every environment of a
  group flag, and raising the percentage only ever adds users. Without a
  `targetingKey` the `cohort` is used, and with neither each request is a fresh
  random roll. Prefer an opaque id over an email address: the key is part of
  the request URL.
- **Any other key** is an audience attribute, matched against the flag's
  audience exactly and case-sensitively. An attribute you don't send never
  matches. `cohort`, `env` and `targetingKey` are reserved and can't be
  audience attributes.

`UserContext` is a `map[string]string`, matching the query string it becomes.
Convert other values yourself, for example `strconv.Itoa(seats)`.

## Group flags (environments)

Select the environment of a group flag with `env`:

```go
flag, err := rf.GetFlag(ctx, "flag-id", rocketflag.UserContext{"env": "production"})
```

## Overrides

### HTTP Client

You can pass in http clients if you use them or have custom ones. Eg:

```go
import (
	"net/http"
)

customHttpClient := &http.Client{}

client := rocketflag.NewClient(rocketflag.WithHTTPClient(customHttpClient))
```

### Custom version

By default, RocketFlag will use the latest API version that the SDK knows about. Right now, this is version 1. You can override this if you
prefer as so:

```go
client := rocketflag.NewClient(rocketflag.WithVersion("v2"))
```

### Custom URL

By default, RocketFlag will use the RocketFlag API. This is https://api.rocketflag.app. You can override this if you prefer as so:

```go
client := rocketflag.NewClient(rocketflag.WithAPIURL("https://api.example.com"))
```

### Caching responses

To avoid hitting the API on every check, you can enable an in-memory cache by
providing a default TTL via `WithCache`. Cached entries are keyed by flag ID
**and** `UserContext`, so different cohorts/users still resolve independently.

```go
client := rocketflag.NewClient(rocketflag.WithCache(5 * time.Minute))

// First call hits the API; subsequent calls within 5 minutes are served from cache.
flag, err := client.GetFlag(ctx, "flag-id", rocketflag.UserContext{"cohort": "beta"})
```

You can override the TTL for a single call, or disable caching for that call by
passing `0`:

```go
// Force a fresh fetch, bypassing the cache.
flag, err := client.GetFlag(ctx, "flag-id", nil, rocketflag.WithCallTTL(0))

// Use a shorter TTL just for this call.
flag, err := client.GetFlag(ctx, "flag-id", nil, rocketflag.WithCallTTL(10*time.Second))
```

Caching is opt-in: without `WithCache` or a per-call override, every call goes
to the API. Each distinct `UserContext` is its own cache entry, so a
`targetingKey` per user means an entry per user. The cache holds at most 10,000
entries and evicts the least recently used one when it is full. Change the cap
with `WithCacheMaxEntries`:

```go
client := rocketflag.NewClient(
	rocketflag.WithCache(5*time.Minute),
	rocketflag.WithCacheMaxEntries(50000),
)
```

### Chaining custom client options

```go
client := rocketflag.NewClient(
	rocketflag.WithHTTPClient(customHttpClient),
	rocketflag.WithVersion("v2"),
	rocketflag.WithAPIURL("https://api.example.com"),
)
```

## Migrating from v1

v2 changes three things:

1. **Import path.** Import `github.com/rocketflag/go-sdk/v2` instead of
   `github.com/rocketflag/go-sdk`.
2. **`GetFlag` takes a `context.Context` first.**
   `rf.GetFlag("flag-id", userContext)` becomes
   `rf.GetFlag(ctx, "flag-id", userContext)`.
3. **`UserContext` is a `map[string]string`.** v1 accepted any value and
   formatted it with `%v`, so a nil or a struct was sent as `<nil>` or
   `{...}` and silently never matched. Convert numbers and booleans yourself:
   `UserContext{"cohort": strconv.FormatBool(beta)}`.

The cache is also capped at 10,000 entries by default (see
[Caching responses](#caching-responses)). v1 remains available at its existing
versions.
