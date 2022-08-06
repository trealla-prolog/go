package trealla

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/wasmerio/wasmer-go/wasmer"
)

//go:embed tpl.wasm
var tplWASM []byte

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
	pl, err := newEngine()
	if err != nil {
		return nil, err
	}
	for _, opt := range opts {
		opt(pl)
	}
	return pl, nil
}

func newEngine() (*prolog, error) {
	engine := wasmer.NewEngine()
	store := wasmer.NewStore(engine)
	module, err := wasmer.NewModule(store, tplWASM)
	if err != nil {
		return nil, err
	}
	return &prolog{
		engine: engine,
		store:  store,
		module: module,
	}, nil
}

const etx = "\x03" // END OF TEXT

// Query executes a query.
func (pl *prolog) Query(ctx context.Context, program string) (Answer, error) {
	raw, err := pl.ask(ctx, program)
	if err != nil {
		return Answer{}, err
	}

	output, js, found := strings.Cut(raw, etx)
	answer := Answer{
		Query:  program,
		Output: output,
	}
	if !found {
		return answer, fmt.Errorf("trealla: unexpected output (missing ETX): %s", raw)
	}

	dec := json.NewDecoder(strings.NewReader(js))
	dec.UseNumber()
	if err := dec.Decode(&answer); err != nil {
		return answer, fmt.Errorf("trealla: decoding error: %w", err)
	}

	// var trapErr *wasmer.TrapError
	// if errors.As(err, &trapErr) {
	// 	// TODO: handle halt(nonzero)
	// }

	return answer, nil
}

// Answer is a query result.
type Answer struct {
	Query   string
	Result  string
	Answers []Solution
	Error   any
	Output  string
}

func (pl *prolog) ask(ctx context.Context, query string) (string, error) {
	query = escapeQuery(query)
	builder := wasmer.NewWasiStateBuilder("tpl").
		Argument("-g").Argument(query).
		Argument("-q").
		CaptureStdout()
	if pl.preopen != "" {
		builder = builder.PreopenDirectory(pl.preopen)
	}
	for alias, dir := range pl.dirs {
		builder = builder.MapDirectory(alias, dir)
	}
	wasiEnv, err := builder.Finalize()
	if err != nil {
		return "", err
	}

	importObject, err := wasiEnv.GenerateImportObject(pl.store, pl.module)
	if err != nil {
		return "", err
	}
	instance, err := wasmer.NewInstance(pl.module, importObject)
	if err != nil {
		return "", err
	}
	defer instance.Close()
	start, err := instance.Exports.GetWasiStartFunction()
	if err != nil {
		return "", err
	}
	ch := make(chan error, 1)
	go func() {
		defer func() {
			if ex := recover(); ex != nil {
				ch <- fmt.Errorf("trealla: panic: %v", ex)
			}
		}()
		_, err := start()
		ch <- err
	}()

	select {
	case <-ctx.Done():
		return "", fmt.Errorf("trealla: canceled: %x", ctx.Err())
	case err := <-ch:
		stdout := string(wasiEnv.ReadStdout())
		return stdout, err
	}
}

func escapeQuery(query string) string {
	query = strings.ReplaceAll(query, `"`, `\"`)
	return fmt.Sprintf(`use_module(library(wasm_toplevel)), wasm_ask("%s")`, query)
}

// Option is an optional parameter for New.
type Option func(*prolog)

// WithPreopenDir sets the preopen directory to dir, granting access to it. Calling this again will overwrite it.
// More or less equivalent to WithMapDir(dir, dir).
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
