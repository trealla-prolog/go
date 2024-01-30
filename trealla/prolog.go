// Package trealla provides a Prolog interpreter.
// Powered by Trealla Prolog running under WebAssembly.
package trealla

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"log"
	"runtime"
	"sync"

	"github.com/bytecodealliance/wasmtime-go/v17"
	"golang.org/x/exp/maps"
)

const defaultConcurrency = 256

// Prolog is a Prolog interpreter.
type Prolog interface {
	// Query executes a query.
	Query(ctx context.Context, query string, options ...QueryOption) Query
	// QueryOnce executes a query, retrieving a single answer and ignoring others.
	QueryOnce(ctx context.Context, query string, options ...QueryOption) (Answer, error)
	// Consult loads a Prolog file with the given path.
	Consult(ctx context.Context, filename string) error
	// ConsultText loads Prolog text into module. Use "user" for the global module.
	ConsultText(ctx context.Context, module string, text string) error
	// Register a native Go predicate.
	// NOTE: this is *experimental* and its API will likely change.
	Register(ctx context.Context, name string, arity int, predicate Predicate) error
	// Clone creates a new clone of this interpreter.
	Clone() (Prolog, error)
	// Close destroys the Prolog instance.
	// If this isn't called and the Prolog variable goes out of scope, runtime finalizers will try to free the memory.
	Close()
	// Stats returns diagnostic information.
	Stats() Stats
}

type prolog struct {
	instance *wasmtime.Instance
	store    *wasmtime.Store
	wasi     *wasmtime.WasiConfig
	memory   *wasmtime.Memory
	closing  bool
	running  map[int32]*query
	spawning map[int32]*query
	limiter  chan struct{}

	ptr             int32
	realloc         wasmFunc
	free            wasmFunc
	pl_consult      wasmFunc
	pl_capture      wasmFunc
	pl_capture_read wasmFunc
	pl_capture_free wasmFunc
	pl_query        wasmFunc
	pl_redo         wasmFunc
	pl_done         wasmFunc

	procs map[string]Predicate

	preopen string
	dirs    map[string]string
	library string
	trace   bool
	quiet   bool
	max     int

	stdout *log.Logger
	stderr *log.Logger
	debug  *log.Logger

	mu *sync.Mutex
}

// New creates a new Prolog interpreter.
func New(opts ...Option) (Prolog, error) {
	pl := &prolog{
		running:  make(map[int32]*query),
		spawning: make(map[int32]*query),
		procs:    make(map[string]Predicate),
		mu:       new(sync.Mutex),
		max:      defaultConcurrency,
	}
	for _, opt := range opts {
		opt(pl)
	}
	if pl.max > 0 {
		pl.limiter = make(chan struct{}, pl.max)
	}
	return pl, pl.init(nil)
}

func (pl *prolog) argv() []string {
	args := []string{"tpl", "-g", "halt", "--ns"}
	if pl.library != "" {
		args = append(args, "--library", pl.library)
	}
	if pl.trace {
		args = append(args, "-t")
	}
	if pl.quiet {
		args = append(args, "-q")
	}
	return args
}

