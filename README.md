<div align="center">

# go-uds-jsonrpc

**Tiny newline-delimited JSON-RPC over Unix domain sockets, for Go.**

[![Go Version](https://img.shields.io/github/go-mod/go-version/floatpane/go-uds-jsonrpc)](https://golang.org)
[![Go Reference](https://pkg.go.dev/badge/github.com/floatpane/go-uds-jsonrpc.svg)](https://pkg.go.dev/github.com/floatpane/go-uds-jsonrpc)
[![GitHub release (latest by date)](https://img.shields.io/github/v/release/floatpane/go-uds-jsonrpc)](https://github.com/floatpane/go-uds-jsonrpc/releases)
[![CI](https://github.com/floatpane/go-uds-jsonrpc/actions/workflows/ci.yml/badge.svg)](https://github.com/floatpane/go-uds-jsonrpc/actions/workflows/ci.yml)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)

</div>

`go-uds-jsonrpc` is what `net/rpc` should be in 2026: JSON on the wire, Unix-socket framing, server-pushed events, no gob, no reflection. It sits between `net/rpc` (too rigid) and gRPC (too heavy) for the very common case of *one daemon, several local clients, one machine*.

## Features

- **Three message shapes** — Request, Response, Event (server-pushed) — on one TCP-ish stream of `\n`-terminated JSON.
- **Cross-platform PID file + IsRunning** — Unix uses signal-0; Windows uses OpenProcess + GetExitCodeProcess.
- **XDG-aware socket paths** — `$XDG_RUNTIME_DIR/<app>/` on Linux, `~/Library/Caches/<app>/` on macOS, sensible fallback on others.
- **Server scaffolding** — handler registry, panic recovery, broadcast helpers, OnConnect / OnDisconnect hooks, context-driven shutdown.
- **Signal handler** — SIGTERM/SIGINT → shutdown, SIGHUP → reload, both wired in one call.
- **Zero dependencies.** stdlib-only.

## Install

```bash
go get github.com/floatpane/go-uds-jsonrpc
```

Requires Go 1.26+.

## Usage

### Server

```go
package main

import (
    "context"
    "encoding/json"
    "log"
    "net"
    "os"
    "os/signal"
    "syscall"

    udsrpc "github.com/floatpane/go-uds-jsonrpc"
)

func main() {
    const app = "myd"

    if err := udsrpc.EnsureRuntimeDir(app); err != nil {
        log.Fatal(err)
    }
    if pid, running := udsrpc.IsRunning(udsrpc.PIDPath(app)); running {
        log.Fatalf("already running (PID %d)", pid)
    }
    if err := udsrpc.WritePID(udsrpc.PIDPath(app)); err != nil {
        log.Fatal(err)
    }
    defer udsrpc.RemovePID(udsrpc.PIDPath(app))

    _ = os.Remove(udsrpc.SocketPath(app))
    l, err := net.Listen("unix", udsrpc.SocketPath(app))
    if err != nil {
        log.Fatal(err)
    }
    defer l.Close()

    s := udsrpc.NewServer()
    s.Handle("Ping", func(_ context.Context, _ *udsrpc.Conn, _ json.RawMessage) (any, error) {
        return map[string]bool{"pong": true}, nil
    })

    ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
    defer cancel()
    log.Println(s.Serve(ctx, l))
}
```

### Client

```go
package main

import (
    "encoding/json"
    "fmt"
    "log"
    "net"

    udsrpc "github.com/floatpane/go-uds-jsonrpc"
)

func main() {
    conn, err := net.Dial("unix", udsrpc.SocketPath("myd"))
    if err != nil {
        log.Fatal(err)
    }
    c := udsrpc.NewConn(conn)
    defer c.Close()

    if err := c.Send(&udsrpc.Request{ID: 1, Method: "Ping"}); err != nil {
        log.Fatal(err)
    }
    msg, err := c.ReceiveMessage()
    if err != nil {
        log.Fatal(err)
    }
    var result map[string]bool
    json.Unmarshal(msg.Response.Result, &result)
    fmt.Println("pong:", result["pong"])
}
```

### Push events from server to all clients

```go
go func() {
    for range time.Tick(5 * time.Second) {
        s.Broadcast("Tick", map[string]int64{"unix": time.Now().Unix()})
    }
}()
```

Clients receive these as `Event` messages when they call `ReceiveMessage()`.

## Wire format

Every message is one JSON object followed by `\n`:

```
{"id":1,"method":"Ping","params":{}}
{"id":1,"result":{"pong":true}}
{"type":"Tick","data":{"unix":1748000000}}
```

`DecodeMessage` discriminates by inspecting the keys:

- has `"type"`  → `Event`
- has `"method"` → `Request`
- otherwise → `Response`

Standard error codes (borrowed from JSON-RPC 2.0):

| Code     | Constant              | Meaning                  |
|----------|-----------------------|--------------------------|
| `-32700` | `ErrCodeParse`        | Invalid JSON received    |
| `-32600` | `ErrCodeInvalidReq`   | Not a valid Request      |
| `-32601` | `ErrCodeNotFound`     | Method not registered    |
| `-32602` | `ErrCodeInvalidParams`| Method exists, bad params|
| `-32603` | `ErrCodeInternal`     | Handler error            |

Return an `*Error` from a handler to forward a specific code/message to the client. Any other non-nil error becomes `ErrCodeInternal` with the message.

## Documentation

Full API reference: [pkg.go.dev/github.com/floatpane/go-uds-jsonrpc](https://pkg.go.dev/github.com/floatpane/go-uds-jsonrpc)

## Contributing

PRs welcome. See [CONTRIBUTING.md](CONTRIBUTING.md).

## Security

Report vulnerabilities privately via [SECURITY.md](SECURITY.md).

## License

MIT. See [LICENSE](LICENSE).
