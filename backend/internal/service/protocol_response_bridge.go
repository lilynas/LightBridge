package service

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"

	"github.com/WilliamWang1721/LightBridge/internal/pkg/apicompat"
	"github.com/gin-gonic/gin"
)

// AnthropicBridgeTarget identifies the downstream protocol produced from an
// Anthropic Messages response.
type AnthropicBridgeTarget int

const (
	AnthropicBridgeTargetGemini AnthropicBridgeTarget = iota + 1
	AnthropicBridgeTargetResponses
)

// ProtocolResponseBridge is a production ResponseWriter that converts an
// Anthropic response while it is being written. Streaming responses are
// parsed event-by-event and flushed immediately; only non-streaming responses
// and upstream error bodies are buffered.
type ProtocolResponseBridge struct {
	mu sync.Mutex

	target   *gin.Context
	kind     AnthropicBridgeTarget
	stream   bool
	model    string
	header   http.Header
	status   int
	size     int
	started  bool
	finished bool
	pending  bytes.Buffer
	body     bytes.Buffer
	err      error

	responsesState *apicompat.AnthropicEventToResponsesState
}

func NewProtocolResponseBridge(target *gin.Context, kind AnthropicBridgeTarget, stream bool, model string) *ProtocolResponseBridge {
	bridge := &ProtocolResponseBridge{
		target: target,
		kind:   kind,
		stream: stream,
		model:  model,
		header: make(http.Header),
		status: http.StatusOK,
		size:   -1,
	}
	if kind == AnthropicBridgeTargetResponses {
		bridge.responsesState = apicompat.NewAnthropicEventToResponsesState()
		bridge.responsesState.Model = model
	}
	return bridge
}

func NewProtocolBridgeContext(parent *gin.Context, ctx context.Context, path string, body []byte, bridge *ProtocolResponseBridge) (*gin.Context, error) {
	if bridge == nil {
		return nil, fmt.Errorf("protocol response bridge is nil")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, path, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	if parent == nil {
		return nil, fmt.Errorf("protocol bridge parent context is nil")
	}
	if parent.Request != nil {
		req.Header = parent.Request.Header.Clone()
		req.Host = parent.Request.Host
		req.RemoteAddr = parent.Request.RemoteAddr
		req.TLS = parent.Request.TLS
	}
	req.Header.Set("Content-Type", "application/json")
	capture := parent.Copy()
	capture.Writer = bridge
	capture.Request = req
	return capture, nil
}

var _ gin.ResponseWriter = (*ProtocolResponseBridge)(nil)

func (b *ProtocolResponseBridge) Header() http.Header {
	return b.header
}

func (b *ProtocolResponseBridge) Status() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.status
}

func (b *ProtocolResponseBridge) Size() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.size
}

func (b *ProtocolResponseBridge) Written() bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.size >= 0
}

func (b *ProtocolResponseBridge) WriteString(value string) (int, error) {
	return b.Write([]byte(value))
}

func (b *ProtocolResponseBridge) WriteHeaderNow() {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.finished || b.size >= 0 {
		return
	}
	b.size = 0
	b.started = true
	if b.stream && b.status < http.StatusBadRequest {
		b.err = b.ensureStreamingHeadersLocked()
	}
}

func (b *ProtocolResponseBridge) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	if b == nil || b.target == nil {
		return nil, nil, fmt.Errorf("protocol bridge target is nil")
	}
	hijacker, ok := any(b.target.Writer).(http.Hijacker)
	if !ok {
		return nil, nil, fmt.Errorf("protocol bridge downstream does not support hijacking")
	}
	return hijacker.Hijack()
}

func (b *ProtocolResponseBridge) Pusher() http.Pusher {
	if b == nil || b.target == nil {
		return nil
	}
	return b.target.Writer.Pusher()
}

// Unwrap allows net/http ResponseController and middleware to reach the real
// downstream writer rather than treating the protocol bridge as a terminal
// test writer.
func (b *ProtocolResponseBridge) Unwrap() http.ResponseWriter {
	if b == nil || b.target == nil {
		return nil
	}
	return b.target.Writer
}

