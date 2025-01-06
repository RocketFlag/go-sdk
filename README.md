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

	"github.com/rocketflag/go-sdk"
)

func main() {
	client, err := rocketflag.NewClient()
	if err != nil {
		log.Fatal(err)
	}

	ctx := context.Background()

	// Example: Get a single flag
	flag, err := client.GetFlag(ctx, "my-flag")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("Flag:", flag)
}
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
