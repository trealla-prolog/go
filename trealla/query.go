package trealla

import (
	"context"
	"fmt"
	"io"
	"runtime"
	"strings"
	"sync"
)

const stx = '\x02' // START OF TEXT
const etx = '\x03' // END OF TEXT

// Query is a Prolog query iterator.
type Query interface {
	// Next computes the next solution. Returns true if it found one and false if there are no more results.
	Next(context.Context) bool
	// Current returns the current solution prepared by Next.
	Current() Answer
	// Close destroys this query. It is not necessary to call this if you exhaust results via Next.
	Close() error
	// Err returns this query's error. Always check this after iterating.
	Err() error
}

type query struct {
	pl       *prolog
	goal     string
	bind     bindings
	subquery int32

	cur  Answer
	next *Answer
	err  error
	done bool

	lock bool
	mu   *sync.Mutex
}

// Query executes a query, returning an iterator for results.
func (pl *prolog) Query(ctx context.Context, goal string, options ...QueryOption) Query {
	q := pl.start(ctx, goal, options...)
	runtime.SetFinalizer(q, finalize)
	return q
}

func (pl *prolog) QueryOnce(ctx context.Context, goal string, options ...QueryOption) (Answer, error) {
	q := pl.start(ctx, goal, options...)
	var ans Answer
	if q.Next(ctx) {
		ans = q.Current()
	}
	q.Close()
	return ans, q.Err()
}

func (pl *prolog) start(ctx context.Context, goal string, options ...QueryOption) *query {
	q := &query{
		pl:   pl,
		goal: goal,
		lock: true,
		mu:   new(sync.Mutex),
	}
	for _, opt := range options {
		opt(q)
	}
	if q.lock {
		pl.mu.Lock()
		defer pl.mu.Unlock()
	}
	if q.pl.instance == nil || pl.closing {
		q.setError(io.EOF)
		return q
	}

	if err := q.reify(); err != nil {
		q.setError(err)
		return q
	}
	goalstr, err := newCString(pl, escapeQuery(q.goal))
	if err != nil {
		q.setError(err)
		return q
	}

	if pl.debug != nil {
		pl.debug.Println("query:", q.goal)
	}

	subqptrv, err := pl.realloc.Call(pl.store, 0, 0, 1, 4)
	if err != nil {
		q.setError(fmt.Errorf("trealla: allocation error: %w", err))
		return q
	}
	subqptr := subqptrv.(int32)
	if subqptr == 0 {
		q.setError(fmt.Errorf("trealla: failed to allocate subquery pointer"))
		return q
	}
	defer pl.free.Call(pl.store, subqptr, 4, 1)

	// stdout string pointer
	stdoutptrv, err := pl.realloc.Call(pl.store, 0, 0, 1, 4)
	if err != nil {
		q.setError(fmt.Errorf("trealla: allocation error: %w", err))
		return q
	}
	stdoutpp := stdoutptrv.(int32)
	if stdoutpp == 0 {
		q.setError(fmt.Errorf("trealla: failed to allocate subquery pointer"))
		return q
	}
	defer pl.free.Call(pl.store, stdoutpp, 4, 1)

	// stdout size pointer
	stdoutlenptrv, err := pl.realloc.Call(pl.store, 0, 0, 1, 4)
	if err != nil {
		q.setError(fmt.Errorf("trealla: allocation error: %w", err))
		return q
	}
	stdoutlenptr := stdoutlenptrv.(int32)
	if stdoutlenptr == 0 {
		q.setError(fmt.Errorf("trealla: failed to allocate subquery pointer"))
		return q
	}
	defer pl.free.Call(pl.store, stdoutlenptr, 4, 1)

	// stderr string pointer
	stderrptrv, err := pl.realloc.Call(pl.store, 0, 0, 1, 4)
	if err != nil {
		q.setError(fmt.Errorf("trealla: allocation error: %w", err))
		return q
	}
	stderrpp := stderrptrv.(int32)
	if stderrpp == 0 {
		q.setError(fmt.Errorf("trealla: failed to allocate subquery pointer"))
		return q
	}
	defer pl.free.Call(pl.store, stderrpp, 4, 1)

	// stderr size pointer
	stderrlenptrv, err := pl.realloc.Call(pl.store, 0, 0, 1, 4)
	if err != nil {
		q.setError(fmt.Errorf("trealla: allocation error: %w", err))
		return q
	}
	stderrlenptr := stderrlenptrv.(int32)
	if stderrlenptr == 0 {
		q.setError(fmt.Errorf("trealla: failed to allocate subquery pointer"))
		return q
	}
	defer pl.free.Call(pl.store, stderrlenptr, 4, 1)

	ch := make(chan error, 2)
	var ret int32
	go func() {
		defer func() {
			if ex := recover(); ex != nil {
				ch <- fmt.Errorf("trealla: panic: %v", ex)
			}
		}()
		v, err := pl.pl_query_captured.Call(pl.store, pl.ptr, goalstr.ptr, subqptr,
			stdoutpp, stdoutlenptr, stderrpp, stderrlenptr, 0)
		if err == nil {
			ret = v.(int32)
		}
		goalstr.free(pl)
		ch <- err
	}()

	select {
	case <-ctx.Done():
		q.Close()
		q.setError(fmt.Errorf("trealla: canceled: %w", ctx.Err()))
		return q

	case err := <-ch:
		q.done = ret == 0
		if err != nil {
			q.setError(fmt.Errorf("trealla: query error: %w", err))
			return q
		}

		// grab subquery pointer
		if !q.done {
			var err error
			q.subquery, err = pl.indirect(subqptr)
			if err != nil {
				q.setError(fmt.Errorf("trealla: couldn't read subquery pointer: %w", err))
				return q
			}
		}

		stdoutlen, err := pl.indirect(stdoutlenptr)
		if err != nil {
			q.setError(fmt.Errorf("trealla: couldn't read stdout pointer: %w", err))
			return q
		}
		stdoutptr, err := pl.indirect(stdoutpp)
		if err != nil {
			q.setError(fmt.Errorf("trealla: couldn't read subquery pointer: %w", err))
			return q
		}
		defer pl.free.Call(pl.store, stdoutptr, stdoutlen, 1)
		stdout, err := pl.gets(stdoutptr, stdoutlen)
		if err != nil {
			panic(err)
		}

		stderrlen, err := pl.indirect(stderrlenptr)
		if err != nil {
			q.setError(fmt.Errorf("trealla: couldn't read stderr pointer: %w", err))
			return q
		}
		stderrptr, err := pl.indirect(stderrpp)
		if err != nil {
			q.setError(fmt.Errorf("trealla: couldn't read subquery pointer: %w", err))
			return q
		}
		defer pl.free.Call(pl.store, stderrptr, stderrlen, 1)
		stderr, err := pl.gets(stderrptr, stderrlen)
		if err != nil {
			panic(err)
		}

		if pl.closing {
			pl.Close()
		}
		ans, err := pl.parse(q.goal, string(stdout), stderr)
		if err == nil {
			q.push(ans)
		} else {
			q.setError(err)
		}
		return q
	}
}

