package trealla

import (
	"bytes"
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
	subquery int32 // pl_sub_query*

	cur  Answer
	next *Answer
	err  error
	done bool
	dead bool

	// output capture pointers
	stdoutptr int32 // char**
	stdoutlen int32 // size_t*
	stderrptr int32 // char**
	stderrlen int32 // size_t*

	stdout *bytes.Buffer
	stderr *bytes.Buffer

	lock bool
	mu   *sync.Mutex
}

// Query executes a query, returning an iterator for results.
func (pl *prolog) Query(ctx context.Context, goal string, options ...QueryOption) Query {
	q := pl.start(ctx, goal, options...)
	runtime.SetFinalizer(q, (*query).Close)
	return q
}

func (pl *prolog) QueryOnce(ctx context.Context, goal string, options ...QueryOption) (Answer, error) {
	pl.mu.Lock()
	defer pl.mu.Unlock()
	return pl.queryOnce(ctx, goal, options...)
}

func (pl *prolog) queryOnce(ctx context.Context, goal string, options ...QueryOption) (Answer, error) {
	options = append(options, withoutLock)
	q := pl.start(ctx, goal, options...)
	var ans Answer
	if q.Next(ctx) {
		ans = q.Current()
	}
	q.Close()
	return ans, q.Err()
}

func (q *query) allocCapture() error {
	pl := q.pl
	var err error
	if q.stdoutptr == 0 {
		q.stdoutptr, err = pl.alloc(ptrSize)
		if err != nil {
			return err
		}
	}
	if q.stdoutlen == 0 {
		q.stdoutlen, err = pl.alloc(ptrSize)
		if err != nil {
			return err
		}
	}
	if q.stderrptr == 0 {
		q.stderrptr, err = pl.alloc(ptrSize)
		if err != nil {
			return err
		}
	}
	if q.stderrlen == 0 {
		q.stderrlen, err = pl.alloc(ptrSize)
		if err != nil {
			return err
		}
	}
	return nil
}

func (q *query) readOutput() error {
	pl := q.pl
	var err error

	if err := q.allocCapture(); err != nil {
		return err
	}

	_, err = pl.pl_capture_read.Call(pl.store, pl.ptr, q.stdoutptr, q.stdoutlen, q.stderrptr, q.stderrlen)
	if err != nil {
		return err
	}

	stdoutlen := pl.indirect(q.stdoutlen)
	stdoutptr := pl.indirect(q.stdoutptr)
	stderrlen := pl.indirect(q.stderrlen)
	stderrptr := pl.indirect(q.stderrptr)

	stdout, err := pl.gets(stdoutptr, stdoutlen)
	if err != nil {
		return err
	}
	q.stdout.WriteString(stdout)

	stderr, err := pl.gets(stderrptr, stderrlen)
	if err != nil {
		return err
	}
	q.stderr.WriteString(stderr)

	pl.pl_capture_free.Call(pl.store, pl.ptr)

	return nil
}

func (q *query) resetOutput() {
	q.stdout.Reset()
	q.stderr.Reset()
}

func (pl *prolog) start(ctx context.Context, goal string, options ...QueryOption) *query {
	q := &query{
		pl:     pl,
		goal:   goal,
		lock:   true,
		stdout: new(bytes.Buffer),
		stderr: new(bytes.Buffer),
		mu:     new(sync.Mutex),
	}
	for _, opt := range options {
		opt(q)
	}

	if pl.limiter != nil {
		pl.limiter <- struct{}{}
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

	subqptr, err := pl.alloc(ptrSize)
	if err != nil {
		q.setError(fmt.Errorf("trealla: failed to allocate subquery pointer"))
		return q
	}
	pl.spawning[subqptr] = q
	defer func(ptr int32) {
		delete(pl.spawning, subqptr)
	}(subqptr)
	defer pl.free.Call(pl.store, subqptr, 4, 1)

	if err := q.allocCapture(); err != nil {
		q.setError(err)
		return q
	}

	ch := make(chan error, 2)
	var ret int32
	go func() {
		defer func() {
			if ex := recover(); ex != nil {
				ch <- fmt.Errorf("trealla: panic: %v", ex)
			}
		}()
		_, err := pl.pl_capture.Call(pl.store, pl.ptr)
		if err != nil {
			ch <- err
			return
		}

		v, err := pl.pl_query.Call(pl.store, pl.ptr, goalstr.ptr, subqptr, 0)
		if err == nil {
			ret = v.(int32)
		}
		goalstr.free(pl)
		ch <- err
	}()

	select {
	case <-ctx.Done():
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
			q.subquery = pl.indirect(subqptr)
			if q.subquery == 0 {
				q.setError(fmt.Errorf("trealla: couldn't read subquery pointer: %w", err))
				return q
			}
			q.pl.running[q.subquery] = q
		}

		if err := q.readOutput(); err != nil {
			q.setError(err)
			return q
		}

		if pl.closing {
			pl.Close()
		}

		stdout := q.stdout.String()
		stderr := q.stderr.String()
		q.resetOutput()

		ans, err := pl.parse(q.goal, stdout, stderr)
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

	ch := make(chan error, 2)
	var ret int32
	go func() {
		defer func() {
			if ex := recover(); ex != nil {
				ch <- fmt.Errorf("trealla: panic: %v", ex)
			}
		}()

		_, err := pl.pl_capture.Call(pl.store, pl.ptr)
		if err != nil {
			ch <- err
			return
		}

		v, err := pl.pl_redo.Call(pl.store, q.subquery)
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
			q.Close()
			return false
		}

		if q.done {
			delete(pl.running, q.subquery)
			// defer q.close()
		}

		if err := q.readOutput(); err != nil {
			q.setError(err)
			return false
		}

		if pl.closing {
			pl.Close()
		}

		stdout := q.stdout.String()
		stderr := q.stderr.String()
		q.resetOutput()

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

	if q.lock {
		q.pl.mu.Lock()
		defer q.pl.mu.Unlock()
	}

	return q.close()
}

func (q *query) close() error {
	if !q.dead {
		q.dead = true
		if q.pl.limiter != nil {
			defer func() {
				<-q.pl.limiter
			}()
		}
	}

	if q.subquery != 0 {
		delete(q.pl.running, q.subquery)
	}

	if !q.done && q.subquery != 0 {
		q.pl.pl_done.Call(q.pl.store, q.subquery)
		q.done = true
		q.subquery = 0
	}

	if q.stdoutptr != 0 {
		q.pl.free.Call(q.pl.store, q.stdoutptr, ptrSize, align)
		q.stdoutptr = 0
	}
	if q.stdoutlen != 0 {
		q.pl.free.Call(q.pl.store, q.stdoutlen, ptrSize, align)
		q.stdoutlen = 0
	}
	if q.stderrptr != 0 {
		q.pl.free.Call(q.pl.store, q.stderrptr, ptrSize, align)
		q.stderrptr = 0
	}
	if q.stderrlen != 0 {
		q.pl.free.Call(q.pl.store, q.stderrlen, ptrSize, align)
		q.stderrlen = 0
	}

	// q.pl = nil

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
