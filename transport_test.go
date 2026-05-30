package udsrpc

import (
	"encoding/json"
	"net"
	"testing"
)

func testPipe() (*Conn, *Conn) {
	a, b := net.Pipe()
	return NewConn(a), NewConn(b)
}

func TestConn_SendReceiveRequest(t *testing.T) {
	client, server := testPipe()
	defer client.Close()
	defer server.Close()

	done := make(chan error, 1)
	go func() {
		params, _ := json.Marshal(map[string]bool{"pong": true})
		done <- client.Send(&Request{ID: 1, Method: "Ping", Params: params})
	}()

	msg, err := server.ReceiveMessage()
	if err != nil {
		t.Fatal(err)
	}
	if msg.Request == nil {
		t.Fatal("expected Request")
	}
	if msg.Request.Method != "Ping" {
		t.Errorf("method = %q, want Ping", msg.Request.Method)
	}
	if err := <-done; err != nil {
		t.Fatal(err)
	}
}

func TestConn_SendResponse(t *testing.T) {
	client, server := testPipe()
	defer client.Close()
	defer server.Close()

	done := make(chan error, 1)
	go func() {
		done <- server.SendResponse(1, map[string]bool{"pong": true})
	}()

	msg, err := client.ReceiveMessage()
	if err != nil {
		t.Fatal(err)
	}
	if msg.Response == nil {
		t.Fatal("expected Response")
	}
	if msg.Response.ID != 1 {
		t.Errorf("id = %d, want 1", msg.Response.ID)
	}
	var result map[string]bool
	if err := json.Unmarshal(msg.Response.Result, &result); err != nil {
		t.Fatal(err)
	}
	if !result["pong"] {
		t.Error("expected pong=true")
	}
	if err := <-done; err != nil {
		t.Fatal(err)
	}
}

func TestConn_SendError(t *testing.T) {
	client, server := testPipe()
	defer client.Close()
	defer server.Close()

	done := make(chan error, 1)
	go func() {
		done <- server.SendError(5, ErrCodeNotFound, "method not found")
	}()

	msg, err := client.ReceiveMessage()
	if err != nil {
		t.Fatal(err)
	}
	if msg.Response == nil || msg.Response.Error == nil {
		t.Fatal("expected error response")
	}
	if msg.Response.Error.Code != ErrCodeNotFound {
		t.Errorf("code = %d, want %d", msg.Response.Error.Code, ErrCodeNotFound)
	}
	if err := <-done; err != nil {
		t.Fatal(err)
	}
}

func TestConn_SendEvent(t *testing.T) {
	client, server := testPipe()
	defer client.Close()
	defer server.Close()

	done := make(chan error, 1)
	go func() {
		done <- server.SendEvent("Hello", map[string]string{"name": "world"})
	}()

	msg, err := client.ReceiveMessage()
	if err != nil {
		t.Fatal(err)
	}
	if msg.Event == nil {
		t.Fatal("expected Event")
	}
	if msg.Event.Type != "Hello" {
		t.Errorf("type = %q, want Hello", msg.Event.Type)
	}
	var data map[string]string
	if err := json.Unmarshal(msg.Event.Data, &data); err != nil {
		t.Fatal(err)
	}
	if data["name"] != "world" {
		t.Errorf("name = %q, want world", data["name"])
	}
	if err := <-done; err != nil {
		t.Fatal(err)
	}
}

func TestConn_MultipleMessages(t *testing.T) {
	client, server := testPipe()
	defer client.Close()
	defer server.Close()

	go func() {
		client.Send(&Request{ID: 1, Method: "A"}) //nolint:errcheck
		client.Send(&Request{ID: 2, Method: "B"}) //nolint:errcheck
	}()

	msg1, err := server.ReceiveMessage()
	if err != nil {
		t.Fatal(err)
	}
	if msg1.Request.ID != 1 {
		t.Errorf("first id = %d, want 1", msg1.Request.ID)
	}
	msg2, err := server.ReceiveMessage()
	if err != nil {
		t.Fatal(err)
	}
	if msg2.Request.ID != 2 {
		t.Errorf("second id = %d, want 2", msg2.Request.ID)
	}
}
