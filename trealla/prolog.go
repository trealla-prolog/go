// Package trealla provides a Prolog interpreter.
// Powered by Trealla Prolog running under WebAssembly.
package trealla

import (
	"context"
	"fmt"
	"log"
	"sync"

	"github.com/wasmerio/wasmer-go/wasmer"
)

// Prolog is a Prolog interpreter.
type Prolog interface {
	// Query executes a query.
	Query(ctx context.Context, query string, options ...QueryOption) Query
	// QueryOnce executes a query, retrieving a single answer and ignoring others.
	QueryOnce(ctx context.Context, query string, options ...QueryOption) (Answer, error)
	// Consult loads a Prolog file with the given path.
	Consult(ctx context.Context, filename string) error
}

type prolog struct {
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

	procs map[string]procedure

	preopen string
	dirs    map[string]string
	library string
	trace   bool
	quiet   bool

	stdout *log.Logger
	stderr *log.Logger
	debug  *log.Logger

	mu *sync.Mutex
}

// New creates a new Prolog interpreter.
func New(opts ...Option) (Prolog, error) {
	pl := &prolog{
		procs: make(map[string]procedure),
		mu:    new(sync.Mutex),
	}
	for _, opt := range opts {
		opt(pl)
	}
	return pl, pl.init()
}

func (pl *prolog) init() error {
	builder := wasmer.NewWasiStateBuilder("tpl").
		Argument("-g").Argument("use_module(user), halt").
		Argument("--ns").
		CaptureStdout().
		CaptureStderr()
	if pl.library != "" {
		builder = builder.Argument("--library").Argument(pl.library)
	}
	if pl.trace {
		builder = builder.Argument("-t")
	}
	if pl.quiet {
		builder = builder.Argument("-q")
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

	importObject, err := wasiEnv.GenerateImportObject(wasmStore, wasmModule)
	if err != nil {
		return err
	}
	importObject.Register("trealla", pl.exports())

	instance, err := wasmer.NewInstance(wasmModule, importObject)
	if err != nil {
		return err
	}
	pl.instance = instance

	// run once to initialize global interpreter

	start, err := instance.Exports.GetWasiStartFunction()
	if err != nil {
		return fmt.Errorf("trealla: failed to get start function: %w", err)
	}
	if _, err := start(); err != nil {
		return fmt.Errorf("trealla: failed to initialize: %w", err)
	}

	mem, err := instance.Exports.GetMemory("memory")
	if err != nil {
		return fmt.Errorf("trealla: failed to get memory: %w", err)
	}
	pl.memory = mem

	pl_global, err := instance.Exports.GetFunction("pl_global")
	if err != nil {
		return errUnexported("pl_global", err)
	}

	ptr, err := pl_global()
	if err != nil {
		return fmt.Errorf("trealla: failed to get interpreter: %w", err)
	}
	pl.ptr = ptr.(int32)

	pl.realloc, err = instance.Exports.GetFunction("canonical_abi_realloc")
	if err != nil {
		return errUnexported("canonical_abi_realloc", err)
	}

	pl.free, err = instance.Exports.GetFunction("canonical_abi_free")
	if err != nil {
		return errUnexported("canonical_abi_free", err)
	}

	pl.pl_query, err = instance.Exports.GetFunction("pl_query")
	if err != nil {
		return errUnexported("pl_query", err)
	}

	pl.pl_redo, err = instance.Exports.GetFunction("pl_redo")
	if err != nil {
		return errUnexported("pl_redo", err)
	}

	pl.pl_done, err = instance.Exports.GetFunction("pl_done")
	if err != nil {
		return errUnexported("pl_done", err)
	}

	pl.pl_consult, err = instance.Exports.GetFunction("pl_consult")
	if err != nil {
		return errUnexported("pl_consult", err)
	}

	if err := pl.loadBuiltins(); err != nil {
		return fmt.Errorf("trealla: failed to load builtins: %w", err)
	}

	return nil
}

func (pl *prolog) consultText(module, text string) error {
	// Module:'$load_chars'(Text).
	goal := Atom(":").Of(Atom(module), Atom("$load_chars").Of(text))
	_, err := pl.QueryOnce(context.Background(), goal.String())
	return err
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

// WithTrace enables tracing for all queries. Traces write to to the query's standard error text stream.
// You can also use the `trace/0` predicate to enable tracing for specific queries.
// Use together with WithStderrLog for automatic tracing.
func WithTrace() Option {
	return func(pl *prolog) {
		pl.trace = true
	}
}

// WithQuiet enables the quiet option. This disables some warning messages.
func WithQuiet() Option {
	return func(pl *prolog) {
		pl.quiet = true
	}
}

// WithStdoutLog sets the standard output logger, writing all stdout input from queries.
func WithStdoutLog(logger *log.Logger) Option {
	return func(pl *prolog) {
		pl.stdout = logger
	}
}

// WithStderrLog sets the standard error logger, writing all stderr input from queries.
// Note that traces are written to stderr.
func WithStderrLog(logger *log.Logger) Option {
	return func(pl *prolog) {
		pl.stderr = logger
	}
}

// WithDebugLog writes debug messages to the given logger.
func WithDebugLog(logger *log.Logger) Option {
	return func(pl *prolog) {
		pl.debug = logger
	}
}
