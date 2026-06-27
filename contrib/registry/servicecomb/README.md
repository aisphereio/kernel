# Servicecomb Registry

## example

### server
```go
package main

import (
	"log"

	"github.com/go-chassis/sc-client"
	"github.com/aisphereio/kernel/contrib/registry/servicecomb/v3"
	"github.com/aisphereio/kernel"
)

func main() {
	c, err := sc.NewClient(sc.Options{
		Endpoints: []string{"127.0.0.1:30100"},
	})
	if err != nil {
		log.Panic(err)
	}
	r := servicecomb.NewRegistry(c)
	app := kernel.New(
		kernel.Name("helloServicecomb"),
		kernel.Registrar(r),
	)
	if err := app.Run(); err != nil {
		log.Fatal(err)
	}
}
```

### client
```go
package main

import (
	"context"
	"log"

	"github.com/go-chassis/sc-client"
	"github.com/aisphereio/kernel/contrib/registry/servicecomb/v3"
	"github.com/aisphereio/kernel/transport/grpc"
)

func main() {
	c, err := sc.NewClient(sc.Options{
		Endpoints: []string{"127.0.0.1:30100"},
	})
	if err != nil {
		log.Panic(err)
	}
	r := servicecomb.NewRegistry(c)
	ctx := context.Background()
	conn, err := grpc.NewClient(
		ctx,
		grpc.WithEndpoint("discovery:///helloServicecomb"),
		grpc.WithDiscovery(r),
	)
	if err != nil {
		return
	}
	defer conn.Close()
}
```
