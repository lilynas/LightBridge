package service

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	openaiwsv2 "github.com/WilliamWang1721/LightBridge/internal/service/openai_ws_v2"
	coderws "github.com/coder/websocket"
	"github.com/tidwall/gjson"
)

// openAIWSInputNamespaceCompatFrameConn keeps passthrough mode transparent for
// normal traffic while recovering one request on the same upstream connection
// when a compatible Responses implementation rejects input[n].namespace.
// Retrying on the same socket is essential for store=false tool continuations:
// moving function_call_output to a new socket can invalidate previous_response_id.
type openAIWSInputNamespaceCompatFrameConn struct {
	inner        openaiwsv2.FrameConn
	accountID    int64
	writeTimeout time.Duration

	writeMu sync.Mutex
	stateMu sync.Mutex
	last    openAIWSNamespaceCompatRequest

	// Learned only after this physical upstream socket explicitly rejects an
	// input namespace. Keeping the capability on the connection avoids making
	// every later turn fail once before retrying.
	inputNamespacesUnsupported atomic.Bool
}

type openAIWSNamespaceCompatRequest struct {
	msgType coderws.MessageType
	payload []byte
	retried bool
}

var _ openaiwsv2.FrameConn = (*openAIWSInputNamespaceCompatFrameConn)(nil)

func newOpenAIWSInputNamespaceCompatFrameConn(
	inner openaiwsv2.FrameConn,
	accountID int64,
	writeTimeout time.Duration,
) openaiwsv2.FrameConn {
	if inner == nil {
		return nil
	}
	return &openAIWSInputNamespaceCompatFrameConn{
		inner:        inner,
		accountID:    accountID,
		writeTimeout: writeTimeout,
	}
}

func (c *openAIWSInputNamespaceCompatFrameConn) ReadFrame(ctx context.Context) (coderws.MessageType, []byte, error) {
	if c == nil || c.inner == nil {
		return coderws.MessageText, nil, errOpenAIWSConnClosed
	}
	if ctx == nil {
		ctx = context.Background()
	}
	for {
		msgType, payload, err := c.inner.ReadFrame(ctx)
		if err != nil {
			return msgType, payload, err
		}
		if msgType != coderws.MessageText || !shouldRetryOpenAIResponsesWSEventWithoutInputNamespaces(payload) {
			return msgType, payload, nil
		}

		retried, retryErr := c.retryLastResponseCreate(ctx)
		if retryErr != nil {
			return msgType, nil, retryErr
		}
		if !retried {
			return msgType, payload, nil
		}
		// Swallow only the compatibility error that triggered a successful retry.
		// The next upstream frame belongs to the normalized request.
	}
}

func (c *openAIWSInputNamespaceCompatFrameConn) WriteFrame(ctx context.Context, msgType coderws.MessageType, payload []byte) error {
	if c == nil || c.inner == nil {
		return errOpenAIWSConnClosed
	}
	if ctx == nil {
		ctx = context.Background()
	}
	c.writeMu.Lock()
	defer c.writeMu.Unlock()
	if msgType == coderws.MessageText &&
		gjson.GetBytes(payload, "type").String() == "response.create" &&
		c.inputNamespacesUnsupported.Load() {
		normalized, changed, err := stripOpenAIResponsesInputNamespacesFromBody(payload)
		if err != nil {
			return err
		}
		if changed {
			payload = normalized
			logOpenAIWSModeInfo(
				"passthrough_ws_input_namespace_compat_apply account_id=%d action=strip_input_namespaces source=connection_capability payload_bytes=%d",
				c.accountID,
				len(payload),
			)
		}
	}
	if err := c.inner.WriteFrame(ctx, msgType, payload); err != nil {
		return err
	}
	if msgType == coderws.MessageText && gjson.GetBytes(payload, "type").String() == "response.create" {
		c.stateMu.Lock()
		c.last = openAIWSNamespaceCompatRequest{
			msgType: msgType,
			payload: append([]byte(nil), payload...),
		}
		c.stateMu.Unlock()
	}
	return nil
}

func (c *openAIWSInputNamespaceCompatFrameConn) Close() error {
	if c == nil || c.inner == nil {
		return nil
	}
	return c.inner.Close()
}

func (c *openAIWSInputNamespaceCompatFrameConn) retryLastResponseCreate(ctx context.Context) (bool, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	c.writeMu.Lock()
	defer c.writeMu.Unlock()

	c.stateMu.Lock()
	if c.last.retried || len(c.last.payload) == 0 {
		c.stateMu.Unlock()
		return false, nil
	}
	normalized, changed, err := stripOpenAIResponsesInputNamespacesFromBody(c.last.payload)
	if err != nil {
		c.stateMu.Unlock()
		return false, err
	}
	if !changed {
		c.stateMu.Unlock()
		return false, nil
	}
	msgType := c.last.msgType
	c.last.payload = append(c.last.payload[:0], normalized...)
	c.last.retried = true
	c.inputNamespacesUnsupported.Store(true)
	c.stateMu.Unlock()

	writeCtx := ctx
	cancel := func() {}
	if c.writeTimeout > 0 {
		writeCtx, cancel = context.WithTimeout(ctx, c.writeTimeout)
	}
	defer cancel()
	if err := c.inner.WriteFrame(writeCtx, msgType, normalized); err != nil {
		return false, fmt.Errorf("retry upstream websocket request without input namespace: %w", err)
	}
	logOpenAIWSModeInfo(
		"passthrough_ws_input_namespace_compat_retry account_id=%d action=strip_input_namespaces retry=1 payload_bytes=%d",
		c.accountID,
		len(normalized),
	)
	return true, nil
}
