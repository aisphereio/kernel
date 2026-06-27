# Polaris Config

```go
import (
	"log"

	"github.com/polarismesh/polaris-go"

	"github.com/aisphereio/kernel/contrib/config/polaris/v3"
)

func main() {
    configApi, err := polaris.NewConfigAPI()
    if err != nil {
    	log.Fatalln(err)
    }

    source, err := New(&configApi, WithNamespace("default"), WithFileGroup("default"), WithFileName("default.yaml"))
    if err != nil {
    	log.Fatalln(err)
    }
    source.Load()
}
```
