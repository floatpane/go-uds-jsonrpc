// Package udsrpc is a tiny newline-delimited JSON-RPC implementation
// designed for daemon ↔ client communication over Unix domain sockets.
//
// It sits between net/rpc (too rigid, gob-only) and full gRPC (too heavy).
// Messages are JSON objects separated by newlines; the wire format is
// language-agnostic, debuggable by hand, and trivial to bridge to any
// other runtime.
//
// Three message shapes share the wire:
//
//   - Request:  {"id": N, "method": "...", "params": {...}}
//   - Response: {"id": N, "result": {...}}  or  {"id": N, "error": {...}}
//   - Event:    {"type": "...", "data": {...}}     (server → client push)
//
// DecodeMessage discriminates between the three by inspecting the keys
// present in the raw JSON: "type" → Event, "method" → Request, else
// Response.
package udsrpc

import "encoding/json"

// Request from client to server. The ID is echoed in the matching
// Response so callers can correlate concurrent in-flight requests.
type Request struct {
	ID     uint64          `json:"id"`
	Method string          `json:"method"`
	Params json.RawMessage `json:"params,omitempty"`
}

// Response from server to client. Either Result or Error is populated,
// never both.
type Response struct {
	ID     uint64          `json:"id"`
	Result json.RawMessage `json:"result,omitempty"`
	Error  *Error          `json:"error,omitempty"`
}

// Event is a server-pushed message with no Request. It has no ID and
// is not acknowledged.
type Event struct {
	Type string          `json:"type"`
	Data json.RawMessage `json:"data,omitempty"`
}

// Error is the error payload inside a Response.
type Error struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// Error implements the error interface so an *Error can be returned
// from handlers directly.
func (e *Error) Error() string { return e.Message }

// Message is a discriminated union for wire decoding. Exactly one of
// Request, Response, or Event is non-nil after DecodeMessage succeeds.
type Message struct {
	Request  *Request
	Response *Response
	Event    *Event
}

// DecodeMessage inspects raw and returns a discriminated Message.
//
// Discriminator order: "type" present → Event, "method" present →
// Request, else → Response. This matches how peers should produce
// messages — never include both "type" and "method", never set "id"
// on Events.
func DecodeMessage(raw json.RawMessage) (Message, error) {
	var probe struct {
		Type   string  `json:"type"`
		Method string  `json:"method"`
		ID     *uint64 `json:"id"`
	}
	if err := json.Unmarshal(raw, &probe); err != nil {
		return Message{}, err
	}

	var m Message
	switch {
	case probe.Type != "":
		var ev Event
		if err := json.Unmarshal(raw, &ev); err != nil {
			return m, err
		}
		m.Event = &ev
	case probe.Method != "":
		var req Request
		if err := json.Unmarshal(raw, &req); err != nil {
			return m, err
		}
		m.Request = &req
	default:
		var resp Response
		if err := json.Unmarshal(raw, &resp); err != nil {
			return m, err
		}
		m.Response = &resp
	}
	return m, nil
}

// Standard error codes, borrowed from JSON-RPC 2.0 so existing
// tooling and client libraries recognize them.
const (
	// ErrCodeParse — invalid JSON was received.
	ErrCodeParse = -32700
	// ErrCodeInvalidReq — request was not a valid Request object.
	ErrCodeInvalidReq = -32600
	// ErrCodeNotFound — method does not exist or is not registered.
	ErrCodeNotFound = -32601
	// ErrCodeInvalidParams — method exists but params are invalid.
	ErrCodeInvalidParams = -32602
	// ErrCodeInternal — internal server error.
	ErrCodeInternal = -32603
)
