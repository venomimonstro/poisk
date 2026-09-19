package search

import (
	"context"
	"sync"

	"github.com/venomimonstro/poisk/internal/search/backend"
)

type flightCall struct {
	done   chan struct{}
	result backend.Result
	err    error
}

type flightGroup struct {
	mu    sync.Mutex
	calls map[string]*flightCall
}

func (g *flightGroup) Do(ctx context.Context, key string, fn func() (backend.Result, error)) (backend.Result, error) {
	g.mu.Lock()
	if g.calls == nil { g.calls = make(map[string]*flightCall) }
	if call, ok := g.calls[key]; ok {
		g.mu.Unlock()
		select {
		case <-ctx.Done(): return backend.Result{}, ctx.Err()
		case <-call.done: return call.result, call.err
		}
	}
	call := &flightCall{done: make(chan struct{})}
	g.calls[key] = call
	g.mu.Unlock()

	call.result, call.err = fn()

	g.mu.Lock()
	delete(g.calls, key)
	close(call.done)
	g.mu.Unlock()
	return call.result, call.err
}
