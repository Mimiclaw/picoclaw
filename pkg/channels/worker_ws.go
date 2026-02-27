package channels

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"

	"github.com/mimiclaw/mimiclaw/pkg/bus"
	"github.com/mimiclaw/mimiclaw/pkg/config"
	"github.com/mimiclaw/mimiclaw/pkg/logger"
)

const (
	workerWSDefaultIdentityFile = "~/.mimiclaw/worker_ws_identity.json"
	workerWSAuthTimeout         = 15 * time.Second
)

type workerWSIdentity struct {
	ID  string `json:"id"`
	Key string `json:"key"`
}

type WorkerWSChannel struct {
	*BaseChannel
	config config.WorkerWSConfig

	ctx    context.Context
	cancel context.CancelFunc

	mu       sync.RWMutex
	writeMu  sync.Mutex
	conn     *websocket.Conn
	identity workerWSIdentity
}

func NewWorkerWSChannel(cfg config.WorkerWSConfig, messageBus *bus.MessageBus) (*WorkerWSChannel, error) {
	base := NewBaseChannel("worker_ws", cfg, messageBus, cfg.AllowFrom)
	c := &WorkerWSChannel{
		BaseChannel: base,
		config:      cfg,
	}
	_ = c.loadIdentity()
	return c, nil
}

func (c *WorkerWSChannel) Start(ctx context.Context) error {
	if err := c.validateConfig(); err != nil {
		return err
	}

	c.ctx, c.cancel = context.WithCancel(ctx)

	if err := c.connectAndAuthenticate(); err != nil {
		if c.reconnectInterval() <= 0 {
			return err
		}
		logger.WarnCF("worker_ws", "Initial connect/auth failed; reconnect loop will retry", map[string]any{
			"error": err.Error(),
		})
	}

	if c.reconnectInterval() > 0 {
		go c.reconnectLoop()
	}
	if c.pingInterval() > 0 {
		go c.pingLoop()
	}

	c.setRunning(true)
	logger.InfoCF("worker_ws", "Worker WS channel started", map[string]any{
		"address": c.config.Address,
		"role":    c.config.Role,
		"name":    c.config.Name,
		"tags":    c.cleanTags(),
	})
	return nil
}

func (c *WorkerWSChannel) Stop(ctx context.Context) error {
	c.setRunning(false)
	if c.cancel != nil {
		c.cancel()
	}

	c.mu.Lock()
	if c.conn != nil {
		_ = c.conn.Close()
		c.conn = nil
	}
	c.mu.Unlock()

	logger.InfoC("worker_ws", "Worker WS channel stopped")
	return nil
}

func (c *WorkerWSChannel) Send(ctx context.Context, msg bus.OutboundMessage) error {
	if !c.IsRunning() {
		return fmt.Errorf("worker_ws channel not running")
	}

	req := map[string]any{
		"type":   "message",
		"to":     c.resolveTarget(msg.ChatID),
		"payload": c.buildPayload(msg.Content),
		"msg_id": c.nextMsgID(),
	}
	return c.sendJSON(req)
}

func (c *WorkerWSChannel) validateConfig() error {
	if strings.TrimSpace(c.config.Address) == "" {
		return fmt.Errorf("worker_ws.address is required")
	}
	if strings.TrimSpace(c.config.Role) == "" {
		return fmt.Errorf("worker_ws.role is required")
	}
	if c.config.Role != "employee" && c.config.Role != "boss" {
		return fmt.Errorf("worker_ws.role must be boss or employee")
	}
	if strings.TrimSpace(c.config.Name) == "" {
		return fmt.Errorf("worker_ws.name is required")
	}
	if len(c.cleanTags()) == 0 {
		return fmt.Errorf("worker_ws.tags requires at least one non-empty tag")
	}
	return nil
}

