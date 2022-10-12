package trealla

import (
	"context"
	_ "embed"
	"fmt"
	"sync"

	"github.com/wasmerio/wasmer-go/wasmer"
)

//go:embed tpl.wasm
var tplWASM []byte

var wasmEngine = wasmer.NewEngine()

// Prolog is a Prolog interpreter.
type Prolog interface {
	// Query executes a query.
	Query(ctx context.Context, query string, options ...QueryOption) Query
	QueryOnce(ctx context.Context, query string, options ...QueryOption) (Answer, error)
	// Consult loads a Prolog file with the given path.
	Consult(ctx context.Context, filename string) error
}

type wasmFunc func(...any) (any, error)

type prolog struct {
	engine   *wasmer.Engine
	store    *wasmer.Store
	module   *wasmer.Module
	instance *wasmer.Instance
	wasi     *wasmer.WasiEnvironment
	memory   *wasmer.Memory

	ptr        int32
	realloc    wasmFunc
	free       wasmFunc
	pl_query   wasmFunc
	pl_consult wasmFunc
	pl_redo    wasmFunc
	pl_done    wasmFunc

	preopen string
	dirs    map[string]string
	library string

	mu *sync.Mutex
}

// New creates a new Prolog interpreter.
func New(opts ...Option) (Prolog, error) {
	pl, err := newProlog(opts...)
	if err != nil {
		return nil, err
	}
	return pl, nil
}

func newProlog(opts ...Option) (*prolog, error) {
	store := wasmer.NewStore(wasmEngine)
	module, err := wasmer.NewModule(store, tplWASM)
	if err != nil {
		return nil, err
	}
	pl := &prolog{
		engine: wasmEngine,
		store:  store,
		module: module,
		mu:     new(sync.Mutex),
	}
	for _, opt := range opts {
		opt(pl)
	}
	err = pl.init()
	return pl, err
}

func (pl *prolog) init() error {
	builder := wasmer.NewWasiStateBuilder("tpl").
		Argument("-g").Argument("halt").
		Argument("-q").
		Argument("--ns").
		CaptureStdout().
		CaptureStderr()
	if pl.library != "" {
		builder = builder.Argument("--library").Argument(pl.library)
	}
	if pl.preopen != "" {
		builder = builder.PreopenDirectory(pl.preopen)
	}
	for alias, dir := range pl.dirs {
		builder = builder.MapDirectory(alias, dir)
	}
	wasiEnv, err := builder.Finalize()
	if err != nil {
		return fmt.Errorf("trealla: failed to init WASI: %w", err)
	}
	pl.wasi = wasiEnv
	importObject, err := wasiEnv.GenerateImportObject(pl.store, pl.module)
	if err != nil {
		return err
	}

	instance, err := wasmer.NewInstance(pl.module, importObject)
	if err != nil {
		return err
	}
	pl.instance = instance

	// run once to initialize global interpreter
	start, err := instance.Exports.GetWasiStartFunction()
	if err != nil {
		return err
	}
	if _, err := start(); err != nil {
		return fmt.Errorf("trealla: failed to initialize: %w", err)
	}

	mem, err := instance.Exports.GetMemory("memory")
	if err != nil {
		return err
	}
	pl.memory = mem

	pl_global, err := instance.Exports.GetFunction("pl_global")
	if err != nil {
		return err
	}
	ptr, err := pl_global()
	if err != nil {
		return err
	}
	pl.ptr = ptr.(int32)

	realloc, err := instance.Exports.GetFunction("canonical_abi_realloc")
	if err != nil {
		return err
	}
	pl.realloc = realloc

	free, err := instance.Exports.GetFunction("canonical_abi_free")
	if err != nil {
		return err
	}
	pl.free = free

	pl_query, err := instance.Exports.GetFunction("pl_query")
	if err != nil {
		return err
	}
	pl.pl_query = pl_query

	pl_redo, err := instance.Exports.GetFunction("pl_redo")
	if err != nil {
		return err
	}
	pl.pl_redo = pl_redo

	pl_done, err := instance.Exports.GetFunction("pl_done")
	if err != nil {
		return err
	}
	pl.pl_done = pl_done

	pl_consult, err := instance.Exports.GetFunction("pl_consult")
	if err != nil {
		return err
	}
	pl.pl_consult = pl_consult

	return nil
}

func (pl *prolog) Consult(_ context.Context, filename string) error {
	pl.mu.Lock()
	defer pl.mu.Unlock()

	fstr, err := newCString(pl, filename)
	if err != nil {
		return err
	}
	defer fstr.free(pl)

	ret, err := pl.pl_consult(pl.ptr, fstr.ptr)
	if err != nil {
		return err
	}
	if ret.(int32) == 0 {
		return fmt.Errorf("trealla: failed to consult file: %s", filename)
	}
	return nil
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

// WithLibraryPath sets the global library path for the interpreter.
// `use_module(library(foo))` will point to here.
// Equivalent to Trealla's `--library` flag.
func WithLibraryPath(path string) Option {
	return func(pl *prolog) {
		pl.library = path
	}
}