func (pl *prolog) init(parent *prolog) error {
	argv := pl.argv()
	wasi := wasmtime.NewWasiConfig()
	wasi.SetArgv(argv)
	if pl.preopen != "" {
		if err := wasi.PreopenDir(pl.preopen, "/"); err != nil {
			panic(err)
		}
	}
	for alias, dir := range pl.dirs {
		if err := wasi.PreopenDir(dir, alias); err != nil {
			panic(err)
		}
	}

	pl.wasi = wasi

	pl.store = wasmtime.NewStore(wasmEngine)
	pl.store.SetWasi(wasi)

	linker := wasmtime.NewLinker(wasmEngine)
	if err := linker.DefineWasi(); err != nil {
		return err
	}
	if err := linker.DefineFunc(pl.store, "trealla", "host-call", pl.hostCall); err != nil {
		return err
	}
	if err := linker.DefineFunc(pl.store, "trealla", "host-resume", hostResume); err != nil {
		return err
	}
	instance, err := linker.Instantiate(pl.store, wasmModule)
	if err != nil {
		return err
	}
	pl.instance = instance

	// run once to initialize global interpreter
	if parent == nil {
		start := instance.GetExport(pl.store, "_start")
		if start == nil {
			return fmt.Errorf("trealla: failed to get start function")
		}
		if _, err := start.Func().Call(pl.store); err != nil {
			return fmt.Errorf("trealla: failed to initialize: %w", err)
		}
	}

	mem := instance.GetExport(pl.store, "memory").Memory()
	if mem == nil {
		return fmt.Errorf("trealla: failed to get memory")
	}
	pl.memory = mem

	pl.realloc, err = pl.function("canonical_abi_realloc")
	if err != nil {
		return err
	}

	pl.free, err = pl.function("canonical_abi_free")
	if err != nil {
		return err
	}

	pl.pl_capture, err = pl.function("pl_capture")
	if err != nil {
		return err
	}

	pl.pl_capture_read, err = pl.function("pl_capture_read")
	if err != nil {
		return err
	}

	pl.pl_capture_free, err = pl.function("pl_capture_free")
	if err != nil {
		return err
	}

	pl.pl_query, err = pl.function("pl_query")
	if err != nil {
		return err
	}

	pl.pl_redo, err = pl.function("pl_redo")
	if err != nil {
		return err
	}

	pl.pl_done, err = pl.function("pl_done")
	if err != nil {
		return err
	}

	pl.pl_consult, err = pl.function("pl_consult")
	if err != nil {
		return err
	}

	if parent != nil {
		if pl.ptr == 0 {
			runtime.SetFinalizer(pl, (*prolog).Close)
		}
		pl.ptr = parent.ptr
		pl.mu = new(sync.Mutex)
		pl.running = make(map[int32]*query)
		pl.spawning = make(map[int32]*query)
		pl.procs = maps.Clone(parent.procs)
		pl.preopen = parent.preopen
		pl.dirs = parent.dirs
		pl.library = parent.library
		pl.quiet = parent.quiet
		pl.trace = parent.trace
		pl.debug = parent.debug
		if parent.max > 0 {
			pl.max = parent.max
			pl.limiter = make(chan struct{}, pl.max)
		}

		if err := pl.become(parent); err != nil {
			return err
		}

		// if any queries are running while we clone, they get copied over as zombies
		// free them
		for pp := range parent.spawning {
			if ptr := pl.indirect(pp); ptr != 0 {
				if _, err := pl.pl_done.Call(pl.store, ptr); err != nil {
					return err
				}
			}
		}
		for ptr := range parent.running {
			if _, err := pl.pl_done.Call(pl.store, ptr); err != nil {
				return err
			}
		}

		return nil
	}

	runtime.SetFinalizer(pl, (*prolog).Close)

	pl_global, err := pl.function("pl_global")
	if err != nil {
		return err
	}
	ptr, err := pl_global.Call(pl.store)
	if err != nil {
		return fmt.Errorf("trealla: failed to get interpreter: %w", err)
	}
	pl.ptr = ptr.(int32)

	if err := pl.loadBuiltins(); err != nil {
		return fmt.Errorf("trealla: failed to load builtins: %w", err)
	}

	return nil
}

func (pl *prolog) Clone() (Prolog, error) {
	pl.mu.Lock()
	defer pl.mu.Unlock()
	return pl.clone()
}

func (pl *prolog) clone() (*prolog, error) {
	clone := new(prolog)
	err := clone.init(pl)
	return clone, err
}

func (pl *prolog) become(parent *prolog) error {
	delta := parent.memory.Size(parent.store) - pl.memory.Size(pl.store)
	if delta > 0 {
		if _, err := pl.memory.Grow(pl.store, delta); err != nil {
			return err
		}
	}
	copy(pl.memory.UnsafeData(pl.store), parent.memory.UnsafeData(parent.store))
	return nil
}

func (pl *prolog) function(symbol string) (wasmFunc, error) {
	export := pl.instance.GetExport(pl.store, symbol)
	if export == nil {
		return nil, errUnexported(symbol)
	}
	return export.Func(), nil
}

func (pl *prolog) alloc(size int32) (int32, error) {
	ptrv, err := pl.realloc.Call(pl.store, 0, 0, align, size)
	if err != nil {
		return 0, err
	}
	ptr, ok := ptrv.(int32)
	if !ok {
		return 0, fmt.Errorf("unexpected return type for alloc: %T (%v)", ptrv, ptrv)
	}
	if ptr == 0 {
		return 0, fmt.Errorf("trealla: failed to allocate wasm memory (out of memory?)")
	}
	return ptr, nil
}

func (pl *prolog) subquery(addr int32) *query {
	if q, ok := pl.running[addr]; ok {
		return q
	}
	for spawn, q := range pl.spawning {
		if ptr := pl.indirect(spawn); ptr != 0 {
			if ptr == addr {
				return q
			}
		}
	}
	return nil
}

func (pl *prolog) Close() {
	pl.mu.Lock()
	defer pl.mu.Unlock()
	pl.instance = nil
	pl.memory = nil
	pl.store = nil
	pl.wasi = nil
}

func (pl *prolog) ConsultText(ctx context.Context, module, text string) error {
	pl.mu.Lock()
	defer pl.mu.Unlock()
	if pl.instance == nil {
		return io.EOF
	}
	return pl.consultText(ctx, module, text)
}

func (pl *prolog) consultText(ctx context.Context, module, text string) error {
	// Module:'$load_chars'(Text).
	goal := Atom(":").Of(Atom(module), Atom("$load_chars").Of(text))
	_, err := pl.queryOnce(ctx, goal.String())
	if err != nil {
		err = fmt.Errorf("trealla: consult text failed: %w", err)
	}
	return err
}