// CloseNotify preserves Gin's streaming cancellation semantics. Gin 1.x may
// still use the deprecated http.CloseNotifier interface from Context.Stream.
func (b *ProtocolResponseBridge) CloseNotify() <-chan bool {
	closed := make(chan bool, 1)
	if b == nil || b.target == nil || b.target.Request == nil {
		return closed
	}
	done := b.target.Request.Context().Done()
	if done == nil {
		return closed
	}
	go func() {
		<-done
		closed <- true
		close(closed)
	}()
	return closed
}

func (b *ProtocolResponseBridge) WriteHeader(statusCode int) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.started || b.finished {
		return
	}
	if statusCode > 0 {
		b.status = statusCode
	}
}

func (b *ProtocolResponseBridge) Write(data []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.finished {
		return 0, http.ErrHandlerTimeout
	}
	if b.err != nil {
		return 0, b.err
	}
	if b.size < 0 {
		b.size = 0
	}
	b.started = true
	if !b.stream || b.status >= http.StatusBadRequest {
		n, err := b.body.Write(data)
		b.size += n
		return n, err
	}
	if err := b.ensureStreamingHeadersLocked(); err != nil {
		b.err = err
		return 0, err
	}
	if _, err := b.pending.Write(data); err != nil {
		b.err = err
		return 0, err
	}
	if err := b.drainEventsLocked(false); err != nil {
		b.err = err
		return 0, err
	}
	b.size += len(data)
	return len(data), nil
}

func (b *ProtocolResponseBridge) Flush() {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.finished || b.err != nil || !b.stream || b.status >= http.StatusBadRequest {
		return
	}
	if b.size < 0 {
		b.size = 0
		b.started = true
	}
	if err := b.ensureStreamingHeadersLocked(); err != nil {
		b.err = err
		return
	}
	if flusher, ok := any(b.target.Writer).(http.Flusher); ok {
		flusher.Flush()
	}
}

func (b *ProtocolResponseBridge) Finalize() error {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.finished {
		return b.err
	}
	b.finished = true
	if b.err != nil {
		return b.err
	}
	if !b.started && b.body.Len() == 0 && b.pending.Len() == 0 {
		if b.target != nil && !b.target.Writer.Written() {
			copySafeBridgeHeaders(b.target.Writer.Header(), b.header)
			b.target.Status(b.status)
			b.target.Writer.WriteHeaderNow()
		}
		return nil
	}
	if !b.stream || b.status >= http.StatusBadRequest {
		return b.finalizeBufferedLocked()
	}
	if err := b.ensureStreamingHeadersLocked(); err != nil {
		return err
	}
	if err := b.drainEventsLocked(true); err != nil {
		return err
	}
	if b.kind == AnthropicBridgeTargetResponses {
		for _, event := range apicompat.FinalizeAnthropicResponsesStream(b.responsesState) {
			line, err := apicompat.ResponsesEventToSSE(event)
			if err != nil {
				return err
			}
			if _, err := io.WriteString(b.target.Writer, line); err != nil {
				return err
			}
		}
	}
	if flusher, ok := any(b.target.Writer).(http.Flusher); ok {
		flusher.Flush()
	}
	return nil
}

func (b *ProtocolResponseBridge) ensureStreamingHeadersLocked() error {
	if b.target == nil {
		return fmt.Errorf("protocol bridge target is nil")
	}
	if b.target.Writer.Written() {
		return nil
	}
	copySafeBridgeHeaders(b.target.Writer.Header(), b.header)
	b.target.Header("Content-Type", "text/event-stream")
	b.target.Header("Cache-Control", "no-cache")
	b.target.Header("X-Accel-Buffering", "no")
	b.target.Status(http.StatusOK)
	return nil
}

func (b *ProtocolResponseBridge) drainEventsLocked(final bool) error {
	raw := strings.ReplaceAll(b.pending.String(), "\r\n", "\n")
	b.pending.Reset()
	for {
		idx := strings.Index(raw, "\n\n")
		if idx < 0 {
			break
		}
		block := raw[:idx]
		raw = raw[idx+2:]
		if err := b.convertEventLocked(block); err != nil {
			return err
		}
	}
	if final && strings.TrimSpace(raw) != "" {
		if err := b.convertEventLocked(raw); err != nil {
			return err
		}
		raw = ""
	}
	_, _ = b.pending.WriteString(raw)
	return nil
}