func (q *query) redo(ctx context.Context) bool {
	if q.lock {
		q.pl.mu.Lock()
		defer q.pl.mu.Unlock()
	}
	if q.pl.instance == nil {
		q.setError(io.EOF)
		return false
	}

	if q.pl.debug != nil {
		q.pl.debug.Println("redo:", q.subquery, q.goal)
	}

	pl := q.pl

	// stdout string pointer
	stdoutptrv, err := pl.realloc.Call(pl.store, 0, 0, 1, 4)
	if err != nil {
		q.setError(fmt.Errorf("trealla: allocation error: %w", err))
		return false
	}
	stdoutpp := stdoutptrv.(int32)
	if stdoutpp == 0 {
		q.setError(fmt.Errorf("trealla: failed to allocate subquery pointer"))
		return false
	}
	defer pl.free.Call(pl.store, stdoutpp, 4, 1)

	// stdout size pointer
	stdoutlenptrv, err := pl.realloc.Call(pl.store, 0, 0, 1, 4)
	if err != nil {
		q.setError(fmt.Errorf("trealla: allocation error: %w", err))
		return false
	}
	stdoutlenptr := stdoutlenptrv.(int32)
	if stdoutlenptr == 0 {
		q.setError(fmt.Errorf("trealla: failed to allocate subquery pointer"))
		return false
	}
	defer pl.free.Call(pl.store, stdoutlenptr, 4, 1)

	// stderr string pointer
	stderrptrv, err := pl.realloc.Call(pl.store, 0, 0, 1, 4)
	if err != nil {
		q.setError(fmt.Errorf("trealla: allocation error: %w", err))
		return false
	}
	stderrpp := stderrptrv.(int32)
	if stderrpp == 0 {
		q.setError(fmt.Errorf("trealla: failed to allocate subquery pointer"))
		return false
	}
	defer pl.free.Call(pl.store, stderrpp, 4, 1)

	// stderr size pointer
	stderrlenptrv, err := pl.realloc.Call(pl.store, 0, 0, 1, 4)
	if err != nil {
		q.setError(fmt.Errorf("trealla: allocation error: %w", err))
		return false
	}
	stderrlenptr := stderrlenptrv.(int32)
	if stderrlenptr == 0 {
		q.setError(fmt.Errorf("trealla: failed to allocate subquery pointer"))
		return false
	}
	defer pl.free.Call(pl.store, stderrlenptr, 4, 1)

	ch := make(chan error, 2)
	var ret int32
	go func() {
		defer func() {
			if ex := recover(); ex != nil {
				ch <- fmt.Errorf("trealla: panic: %v", ex)
			}
		}()
		v, err := pl.pl_redo_captured.Call(pl.store, q.subquery, stdoutpp, stdoutlenptr, stderrpp, stderrlenptr)
		if err == nil {
			ret = v.(int32)
		}
		ch <- err
	}()

	select {
	case <-ctx.Done():
		q.setError(fmt.Errorf("trealla: canceled: %w", ctx.Err()))
		q.Close()
		return false

	case err := <-ch:
		q.done = ret == 0
		if err != nil {
			q.setError(fmt.Errorf("trealla: query error: %w", err))
			return false
		}

		stdoutlen, err := pl.indirect(stdoutlenptr)
		if err != nil {
			q.setError(fmt.Errorf("trealla: couldn't read stdout pointer: %w", err))
			return false
		}
		stdoutptr, err := pl.indirect(stdoutpp)
		if err != nil {
			q.setError(fmt.Errorf("trealla: couldn't read subquery pointer: %w", err))
			return false
		}
		defer pl.free.Call(pl.store, stdoutptr, stdoutlen, 1)
		stdout, err := pl.gets(stdoutptr, stdoutlen)
		if err != nil {
			panic(err)
		}

		stderrlen, err := pl.indirect(stderrlenptr)
		if err != nil {
			q.setError(fmt.Errorf("trealla: couldn't read stderr pointer: %w", err))
			return false
		}
		stderrptr, err := pl.indirect(stderrpp)
		if err != nil {
			q.setError(fmt.Errorf("trealla: couldn't read subquery pointer: %w", err))
			return false
		}
		defer pl.free.Call(pl.store, stderrptr, stderrlen, 1)
		stderr, err := pl.gets(stderrptr, stderrlen)
		if err != nil {
			panic(err)
		}

		if pl.closing {
			pl.Close()
		}
		ans, err := pl.parse(q.goal, stdout, stderr)
		switch {
		case IsFailure(err):
			return false
		case err != nil:
			q.setError(err)
			return false
		}
		q.push(ans)
		return true
	}
}

