package udsrpc

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func newUDSServer(t *testing.T) (*Server, net.Listener, string, func()) {
	t.Helper()
	dir := t.TempDir()
	sock := filepath.Join(dir, "s")
	l, err := net.Listen("unix", sock)
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	s := NewServer()
	s.Logger = log.New(io.Discard, "", 0)
	cleanup := func() { l.Close() }
	return s, l, sock, cleanup
}

func dialConn(t *testing.T, sock string) *Conn {
	t.Helper()
	c, err := net.Dial("unix", sock)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	return NewConn(c)
}

func TestServer_HandleRequestResponse(t *testing.T) {
	s, l, sock, cleanup := newUDSServer(t)
	defer cleanup()

	s.Handle("Echo", func(_ context.Context, _ *Conn, params json.RawMessage) (any, error) {
		var p struct {
			Msg string `json:"msg"`
		}
		if err := json.Unmarshal(params, &p); err != nil {
			return nil, err
		}
		return map[string]string{"echo": p.Msg}, nil
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go s.Serve(ctx, l)
	c := dialConn(t, sock)
	defer c.Close()

	params, _ := json.Marshal(map[string]string{"msg": "hello"})
	if err := c.Send(&Request{ID: 7, Method: "Echo", Params: params}); err != nil {
		t.Fatal(err)
	}

	msg, err := c.ReceiveMessage()
	if err != nil {
		t.Fatal(err)
	}
	if msg.Response == nil {
		t.Fatal("expected Response")
	}
	if msg.Response.ID != 7 {
		t.Errorf("id = %d, want 7", msg.Response.ID)
	}
	var out map[string]string
	if err := json.Unmarshal(msg.Response.Result, &out); err != nil {
		t.Fatal(err)
	}
	if out["echo"] != "hello" {
		t.Errorf("echo = %q, want hello", out["echo"])
	}
}

func TestServer_MethodNotFound(t *testing.T) {
	s, l, sock, cleanup := newUDSServer(t)
	defer cleanup()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go s.Serve(ctx, l)
	c := dialConn(t, sock)
	defer c.Close()

	if err := c.Send(&Request{ID: 1, Method: "Nope"}); err != nil {
		t.Fatal(err)
	}
	msg, err := c.ReceiveMessage()
	if err != nil {
		t.Fatal(err)
	}
	if msg.Response == nil || msg.Response.Error == nil {
		t.Fatal("expected error response")
	}
	if msg.Response.Error.Code != ErrCodeNotFound {
		t.Errorf("code = %d, want %d", msg.Response.Error.Code, ErrCodeNotFound)
	}
}

func TestServer_HandlerErrorForwardsCode(t *testing.T) {
	s, l, sock, cleanup := newUDSServer(t)
	defer cleanup()

	s.Handle("Boom", func(_ context.Context, _ *Conn, _ json.RawMessage) (any, error) {
		return nil, &Error{Code: ErrCodeInvalidParams, Message: "bad params"}
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go s.Serve(ctx, l)
	c := dialConn(t, sock)
	defer c.Close()

	if err := c.Send(&Request{ID: 1, Method: "Boom"}); err != nil {
		t.Fatal(err)
	}
	msg, err := c.ReceiveMessage()
	if err != nil {
		t.Fatal(err)
	}
	if msg.Response == nil || msg.Response.Error == nil {
		t.Fatal("expected error")
	}
	if msg.Response.Error.Code != ErrCodeInvalidParams {
		t.Errorf("code = %d, want %d", msg.Response.Error.Code, ErrCodeInvalidParams)
	}
	if msg.Response.Error.Message != "bad params" {
		t.Errorf("message = %q", msg.Response.Error.Message)
	}
}

func TestServer_GenericErrorBecomesInternal(t *testing.T) {
	s, l, sock, cleanup := newUDSServer(t)
	defer cleanup()

	s.Handle("Boom", func(_ context.Context, _ *Conn, _ json.RawMessage) (any, error) {
		return nil, errors.New("kaboom")
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go s.Serve(ctx, l)
	c := dialConn(t, sock)
	defer c.Close()

	if err := c.Send(&Request{ID: 1, Method: "Boom"}); err != nil {
		t.Fatal(err)
	}
	msg, err := c.ReceiveMessage()
	if err != nil {
		t.Fatal(err)
	}
	if msg.Response.Error.Code != ErrCodeInternal {
		t.Errorf("code = %d, want %d", msg.Response.Error.Code, ErrCodeInternal)
	}
	if msg.Response.Error.Message != "kaboom" {
		t.Errorf("message = %q", msg.Response.Error.Message)
	}
}

func TestServer_BroadcastReachesAllClients(t *testing.T) {
	s, l, sock, cleanup := newUDSServer(t)
	defer cleanup()

	var ready sync.WaitGroup
	ready.Add(2)
	s.OnConnect(func(_ *Conn) { ready.Done() })

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go s.Serve(ctx, l)
	a := dialConn(t, sock)
	defer a.Close()
	b := dialConn(t, sock)
	defer b.Close()
	ready.Wait()

	s.Broadcast("Tick", map[string]int{"n": 1})

	for _, c := range []*Conn{a, b} {
		msg, err := c.ReceiveMessage()
		if err != nil {
			t.Fatal(err)
		}
		if msg.Event == nil || msg.Event.Type != "Tick" {
			t.Fatalf("got %+v, want Tick event", msg)
		}
	}
}

func TestServer_DisconnectHookFires(t *testing.T) {
	s, l, sock, cleanup := newUDSServer(t)
	defer cleanup()

	var got atomic.Int32
	s.OnDisconnect(func(_ *Conn) { got.Add(1) })

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go s.Serve(ctx, l)
	c := dialConn(t, sock)
	c.Close()

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if got.Load() == 1 {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("disconnect hook not called")
}

func TestServer_CancelCtxStopsServe(t *testing.T) {
	s, l, _, cleanup := newUDSServer(t)
	defer cleanup()

	ctx, cancel := context.WithCancel(context.Background())
	errCh := make(chan error, 1)
	go func() { errCh <- s.Serve(ctx, l) }()

	cancel()
	select {
	case err := <-errCh:
		if err != nil {
			t.Fatalf("Serve returned %v, want nil after ctx cancel", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Serve did not return after ctx cancel")
	}
}
