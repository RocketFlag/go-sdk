package rocketflag_test

import (
	"context"
	"fmt"
	"log"
	"time"

	rocketflag "github.com/rocketflag/go-sdk/v2"
)

func ExampleClient_GetFlag() {
	rf := rocketflag.NewClient(rocketflag.WithCache(5 * time.Minute))

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	flag, err := rf.GetFlag(ctx, "flag-id", rocketflag.UserContext{
		"targetingKey": "user-42",
		"plan":         "pro",
		"country":      "AU",
	})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(flag.Enabled)
}