func (b *ProtocolResponseBridge) convertEventLocked(block string) error {
	eventName := ""
	dataLines := make([]string, 0, 2)
	for _, rawLine := range strings.Split(block, "\n") {
		line := strings.TrimSuffix(rawLine, "\r")
		switch {
		case strings.HasPrefix(line, "event:"):
			eventName = strings.TrimSpace(strings.TrimPrefix(line, "event:"))
		case strings.HasPrefix(line, "data:"):
			dataLines = append(dataLines, strings.TrimSpace(strings.TrimPrefix(line, "data:")))
		}
	}
	payload := strings.TrimSpace(strings.Join(dataLines, "\n"))
	if payload == "" || payload == "[DONE]" {
		return nil
	}
	var event apicompat.AnthropicStreamEvent
	if err := json.Unmarshal([]byte(payload), &event); err != nil {
		return fmt.Errorf("decode anthropic SSE event: %w", err)
	}
	if event.Type == "" {
		event.Type = eventName
	}
	switch b.kind {
	case AnthropicBridgeTargetGemini:
		for _, chunk := range anthropicStreamEventToGeminiChunks(&event, b.model) {
			encoded, err := json.Marshal(chunk)
			if err != nil {
				return err
			}
			if _, err := fmt.Fprintf(b.target.Writer, "data: %s\n\n", encoded); err != nil {
				return err
			}
		}
	case AnthropicBridgeTargetResponses:
		for _, responseEvent := range apicompat.AnthropicEventToResponsesEvents(&event, b.responsesState) {
			line, err := apicompat.ResponsesEventToSSE(responseEvent)
			if err != nil {
				return err
			}
			if _, err := io.WriteString(b.target.Writer, line); err != nil {
				return err
			}
		}
	default:
		return fmt.Errorf("unsupported protocol bridge target: %d", b.kind)
	}
	if flusher, ok := any(b.target.Writer).(http.Flusher); ok {
		flusher.Flush()
	}
	return nil
}

func (b *ProtocolResponseBridge) finalizeBufferedLocked() error {
	if b.target == nil {
		return fmt.Errorf("protocol bridge target is nil")
	}
	status := b.status
	if status <= 0 {
		status = http.StatusOK
	}
	body := b.body.Bytes()
	if len(body) == 0 {
		copySafeBridgeHeaders(b.target.Writer.Header(), b.header)
		b.target.Status(status)
		b.target.Writer.WriteHeaderNow()
		return nil
	}
	if status >= http.StatusBadRequest {
		copySafeBridgeHeaders(b.target.Writer.Header(), b.header)
		contentType := b.header.Get("Content-Type")
		if contentType == "" {
			contentType = "application/json"
		}
		b.target.Data(status, contentType, body)
		return nil
	}
	switch b.kind {
	case AnthropicBridgeTargetGemini:
		return WriteCapturedAnthropicAsGemini(b.target, status, b.header, body, false, b.model)
	case AnthropicBridgeTargetResponses:
		var anthropicResponse apicompat.AnthropicResponse
		if err := json.Unmarshal(body, &anthropicResponse); err != nil {
			contentType := b.header.Get("Content-Type")
			if contentType == "" {
				contentType = "application/json"
			}
			b.target.Data(status, contentType, body)
			return err
		}
		response := apicompat.AnthropicToResponsesResponse(&anthropicResponse)
		if response.Model == "" {
			response.Model = b.model
		}
		if requestID := b.header.Get("x-request-id"); requestID != "" {
			b.target.Header("x-request-id", requestID)
		}
		b.target.JSON(http.StatusOK, response)
		return nil
	default:
		return fmt.Errorf("unsupported protocol bridge target: %d", b.kind)
	}
}

func copySafeBridgeHeaders(dst, src http.Header) {
	for key, values := range src {
		if isHopByHopHeader(key) {
			continue
		}
		for _, value := range values {
			dst.Add(key, value)
		}
	}
}

func isHopByHopHeader(key string) bool {
	switch strings.ToLower(strings.TrimSpace(key)) {
	case "connection", "keep-alive", "proxy-authenticate", "proxy-authorization", "te", "trailer", "transfer-encoding", "upgrade":
		return true
	default:
		return false
	}
}
