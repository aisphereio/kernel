# dtmx

`dtmx` is the kernel wrapper for DTM HTTP Saga orchestration.

It provides:

- `dtmx.Config` for configx/yaml integration.
- `dtmx.New` for a logx/errorx/metricsx aware HTTP client.
- `dtmx.NewSaga` and `SubmitSaga` for `/api/dtmsvr/submit`.
- `BranchURL` for building application callback URLs.

DTM remains an external service. The application must expose idempotent action
and compensation endpoints. Large payloads should not be stored in DTM payloads;
put large data in object storage first and pass only object keys/hashes in saga
payloads.

Example:

```go
dtm, _ := dtmx.New(dtmx.Config{
    Enabled: true,
    Server: "http://127.0.0.1:36789/api/dtmsvr",
    ServiceBaseURL: "http://127.0.0.1:18001",
    BranchPrefix: "/internal/dtm",
    WaitResult: true,
})

gid, _ := dtm.NewGID(ctx)
saga := dtmx.NewSaga(gid)
saga.WaitResult = true
saga.Add(dtm.BranchURL("/skill/package/promote"), dtm.BranchURL("/skill/package/promote_compensate"), payload)
saga.Add(dtm.BranchURL("/skill/metadata/upsert"), dtm.BranchURL("/skill/metadata/upsert_compensate"), payload)
err := dtm.SubmitSaga(ctx, saga)
```
