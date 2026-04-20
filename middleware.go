package main

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"sync/atomic"

	"go.uber.org/yarpc/api/transport"
	"go.uber.org/yarpc/yarpcerrors"
)

const (
	TreeHeader   = "x-error-tree"
	BypassHeader = "x-error-bypass"
	HeaderLimit  = 3072 // 3KB hard limit
)

type NodeStatus string

const (
	StatusSuccess NodeStatus = "S" // Success
	StatusError   NodeStatus = "E" // Error
)

type ctxKey int

const (
	collectorKey ctxKey = iota
	bypassKey
)

type ErrorNode struct {
	Service   string       `json:"s"`
	Status    NodeStatus   `json:"st"`
	Code      string       `json:"c,omitempty"`
	Message   string       `json:"e,omitempty"`
	Children  []*ErrorNode `json:"ch,omitempty"`
	Truncated bool         `json:"tr,omitempty"`
}

type TreeCollector struct {
	children atomic.Pointer[[]*ErrorNode]
	mu       sync.Mutex
	// children []*ErrorNode
	seen map[string]bool
}

func NewTreeCollector() *TreeCollector {
	col := &TreeCollector{seen: make(map[string]bool)}
	empty := make([]*ErrorNode, 0)
	col.children.Store(&empty)
	return col
}

type TreeMiddleware struct {
	ServiceName string
}

// middleware for inbound requests
func (m *TreeMiddleware) Handle(ctx context.Context, req *transport.Request, resw transport.ResponseWriter, handler transport.UnaryHandler) error {

	val, ok := req.Headers.Get(BypassHeader)

	if ok && val == "true" {
		return handler.Handle(context.WithValue(ctx, bypassKey, true), req, resw)
	}

	col := NewTreeCollector()

	ctx = context.WithValue(ctx, collectorKey, col)
	err := handler.Handle(ctx, req, resw)

	status, code, msg := StatusSuccess, "OK", ""
	capturedChildren := *col.children.Load()

	if err != nil {
		status = StatusError
		yErr := yarpcerrors.FromError(err)
		code = yErr.Code().String()
		msg = yErr.Message()

		// Simple Error Drift Detection: Compare current error with the first child failure
		if len(capturedChildren) > 0 && capturedChildren[0].Status == StatusError {
			if capturedChildren[0].Code != code {
				msg = fmt.Sprintf("[DRIFT: %s -> %s] %s", capturedChildren[0].Code, code, msg)
			}
		}
	}

	root := &ErrorNode{
		Service: m.ServiceName, Status: status, Code: code, Message: msg, Children: capturedChildren,
	}

	resw.AddHeaders(transport.HeadersFromMap(map[string]string{
		TreeHeader: string(m.trim(root)),
	}))
	return err
}

// middleware for outbound requests
func (m *TreeMiddleware) Call(ctx context.Context, req *transport.Request, out transport.UnaryOutbound) (*transport.Response, error) {
	if val, ok := ctx.Value(bypassKey).(bool); ok && val {
		req.Headers = req.Headers.With(BypassHeader, "true")
	}

	res, err := out.Call(ctx, req)
	col, ok := ctx.Value(collectorKey).(*TreeCollector)
	if !ok || res == nil {
		return res, err
	}

	if val, ok := res.Headers.Get(TreeHeader); ok {
		var child ErrorNode
		if json.Unmarshal([]byte(val), &child) == nil {
			key := fmt.Sprintf("%s-%s", child.Service, child.Code)
			col.mu.Lock()
			// To avoid duplicates in case of RPC retries
			if !col.seen[key] {
				// 1. Get the current slice
				oldSlice := *col.children.Load()

				// 2. Create a NEW slice with the new child
				newSlice := make([]*ErrorNode, len(oldSlice), len(oldSlice)+1)
				copy(newSlice, oldSlice)
				newSlice = append(newSlice, &child)

				// 3. Atomically swap the pointer
				col.children.Store(&newSlice)
				col.seen[key] = true
			}
			col.mu.Unlock()
		}
	}
	return res, err
}

func (m *TreeMiddleware) trim(root *ErrorNode) []byte {
	thresholds := []int{50, 10, 0} // Progressive trimming passes
	for _, t := range thresholds {
		raw, _ := json.Marshal(root)
		if len(raw) <= HeaderLimit {
			return raw
		}
		m.walkAndTrim(root, t)
	}
	root.Children = nil
	raw, _ := json.Marshal(root)
	return raw
}

func (m *TreeMiddleware) walkAndTrim(n *ErrorNode, limit int) {
	if len(n.Message) > limit {
		if limit == 0 {
			n.Message = ""
		} else {
			n.Message = n.Message[:limit] + ".."
		}
		n.Truncated = true
	}
	for _, child := range n.Children {
		m.walkAndTrim(child, limit)
	}
}
