# RocketFlag Go SDK

This SDK provides a convenient way to interact with the RocketFlag API from your Go applications.

## Installation

```bash
go get github.com/rocketflag/go-sdk
```

## Basic Usage

```go
package main

import (
	"context"
	"fmt"
	"log"

	rocketflag "github.com/rocketflag/go-sdk"
)

func main() {
	rf := rocketflag.NewClient()

	// Example: Get a single flag
	flag, err := rf.GetFlag("flag-id", rocketflag.UserContext{})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("Flag:", flag)
}
```

## Including a Cohort

If you want to use a cohort and only enable flags for certain users, you'll need to setup the accepted cohorts in the console. Once done,
you can pass the cohort like so with the SDK:

```go
flag, err := rf.GetFlag("flag-id", rocketflag.UserContext{"cohort": "user@example.com"})
```

## Overrides

### HTTP Client

You can pass in http clients if you use them or have custom ones. Eg:

```go
import (
	"net/http"
)

customHttpClient := &http.Client{}

client := rocketflag.NewClient(WithHTTPClient(customHttpClient))
```

### Custom version

By default, RocketFlag will use the latest API version that the SDK knows about. Right now, this is version 1. You can override this if you
prefer as so:

```go
client := rocketflag.NewClient(WithVersion("v2"))
```

### Custom URL

By default, RocketFlag will use the RocketFlag API. This is https://api.rocketflag.app. You can override this if you prefer as so:

```go
client := rocketflag.NewClient(WithAPIURL("https://api.example.com"))
```

### Caching responses

To avoid hitting the API on every check, you can enable an in-memory cache by
providing a default TTL via either `WithCacheSeconds` or `WithCacheMinutes`
(not both — combining them returns an error from the next `GetFlag` call).
Cached entries are keyed by flag ID **and** `UserContext`, so different
cohorts/users still resolve independently.

```go
client := rocketflag.NewClient(rocketflag.WithCacheMinutes(5))

// First call hits the API; subsequent calls within 5 minutes are served from cache.
flag, err := client.GetFlag("flag-id", rocketflag.UserContext{"cohort": "beta"})
```

You can override the TTL for a single call, or disable caching for that call by
passing `0`:

```go
// Force a fresh fetch, bypassing the cache.
flag, err := client.GetFlag("flag-id", nil, rocketflag.WithCallSeconds(0))

// Use a shorter TTL just for this call.
flag, err := client.GetFlag("flag-id", nil, rocketflag.WithCallSeconds(10))
```

Caching is opt-in — without `WithCacheSeconds`/`WithCacheMinutes` or a per-call
override, every call goes to the API.

### Chaining custom client options

```go
client := rocketflag.NewClient(
  WithHTTPClient(customHttpClient),
  WithVersion("v2"),
  WithAPIURL("https://api.example.com")
)
```