func (c *WorkerWSChannel) connectAndAuthenticate() error {
	dialer := websocket.DefaultDialer
	dialer.HandshakeTimeout = 10 * time.Second

	conn, _, err := dialer.Dial(c.config.Address, nil)
	if err != nil {
		return fmt.Errorf("dial worker ws failed: %w", err)
	}

	if err := c.awaitHello(conn); err != nil {
		_ = conn.Close()
		return err
	}

	if err := c.writeJSONConn(conn, c.buildAuthMessage()); err != nil {
		_ = conn.Close()
		return fmt.Errorf("send auth failed: %w", err)
	}

	authOK, err := c.awaitAuthOK(conn)
	if err != nil {
		_ = conn.Close()
		return err
	}

	c.mu.Lock()
	if c.conn != nil {
		_ = c.conn.Close()
	}
	c.conn = conn
	c.identity = workerWSIdentity{ID: authOK.ID, Key: authOK.Key}
	c.mu.Unlock()

	if err := c.saveIdentity(); err != nil {
		logger.WarnCF("worker_ws", "Failed to persist identity", map[string]any{"error": err.Error()})
	}

	logger.InfoCF("worker_ws", "Authenticated to worker ws", map[string]any{
		"id":          authOK.ID,
		"reconnected": authOK.Reconnected,
		"role":        authOK.Role,
	})

	go c.readLoop(conn)
	return nil
}

func (c *WorkerWSChannel) awaitHello(conn *websocket.Conn) error {
	_ = conn.SetReadDeadline(time.Now().Add(workerWSAuthTimeout))
	defer conn.SetReadDeadline(time.Time{})

	_, data, err := conn.ReadMessage()
	if err != nil {
		return fmt.Errorf("waiting hello failed: %w", err)
	}

	var msg struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal(data, &msg); err != nil {
		return fmt.Errorf("hello is not valid json: %w", err)
	}
	if msg.Type != "hello" {
		return fmt.Errorf("expected hello, got %q", msg.Type)
	}
	return nil
}

type workerWSAuthOK struct {
	Type        string `json:"type"`
	ID          string `json:"id"`
	Key         string `json:"key"`
	Role        string `json:"role"`
	Reconnected bool   `json:"reconnected"`
}

func (c *WorkerWSChannel) awaitAuthOK(conn *websocket.Conn) (*workerWSAuthOK, error) {
	_ = conn.SetReadDeadline(time.Now().Add(workerWSAuthTimeout))
	defer conn.SetReadDeadline(time.Time{})

	for {
		_, data, err := conn.ReadMessage()
		if err != nil {
			return nil, fmt.Errorf("waiting auth_ok failed: %w", err)
		}

		var base struct {
			Type    string `json:"type"`
			Code    string `json:"code"`
			Message string `json:"message"`
		}
		if err := json.Unmarshal(data, &base); err != nil {
			return nil, fmt.Errorf("auth response invalid json: %w", err)
		}

		switch base.Type {
		case "auth_ok":
			var ok workerWSAuthOK
			if err := json.Unmarshal(data, &ok); err != nil {
				return nil, fmt.Errorf("parse auth_ok failed: %w", err)
			}
			if ok.ID == "" || ok.Key == "" {
				return nil, fmt.Errorf("auth_ok missing id/key")
			}
			return &ok, nil
		case "error":
			if base.Code == "" {
				base.Code = "auth_failed"
			}
			return nil, fmt.Errorf("auth rejected: %s (%s)", base.Message, base.Code)
		default:
			// Ignore unrelated packets during auth window.
		}
	}
}

func (c *WorkerWSChannel) buildAuthMessage() map[string]any {
	msg := map[string]any{
		"type": "auth",
		"role": c.config.Role,
		"name": c.config.Name,
		"tags": c.cleanTags(),
		"meta": map[string]any{
			"client":    "mimiclaw",
			"timestamp": time.Now().UnixMilli(),
		},
	}
	if strings.TrimSpace(c.config.AuthKey) != "" {
		msg["authkey"] = c.config.AuthKey
	}

	c.mu.RLock()
	id := c.identity.ID
	key := c.identity.Key
	c.mu.RUnlock()
	if id != "" && key != "" {
		msg["identity"] = map[string]any{
			"id":  id,
			"key": key,
		}
	}

	return msg
}

func (c *WorkerWSChannel) readLoop(conn *websocket.Conn) {
	for {
		select {
		case <-c.ctx.Done():
			return
		default:
		}

		_, data, err := conn.ReadMessage()
		if err != nil {
			if c.ctx.Err() == nil {
				logger.WarnCF("worker_ws", "Read loop ended", map[string]any{"error": err.Error()})
			}
			c.clearConnIfCurrent(conn)
			return
		}

		c.handleServerMessage(data)
	}
}

