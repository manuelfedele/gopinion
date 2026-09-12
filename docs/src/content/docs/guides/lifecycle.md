---
title: Server Lifecycle
description: Let GOpinion own startup, timeouts, registration, and graceful shutdown.
sidebar:
  order: 6
---

GOpinion owns the HTTP server so endpoint code cannot accidentally bypass its
policy chain.

## Register before running

```go
app, err := gopinion.New(
    gopinion.WithAuthenticator(authenticator),
    gopinion.WithAuthorizer(authorizer),
)
if err != nil {
    return err
}

if err := app.Register(gopinion.AuthorizedGet("/profile", prepareProfile, profile)); err != nil {
    return err
}
if err := app.Register(gopinion.AuthorizedList("/events", prepareEventList, listEvents)); err != nil {
    return err
}

return app.Run(ctx)
```

`Run` may be called once. Route registration closes as soon as it starts.
It serves HTTP, so production deployments must terminate TLS at a trusted load
balancer, ingress, gateway, or service mesh before forwarding requests.

## Connect shutdown to signals

```go
ctx, stop := signal.NotifyContext(
    context.Background(),
    os.Interrupt,
    syscall.SIGTERM,
)
defer stop()

if err := app.Run(ctx); err != nil {
    log.Fatal(err)
}
```

When the context is cancelled, GOpinion stops accepting connections and waits
for active requests up to `shutdown_timeout`. If that deadline expires, it
force-closes active connections.

## Bound every connection phase

```yaml
server:
  address: ":8080"
  read_header_timeout: 5s
  read_timeout: 15s
  write_timeout: 30s
  idle_timeout: 60s
  shutdown_timeout: 10s
  max_body_bytes: 1048576
```

All duration values must be positive Go duration strings.

| Setting | Protects |
| --- | --- |
| `read_header_timeout` | Slow or incomplete request headers |
| `read_timeout` | Slow complete request uploads |
| `write_timeout` | Slow response consumption |
| `idle_timeout` | Idle keep-alive connections |
| `shutdown_timeout` | Deployment shutdown deadlines |
| `max_body_bytes` | Memory and parsing work per request |

## Preserve cancellation

Use the request context for database and downstream work:

```go
func getSummary(ctx gopinion.Context) (Summary, error) {
    return summaries.Load(ctx.Request().Context(), ctx.Principal().Subject)
}
```

This propagates client disconnects and upstream deadlines. During shutdown,
active request contexts remain valid for the configured grace period and are
cancelled if the server must force-close their connections.
