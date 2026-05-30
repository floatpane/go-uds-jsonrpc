package udsrpc

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"runtime/debug"
	"sync"
)

// HandlerFunc handles a single Request. The returned value is marshaled
// as the Response.Result. To return a custom error code, return an
// *Error (the server forwards Code and Message verbatim). Any other
// non-nil error is sent as ErrCodeInternal with err.Error() as the
// message.
type HandlerFunc func(ctx context.Context, conn *Conn, params json.RawMessage) (any, error)

// Server multiplexes incoming Requests on accepted connections to
// registered HandlerFuncs and tracks all live clients so events can be
// broadcast.
//
// A zero-value Server is not usable — call NewServer.
type Server struct {
	handlerMu sync.RWMutex
	handlers  map[string]HandlerFunc

	clientMu sync.RWMutex
	clients  map[*Conn]struct{}

	hookMu       sync.RWMutex
	onConnect    func(*Conn)
	onDisconnect func(*Conn)

	// Logger receives accept errors and recovered panics. Defaults to
	// log.Default(); set to a no-op logger to silence the server.
	Logger *log.Logger
}

// NewServer returns a Server with no handlers registered.
func NewServer() *Server {
	return &Server{
		handlers: make(map[string]HandlerFunc),
		clients:  make(map[*Conn]struct{}),
		Logger:   log.Default(),
	}
}

// Handle registers fn as the handler for method. Calling Handle with
// the same method twice replaces the previous handler.
func (s *Server) Handle(method string, fn HandlerFunc) {
	s.handlerMu.Lock()
	defer s.handlerMu.Unlock()
	s.handlers[method] = fn
}

// OnConnect installs a hook called once per accepted connection,
// before any messages are read. The hook runs synchronously on the
// accept goroutine; expensive work should be dispatched elsewhere.
func (s *Server) OnConnect(fn func(*Conn)) {
	s.hookMu.Lock()
	defer s.hookMu.Unlock()
	s.onConnect = fn
}

// OnDisconnect installs a hook called once per connection when it
// closes (cleanly or after error).
func (s *Server) OnDisconnect(fn func(*Conn)) {
	s.hookMu.Lock()
	defer s.hookMu.Unlock()
	s.onDisconnect = fn
}

// Clients returns a snapshot slice of all currently-connected clients.
// Safe to call concurrently with Serve.
func (s *Server) Clients() []*Conn {
	s.clientMu.RLock()
	defer s.clientMu.RUnlock()
	out := make([]*Conn, 0, len(s.clients))
	for c := range s.clients {
		out = append(out, c)
	}
	return out
}

// Broadcast sends an Event to every connected client. Errors per
// connection are logged but do not abort the broadcast.
func (s *Server) Broadcast(eventType string, data any) {
	s.BroadcastFunc(eventType, data, nil)
}

// BroadcastFunc sends an Event to every connected client for which
// predicate returns true. A nil predicate matches all clients.
func (s *Server) BroadcastFunc(eventType string, data any, predicate func(*Conn) bool) {
	for _, c := range s.Clients() {
		if predicate != nil && !predicate(c) {
			continue
		}
		if err := c.SendEvent(eventType, data); err != nil {
			s.logf("broadcast %q to %s: %v", eventType, c.RemoteAddr(), err)
		}
	}
}

// Serve accepts connections on l and dispatches their Requests to
// registered handlers until ctx is canceled or l.Close is called.
//
// On ctx cancel, Serve closes l (so the blocked Accept returns) and
// returns nil. Other accept errors are returned to the caller.
func (s *Server) Serve(ctx context.Context, l net.Listener) error {
	// Close the listener when ctx cancels so the Accept loop unblocks.
	go func() {
		<-ctx.Done()
		l.Close() //nolint:errcheck
	}()

	for {
		conn, err := l.Accept()
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			if errors.Is(err, net.ErrClosed) {
				return nil
			}
			return fmt.Errorf("accept: %w", err)
		}

		c := NewConn(conn)
		s.addClient(c)
		go s.serveConn(ctx, c)
	}
}

func (s *Server) serveConn(ctx context.Context, c *Conn) {
	defer func() {
		if r := recover(); r != nil {
			s.logf("connection %s panic: %v\n%s", c.RemoteAddr(), r, debug.Stack())
		}
		s.removeClient(c)
		c.Close() //nolint:errcheck
		s.hookMu.RLock()
		fn := s.onDisconnect
		s.hookMu.RUnlock()
		if fn != nil {
			fn(c)
		}
	}()

	s.hookMu.RLock()
	onConn := s.onConnect
	s.hookMu.RUnlock()
	if onConn != nil {
		onConn(c)
	}

	for {
		msg, err := c.ReceiveMessage()
		if err != nil {
			if errors.Is(err, io.EOF) || errors.Is(err, net.ErrClosed) {
				return
			}
			// Send a parse error response only if we can identify the request id —
			// which we cannot here. Just close.
			s.logf("receive from %s: %v", c.RemoteAddr(), err)
			return
		}

		if msg.Request == nil {
			// Server ignores Responses and Events from clients.
			continue
		}

		req := msg.Request
		go s.dispatch(ctx, c, req)
	}
}

func (s *Server) dispatch(ctx context.Context, c *Conn, req *Request) {
	defer func() {
		if r := recover(); r != nil {
			s.logf("handler %q panic: %v\n%s", req.Method, r, debug.Stack())
			_ = c.SendError(req.ID, ErrCodeInternal, "handler panic")
		}
	}()

	s.handlerMu.RLock()
	fn, ok := s.handlers[req.Method]
	s.handlerMu.RUnlock()
	if !ok {
		_ = c.SendError(req.ID, ErrCodeNotFound, "method not found: "+req.Method)
		return
	}

	result, err := fn(ctx, c, req.Params)
	if err != nil {
		var e *Error
		if errors.As(err, &e) {
			_ = c.SendError(req.ID, e.Code, e.Message)
			return
		}
		_ = c.SendError(req.ID, ErrCodeInternal, err.Error())
		return
	}
	_ = c.SendResponse(req.ID, result)
}

func (s *Server) addClient(c *Conn) {
	s.clientMu.Lock()
	s.clients[c] = struct{}{}
	s.clientMu.Unlock()
}

func (s *Server) removeClient(c *Conn) {
	s.clientMu.Lock()
	delete(s.clients, c)
	s.clientMu.Unlock()
}

func (s *Server) logf(format string, args ...any) {
	if s.Logger == nil {
		return
	}
	s.Logger.Printf(format, args...)
}