func (pl *prolog) Consult(_ context.Context, filename string) error {
	pl.mu.Lock()
	defer pl.mu.Unlock()
	if pl.instance == nil {
		return io.EOF
	}
	return pl.consult(filename)
}

func (pl *prolog) consult(filename string) error {
	fstr, err := newCString(pl, filename)
	if err != nil {
		return err
	}
	defer fstr.free(pl)

	ret, err := pl.pl_consult.Call(pl.store, pl.ptr, fstr.ptr)
	if err != nil {
		return err
	}
	if ret.(int32) == 0 {
		return fmt.Errorf("trealla: failed to consult file: %s", filename)
	}
	return nil
}

func (pl *prolog) indirect(ptr int32) int32 {
	if ptr == 0 {
		return 0
	}

	data := pl.memory.UnsafeData(pl.store)
	buf := bytes.NewBuffer(data[uint32(ptr):uint32(ptr+ptrSize)])
	var v int32
	if err := binary.Read(buf, binary.LittleEndian, &v); err != nil {
		return 0
	}
	return v
}

func (pl *prolog) Register(ctx context.Context, name string, arity int, proc Predicate) error {
	pl.mu.Lock()
	defer pl.mu.Unlock()
	if pl.instance == nil {
		return io.EOF
	}
	return pl.register(ctx, name, arity, proc)
}

func (pl *prolog) register(ctx context.Context, name string, arity int, proc Predicate) error {
	functor := Atom(name)
	pi := piTerm(functor, arity)
	pl.procs[pi.String()] = proc
	vars := numbervars(arity)
	head := functor.Of(vars...)
	body := Atom("host_rpc").Of(head)
	clause := fmt.Sprintf(`%s :- %s.`, head.String(), body.String())
	return pl.consultText(ctx, "user", clause)
}

type Stats struct {
	MemorySize int
}

func (pl *prolog) Stats() Stats {
	pl.mu.Lock()
	defer pl.mu.Unlock()
	return pl.stats()
}

func (pl *prolog) stats() Stats {
	if pl.memory == nil || pl.store == nil {
		return Stats{}
	}
	return Stats{
		MemorySize: int(pl.memory.DataSize(pl.store)),
	}
}

// lockedProlog skips the locking the normal *prolog does.
// It's only valid during a single RPC call.
type lockedProlog struct {
	prolog *prolog
	dead   bool
}

func (pl *lockedProlog) kill() {
	pl.dead = true
	pl.prolog = nil
}

func (pl *lockedProlog) ensure() error {
	if pl.dead {
		return fmt.Errorf("trealla: using invalid reference to interpreter")
	}
	return nil
}

func (pl *lockedProlog) Clone() (Prolog, error) {
	if err := pl.ensure(); err != nil {
		return nil, err
	}
	return pl.prolog.clone()
}

func (pl *lockedProlog) Query(ctx context.Context, ask string, options ...QueryOption) Query {
	if err := pl.ensure(); err != nil {
		return &query{err: err}
	}
	return pl.prolog.Query(ctx, ask, append(options, withoutLock)...)
}

func (pl *lockedProlog) QueryOnce(ctx context.Context, query string, options ...QueryOption) (Answer, error) {
	if err := pl.ensure(); err != nil {
		return Answer{}, err
	}
	return pl.prolog.queryOnce(ctx, query, options...)
}

func (pl *lockedProlog) ConsultText(ctx context.Context, module, text string) error {
	if err := pl.ensure(); err != nil {
		return err
	}
	return pl.prolog.consultText(ctx, module, text)
}

func (pl *lockedProlog) Consult(_ context.Context, filename string) error {
	if err := pl.ensure(); err != nil {
		return err
	}
	return pl.prolog.consult(filename)
}

func (pl *lockedProlog) Register(ctx context.Context, name string, arity int, proc Predicate) error {
	if err := pl.ensure(); err != nil {
		return err
	}
	return pl.prolog.register(ctx, name, arity, proc)
}

func (pl *lockedProlog) Close() {
	if err := pl.ensure(); err != nil {
		return
	}
	pl.prolog.closing = true
}

func (pl *lockedProlog) Stats() Stats {
	if err := pl.ensure(); err != nil {
		return Stats{}
	}
	return pl.prolog.stats()
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

// WithMaxConcurrency sets the maximum number of simultaneously running queries.
// This is useful for limiting the amount of memory an interpreter will use.
// Set to 0 to disable concurrency limits. Default is 256.
// Note that interpreters are single-threaded, so only one query is truly executing
// at once, but pending queries can still consume memory (which is currently limited to 4GB).
// This knob will limit the number of queries that can actively consume the interpreter's memory.
func WithMaxConcurrency(queries int) Option {
	return func(pl *prolog) {
		pl.max = queries
	}
}

var (
	_ Prolog = (*prolog)(nil)
	_ Prolog = &lockedProlog{}
)
