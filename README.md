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

### Chaining custom client options

```go
client := rocketflag.NewClient(
  WithHTTPClient(customHttpClient),
  WithVersion("v2"),
  WithAPIURL("https://api.example.com")
)
```