func (c *WorkerWSChannel) handleServerMessage(data []byte) {
	var msg map[string]any
	if err := json.Unmarshal(data, &msg); err != nil {
		logger.WarnCF("worker_ws", "Ignoring invalid json packet", map[string]any{"error": err.Error()})
		return
	}

	msgType, _ := msg["type"].(string)
	switch msgType {
	case "pong", "health_report_ack":
		logger.DebugCF("worker_ws", "Heartbeat packet received", map[string]any{"type": msgType})
	case "delivery_ack":
		logger.InfoCF("worker_ws", "Delivery ack", map[string]any{
			"msg_id": msg["msg_id"],
		})
	case "error":
		logger.WarnCF("worker_ws", "Server error", map[string]any{
			"code":    msg["code"],
			"message": msg["message"],
		})
	case "system":
		logger.WarnCF("worker_ws", "System event", map[string]any{
			"event": msg["event"],
			"by":    msg["by"],
		})
	case "message":
		c.handleInboundRoutedMessage(msg)
	case "auth_ok":
		if id := asString(msg["id"]); id != "" {
			key := asString(msg["key"])
			c.mu.Lock()
			c.identity = workerWSIdentity{ID: id, Key: key}
			c.mu.Unlock()
			_ = c.saveIdentity()
		}
	default:
		logger.DebugCF("worker_ws", "Ignoring unsupported packet type", map[string]any{"type": msgType})
	}
}

func (c *WorkerWSChannel) handleInboundRoutedMessage(msg map[string]any) {
	fromID := asString(msg["from_id"])
	if fromID == "" {
		fromID = "worker_ws"
	}
	if !c.IsAllowed(fromID) {
		return
	}

	content := payloadToContent(msg["payload"])
	if content == "" {
		content = "{}"
	}

	metadata := map[string]string{}
	if v := asString(msg["msg_id"]); v != "" {
		metadata["msg_id"] = v
	}
	if v := asString(msg["from_role"]); v != "" {
		metadata["from_role"] = v
	}
	if v := asString(msg["to"]); v != "" {
		metadata["to"] = v
	}
	if ts := asString(msg["timestamp"]); ts != "" {
		metadata["timestamp"] = ts
	}

	c.HandleMessage(fromID, fromID, content, nil, metadata)
}

func (c *WorkerWSChannel) pingLoop() {
	ticker := time.NewTicker(c.pingInterval())
	defer ticker.Stop()

	for {
		select {
		case <-c.ctx.Done():
			return
		case <-ticker.C:
			if c.getConn() == nil {
				continue
			}
			if err := c.sendJSON(map[string]any{"type": "ping"}); err != nil && c.ctx.Err() == nil {
				logger.WarnCF("worker_ws", "Ping failed", map[string]any{"error": err.Error()})
			}
		}
	}
}

func (c *WorkerWSChannel) reconnectLoop() {
	ticker := time.NewTicker(c.reconnectInterval())
	defer ticker.Stop()

	for {
		select {
		case <-c.ctx.Done():
			return
		case <-ticker.C:
			if c.getConn() != nil {
				continue
			}
			logger.InfoC("worker_ws", "Attempting reconnect")
			if err := c.connectAndAuthenticate(); err != nil {
				logger.WarnCF("worker_ws", "Reconnect failed", map[string]any{"error": err.Error()})
			}
		}
	}
}

func (c *WorkerWSChannel) sendJSON(v any) error {
	conn := c.getConn()
	if conn == nil {
		return fmt.Errorf("worker ws not connected")
	}
	return c.writeJSONConn(conn, v)
}

func (c *WorkerWSChannel) writeJSONConn(conn *websocket.Conn, v any) error {
	data, err := json.Marshal(v)
	if err != nil {
		return err
	}
	c.writeMu.Lock()
	defer c.writeMu.Unlock()
	return conn.WriteMessage(websocket.TextMessage, data)
}

func (c *WorkerWSChannel) clearConnIfCurrent(conn *websocket.Conn) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.conn == conn {
		_ = c.conn.Close()
		c.conn = nil
	}
}

func (c *WorkerWSChannel) getConn() *websocket.Conn {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.conn
}

