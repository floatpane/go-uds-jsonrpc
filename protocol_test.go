package udsrpc

import (
	"encoding/json"
	"testing"
)

func TestDecodeMessage_Request(t *testing.T) {
	raw := json.RawMessage(`{"id":1,"method":"Ping"}`)
	msg, err := DecodeMessage(raw)
	if err != nil {
		t.Fatal(err)
	}
	if msg.Request == nil {
		t.Fatal("expected Request, got nil")
	}
	if msg.Request.Method != "Ping" {
		t.Errorf("method = %q, want Ping", msg.Request.Method)
	}
	if msg.Request.ID != 1 {
		t.Errorf("id = %d, want 1", msg.Request.ID)
	}
	if msg.Response != nil || msg.Event != nil {
		t.Error("expected only Request to be set")
	}
}

func TestDecodeMessage_Response(t *testing.T) {
	raw := json.RawMessage(`{"id":1,"result":{"pong":true}}`)
	msg, err := DecodeMessage(raw)
	if err != nil {
		t.Fatal(err)
	}
	if msg.Response == nil {
		t.Fatal("expected Response, got nil")
	}
	if msg.Response.ID != 1 {
		t.Errorf("id = %d, want 1", msg.Response.ID)
	}
	if msg.Response.Error != nil {
		t.Error("expected no error")
	}
}

func TestDecodeMessage_ResponseError(t *testing.T) {
	raw := json.RawMessage(`{"id":2,"error":{"code":-32601,"message":"not found"}}`)
	msg, err := DecodeMessage(raw)
	if err != nil {
		t.Fatal(err)
	}
	if msg.Response == nil {
		t.Fatal("expected Response")
	}
	if msg.Response.Error == nil {
		t.Fatal("expected error in response")
	}
	if msg.Response.Error.Code != ErrCodeNotFound {
		t.Errorf("code = %d, want %d", msg.Response.Error.Code, ErrCodeNotFound)
	}
	if msg.Response.Error.Message != "not found" {
		t.Errorf("message = %q, want 'not found'", msg.Response.Error.Message)
	}
}

func TestDecodeMessage_Event(t *testing.T) {
	raw := json.RawMessage(`{"type":"NewItem","data":{"id":"abc"}}`)
	msg, err := DecodeMessage(raw)
	if err != nil {
		t.Fatal(err)
	}
	if msg.Event == nil {
		t.Fatal("expected Event, got nil")
	}
	if msg.Event.Type != "NewItem" {
		t.Errorf("type = %q, want NewItem", msg.Event.Type)
	}
	var data struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(msg.Event.Data, &data); err != nil {
		t.Fatal(err)
	}
	if data.ID != "abc" {
		t.Errorf("id = %q, want abc", data.ID)
	}
}

func TestDecodeMessage_Invalid(t *testing.T) {
	raw := json.RawMessage(`{invalid}`)
	if _, err := DecodeMessage(raw); err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestError_ErrorInterface(t *testing.T) {
	e := &Error{Code: ErrCodeInternal, Message: "something broke"}
	if e.Error() != "something broke" {
		t.Errorf("Error() = %q, want 'something broke'", e.Error())
	}
}

func TestRequestRoundTrip(t *testing.T) {
	type params struct {
		Limit int `json:"limit"`
	}
	raw, _ := json.Marshal(params{Limit: 50})
	req := Request{ID: 42, Method: "Fetch", Params: raw}

	data, err := json.Marshal(req)
	if err != nil {
		t.Fatal(err)
	}

	msg, err := DecodeMessage(data)
	if err != nil {
		t.Fatal(err)
	}
	if msg.Request == nil {
		t.Fatal("expected Request")
	}
	if msg.Request.ID != 42 {
		t.Errorf("id = %d, want 42", msg.Request.ID)
	}
	var p params
	if err := json.Unmarshal(msg.Request.Params, &p); err != nil {
		t.Fatal(err)
	}
	if p.Limit != 50 {
		t.Errorf("limit = %d, want 50", p.Limit)
	}
}
