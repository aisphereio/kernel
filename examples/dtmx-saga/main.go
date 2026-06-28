package main

import (
	"context"
	"fmt"
	"time"

	"github.com/aisphereio/kernel/dtmx"
	_ "github.com/aisphereio/kernel/dtmx/dtm"
)

func main() {
	manager, err := dtmx.New(dtmx.Config{
		Enabled:        false, // Set true when a DTM server is running.
		Driver:         "dtm",
		Server:         "http://127.0.0.1:36789/api/dtmsvr",
		ServiceBaseURL: "http://127.0.0.1:18001",
		BranchPrefix:   "/internal/dtm",
		WaitResult:     true,
		Timeout:        10 * time.Second,
	})
	if err != nil {
		panic(err)
	}
	defer manager.Close()

	if !manager.Enabled() {
		fmt.Println("dtmx disabled")
		return
	}

	ctx := context.Background()
	gid, err := manager.NewGID(ctx)
	if err != nil {
		panic(err)
	}
	saga := dtmx.NewSaga(gid, "example").
		AddHTTP("step1", manager.BranchURL("example/action"), manager.BranchURL("example/compensate"), map[string]any{"hello": "world"})

	_, err = manager.SubmitSaga(ctx, saga)
	if err != nil {
		panic(err)
	}
	fmt.Println("submitted", gid)
}
