# Forbidden Patterns

## Removed package

```go
import "github.com/aisphereio/kernel/errors"
```

Forbidden. Use:

```go
import "github.com/aisphereio/kernel/errorx"
```

## Raw business errors

Forbidden for API/business boundaries:

```go
return errors.New("skill not found")
return fmt.Errorf("skill %s not found", id)
```

Use:

```go
return errorx.NotFound(
    "AIHUB_SKILL_NOT_FOUND",
    "skill not found",
    errorx.WithMetadata("skill_id", id),
)
```

## Dynamic error codes

Forbidden:

```go
return errorx.NotFound(errorx.Code("SKILL_"+id+"_NOT_FOUND"), "skill not found")
```

Codes must be stable and low-cardinality.
