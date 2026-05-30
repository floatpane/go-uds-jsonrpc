# Security Policy

## Supported Versions

Only the latest release of go-uds-jsonrpc is supported with security updates.

## Reporting a Vulnerability

If you discover a security vulnerability in go-uds-jsonrpc, please report it responsibly. **Do not open a public issue.**

Email us at [us@floatpane.com](mailto:us@floatpane.com) with:

- A description of the vulnerability
- Steps to reproduce the issue
- The potential impact
- Any suggested fixes (optional)

We will acknowledge your report within 48 hours and aim to provide a fix or mitigation plan within 7 days, depending on severity.

## Scope

This policy covers the go-uds-jsonrpc codebase and its official releases.

Of particular interest:

- Crafted JSON that produces panics, runaway allocations, or stack overflows in `DecodeMessage` or `Conn.ReceiveMessage`.
- Race conditions in `Server` broadcast / handler dispatch under concurrent connections.
- Socket-path TOCTOU between `EnsureRuntimeDir`, `WritePID`, and `net.Listen("unix", …)`.
- PID-file races (`IsRunning` returning true for a recycled PID).
- Unbounded resource use in the accept loop (file descriptors, goroutines per stalled client).

Third-party dependencies are outside our direct control, but we will work to address reported issues in them as quickly as possible.

## Disclosure

We ask that you give us reasonable time to address the issue before disclosing it publicly. We are committed to crediting reporters in release notes (unless you prefer to remain anonymous).
