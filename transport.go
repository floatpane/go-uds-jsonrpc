package udsrpc

import (
	"encoding/json"
	"fmt"
	"net"
	"sync"
)

// Conn wraps a net.Conn with newline-delimited JSON encoding/decoding.
// Send is serialized by an internal mutex so concurrent goroutines can
// push to the same connection without interleaving bytes; ReceiveMessage
// is not safe for concurrent use and is intended to be driven by a
// single reader goroutine.
type Conn struct {
	conn net.Conn
	enc  *json.Encoder
	dec  *json.Decoder
	mu   sync.Mutex // serializes writes
}

// NewConn wraps an existing net.Conn.
func NewConn(c net.Conn) *Conn {
	return &Conn{
		conn: c,
		enc:  json.NewEncoder(c),
		dec:  json.NewDecoder(c),
	}
}

// Send writes a JSON-encoded value followed by a newline. Safe to call
// from multiple goroutines.
func (c *Conn) Send(v interface{}) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.enc.Encode(v)
}

// SendResponse sends a successful response with the given result.
// The result is marshaled with encoding/json.
func (c *Conn) SendResponse(id uint64, result interface{}) error {
	raw, err := json.Marshal(result)
	if err != nil {
		return fmt.Errorf("marshal result: %w", err)
	}
	return c.Send(&Response{
		ID:     id,
		Result: raw,
	})
}

// SendError sends an error response with the given code and message.
func (c *Conn) SendError(id uint64, code int, message string) error {
	return c.Send(&Response{
		ID:    id,
		Error: &Error{Code: code, Message: message},
	})
}

// SendEvent pushes a server-originated event with no Request to ack.
func (c *Conn) SendEvent(eventType string, data interface{}) error {
	raw, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("marshal event data: %w", err)
	}
	return c.Send(&Event{
		Type: eventType,
		Data: raw,
	})
}

// ReceiveMessage reads and decodes the next JSON message, returning a
// discriminated Message. Blocks until a full JSON object is read or the
// underlying connection closes.
func (c *Conn) ReceiveMessage() (Message, error) {
	var raw json.RawMessage
	if err := c.dec.Decode(&raw); err != nil {
		return Message{}, err
	}
	return DecodeMessage(raw)
}

// Close closes the underlying connection.
func (c *Conn) Close() error {
	return c.conn.Close()
}

// RemoteAddr returns the remote address of the underlying connection.
func (c *Conn) RemoteAddr() net.Addr {
	return c.conn.RemoteAddr()
}

// LocalAddr returns the local address of the underlying connection.
func (c *Conn) LocalAddr() net.Addr {
	return c.conn.LocalAddr()
}