func (c *WorkerWSChannel) cleanTags() []string {
	tags := make([]string, 0, len(c.config.Tags))
	for _, tag := range c.config.Tags {
		tag = strings.TrimSpace(tag)
		if tag != "" {
			tags = append(tags, tag)
		}
	}
	return tags
}

func (c *WorkerWSChannel) reconnectInterval() time.Duration {
	sec := c.config.ReconnectInterval
	if sec <= 0 {
		return 0
	}
	if sec < 5 {
		sec = 5
	}
	return time.Duration(sec) * time.Second
}

func (c *WorkerWSChannel) pingInterval() time.Duration {
	sec := c.config.PingInterval
	if sec <= 0 {
		sec = 15
	}
	if sec < 5 {
		sec = 5
	}
	return time.Duration(sec) * time.Second
}

func (c *WorkerWSChannel) resolveIdentityPath() string {
	path := strings.TrimSpace(c.config.IdentityFile)
	if path == "" {
		path = workerWSDefaultIdentityFile
	}
	return expandHomePath(path)
}

func (c *WorkerWSChannel) loadIdentity() error {
	path := c.resolveIdentityPath()
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	var id workerWSIdentity
	if err := json.Unmarshal(data, &id); err != nil {
		return err
	}
	if id.ID == "" || id.Key == "" {
		return nil
	}

	c.mu.Lock()
	c.identity = id
	c.mu.Unlock()
	return nil
}

func (c *WorkerWSChannel) saveIdentity() error {
	path := c.resolveIdentityPath()

	c.mu.RLock()
	id := c.identity
	c.mu.RUnlock()
	if id.ID == "" || id.Key == "" {
		return nil
	}

	data, err := json.MarshalIndent(id, "", "  ")
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o600)
}

func (c *WorkerWSChannel) resolveTarget(chatID string) any {
	chatID = strings.TrimSpace(chatID)
	if chatID == "" {
		return "*"
	}

	if chatID == "*" || strings.EqualFold(chatID, "all") {
		return chatID
	}

	if strings.Contains(chatID, ",") {
		parts := strings.Split(chatID, ",")
		targets := make([]string, 0, len(parts))
		for _, p := range parts {
			p = strings.TrimSpace(p)
			if p != "" {
				targets = append(targets, p)
			}
		}
		if len(targets) == 1 {
			return targets[0]
		}
		if len(targets) > 1 {
			return targets
		}
	}

	return chatID
}

func (c *WorkerWSChannel) buildPayload(content string) any {
	content = strings.TrimSpace(content)
	if content == "" {
		return map[string]any{"content": ""}
	}

	var parsed any
	if err := json.Unmarshal([]byte(content), &parsed); err == nil {
		return parsed
	}
	return map[string]any{"content": content}
}

func (c *WorkerWSChannel) nextMsgID() string {
	return fmt.Sprintf("mimiclaw-%d", time.Now().UnixNano())
}

func asString(v any) string {
	switch val := v.(type) {
	case string:
		return val
	case json.Number:
		return val.String()
	case float64:
		return fmt.Sprintf("%.0f", val)
	case int64:
		return fmt.Sprintf("%d", val)
	case int:
		return fmt.Sprintf("%d", val)
	default:
		if v == nil {
			return ""
		}
		return fmt.Sprintf("%v", v)
	}
}

func payloadToContent(payload any) string {
	switch p := payload.(type) {
	case nil:
		return ""
	case string:
		return p
	case map[string]any:
		for _, key := range []string{"content", "message", "text", "result", "task"} {
			if s, ok := p[key].(string); ok && strings.TrimSpace(s) != "" {
				return s
			}
		}
		b, err := json.Marshal(p)
		if err != nil {
			return fmt.Sprintf("%v", p)
		}
		return string(b)
	default:
		b, err := json.Marshal(p)
		if err != nil {
			return fmt.Sprintf("%v", p)
		}
		return string(b)
	}
}

func expandHomePath(path string) string {
	if path == "" {
		return path
	}
	if path[0] == '~' {
		home, _ := os.UserHomeDir()
		if len(path) > 1 && (path[1] == '/' || path[1] == '\\') {
			return filepath.Join(home, path[2:])
		}
		return home
	}
	return path
}
