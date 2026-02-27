package channels

import (
	"testing"

	"github.com/mimiclaw/mimiclaw/pkg/bus"
	"github.com/mimiclaw/mimiclaw/pkg/config"
)

func newTestWorkerWS(t *testing.T, cfg config.WorkerWSConfig) *WorkerWSChannel {
	t.Helper()
	ch, err := NewWorkerWSChannel(cfg, bus.NewMessageBus())
	if err != nil {
		t.Fatalf("NewWorkerWSChannel() error: %v", err)
	}
	return ch
}

func TestWorkerWSValidateConfig(t *testing.T) {
	ch := newTestWorkerWS(t, config.WorkerWSConfig{
		Address: "ws://127.0.0.1:3001/ws",
		Role:    "employee",
		Name:    "Researcher-1",
		Tags:    []string{"research"},
	})

	if err := ch.validateConfig(); err != nil {
		t.Fatalf("validateConfig() error: %v", err)
	}
}

func TestWorkerWSValidateConfigRequiredFields(t *testing.T) {
	tests := []config.WorkerWSConfig{
		{Role: "employee", Name: "n", Tags: []string{"t"}},
		{Address: "ws://127.0.0.1:3001/ws", Name: "n", Tags: []string{"t"}},
		{Address: "ws://127.0.0.1:3001/ws", Role: "employee", Tags: []string{"t"}},
		{Address: "ws://127.0.0.1:3001/ws", Role: "employee", Name: "n", Tags: []string{}},
	}

	for i, tc := range tests {
		ch := newTestWorkerWS(t, tc)
		if err := ch.validateConfig(); err == nil {
			t.Fatalf("case %d: expected validateConfig error", i)
		}
	}
}

func TestWorkerWSResolveTarget(t *testing.T) {
	ch := newTestWorkerWS(t, config.WorkerWSConfig{})

	if got := ch.resolveTarget(""); got != "*" {
		t.Fatalf("resolveTarget(\"\") = %#v, want \"*\"", got)
	}
	if got := ch.resolveTarget("boss-1"); got != "boss-1" {
		t.Fatalf("resolveTarget(single) = %#v", got)
	}
	if got := ch.resolveTarget("all"); got != "all" {
		t.Fatalf("resolveTarget(all) = %#v", got)
	}
	got := ch.resolveTarget("boss-1,employee-2")
	targets, ok := got.([]string)
	if !ok || len(targets) != 2 || targets[0] != "boss-1" || targets[1] != "employee-2" {
		t.Fatalf("resolveTarget(list) = %#v", got)
	}
}

func TestWorkerWSBuildPayload(t *testing.T) {
	ch := newTestWorkerWS(t, config.WorkerWSConfig{})

	raw := ch.buildPayload(`{"task":"analyze"}`)
	obj, ok := raw.(map[string]any)
	if !ok || obj["task"] != "analyze" {
		t.Fatalf("buildPayload(json) = %#v", raw)
	}

	text := ch.buildPayload("hello")
	obj, ok = text.(map[string]any)
	if !ok || obj["content"] != "hello" {
		t.Fatalf("buildPayload(text) = %#v", text)
	}
}

func TestWorkerWSBuildAuthMessageIncludesIdentity(t *testing.T) {
	ch := newTestWorkerWS(t, config.WorkerWSConfig{
		Role: "employee",
		Name: "Researcher-1",
		Tags: []string{"research"},
	})
	ch.identity = workerWSIdentity{ID: "employee-1", Key: "secret"}

	msg := ch.buildAuthMessage()
	identity, ok := msg["identity"].(map[string]any)
	if !ok {
		t.Fatalf("auth message missing identity: %#v", msg)
	}
	if identity["id"] != "employee-1" || identity["key"] != "secret" {
		t.Fatalf("auth identity = %#v", identity)
	}
}
