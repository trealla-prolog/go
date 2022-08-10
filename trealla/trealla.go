package trealla

import (
	"context"
	_ "embed"

	"github.com/wasmerio/wasmer-go/wasmer"
)

//go:embed tpl.wasm
var tplWASM []byte

var wasmEngine = wasmer.NewEngine()

// Prolog is a Prolog interpreter.
type Prolog interface {
	// Query executes a query.
	Query(ctx context.Context, query string) (Answer, error)
}

type prolog struct {
	engine *wasmer.Engine
	store  *wasmer.Store
	module *wasmer.Module

	preopen string
	dirs    map[string]string
}

// New creates a new Prolog interpreter.
func New(opts ...Option) (Prolog, error) {
	pl, err := newProlog()
	if err != nil {
		return nil, err
	}
	for _, opt := range opts {
		opt(pl)
	}
	return pl, nil
}

func newProlog() (*prolog, error) {
	store := wasmer.NewStore(wasmEngine)
	module, err := wasmer.NewModule(store, tplWASM)
	if err != nil {
		return nil, err
	}
	return &prolog{
		engine: wasmEngine,
		store:  store,
		module: module,
	}, nil
}

// Option is an optional parameter for New.
type Option func(*prolog)

// WithPreopenDir sets the preopen directory to dir, granting access to it. Calling this again will overwrite it.
// More or less equivalent to `WithMapDir(dir, dir)`.
func WithPreopenDir(dir string) Option {
	return func(pl *prolog) {
		pl.preopen = dir
	}
}

// WithMapDir sets alias to point to directory dir, granting access to it.
// This can be called multiple times with different aliases.
func WithMapDir(alias, dir string) Option {
	return func(pl *prolog) {
		if pl.dirs == nil {
			pl.dirs = make(map[string]string)
		}
		pl.dirs[alias] = dir
	}
}