func (q *query) Next(ctx context.Context) bool {
	q.mu.Lock()
	defer q.mu.Unlock()

	if q.err != nil {
		return false
	}

	if q.pop() {
		return true
	}

	if q.done {
		return false
	}

	if q.redo(ctx) {
		return q.pop()
	}

	return false
}

func (q *query) push(a Answer) {
	q.next = &a
}

func (q *query) pop() bool {
	if q.next == nil {
		return false
	}
	q.cur = *q.next
	q.next = nil
	return true
}

func (q *query) Current() Answer {
	q.mu.Lock()
	defer q.mu.Unlock()
	return q.cur
}

func (q *query) Close() error {
	q.mu.Lock()
	defer q.mu.Unlock()

	if !q.done && q.subquery != 0 {
		if q.lock {
			q.pl.mu.Lock()
		}
		q.pl.pl_done.Call(q.pl.store, q.subquery)
		if q.lock {
			q.pl.mu.Unlock()
		}
	}

	q.done = true
	return nil
}

func (q *query) bindVar(name string, value Term) {
	for i, bind := range q.bind {
		if bind.name == name {
			bind.value = value
			q.bind[i] = bind
			return
		}
	}
	q.bind = append(q.bind, binding{
		name:  name,
		value: value,
	})
}

func (q *query) reify() error {
	if len(q.bind) == 0 {
		return nil
	}

	var sb strings.Builder
	sb.WriteString(q.bind.String())
	sb.WriteString(", ")
	sb.WriteString(q.goal)
	q.goal = sb.String()
	return nil
}

func (q *query) setError(err error) {
	if err != nil && q.err == nil {
		q.err = err
	}
}

func (q *query) Err() error {
	q.mu.Lock()
	defer q.mu.Unlock()
	return q.err
}

func escapeQuery(query string) string {
	query = queryEscaper.Replace(query)
	return fmt.Sprintf(`js_ask(%s).`, escapeString(query))
}

func finalize(q *query) {
	q.Close()
}

// QueryOption is an optional parameter for queries.
type QueryOption func(*query)

// WithBind binds the given variable to the given term.
// This can be handy for passing data into queries.
// `WithBind("X", "foo")` is equivalent to prepending `X = "foo",` to the query.
func WithBind(variable string, value Term) QueryOption {
	return func(q *query) {
		q.bindVar(variable, value)
	}
}

// WithBinding binds a map of variables to terms.
// This can be handy for passing data into queries.
func WithBinding(subs Substitution) QueryOption {
	return func(q *query) {
		for _, bind := range subs.bindings() {
			q.bindVar(bind.name, bind.value)
		}
	}
}

func withoutLock(q *query) {
	q.lock = false
}

var queryEscaper = strings.NewReplacer("\t", " ", "\n", " ", "\r", "")

var _ Query = (*query)(nil)
