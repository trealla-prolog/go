package trealla

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
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

	queue []Answer
	cur   Answer
	err   error
	done  bool

	mu *sync.Mutex
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
	pl.mu.Lock()
	defer pl.mu.Unlock()

	q := &query{
		pl:   pl,
		goal: goal,
		mu:   new(sync.Mutex),
	}

	for _, opt := range options {
		opt(q)
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

	subqptrv, err := pl.realloc(0, 0, 1, 4)
	if err != nil {
		q.setError(err)
		return q
	}
	subqptr := subqptrv.(int32)
	defer pl.free(subqptr, 4, 1)

	ch := make(chan error, 2)
	var ret int32
	go func() {
		defer func() {
			if ex := recover(); ex != nil {
				ch <- fmt.Errorf("trealla: panic: %v", ex)
			}
		}()
		v, err := pl.pl_query(pl.ptr, goalstr.ptr, subqptr)
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
			q.setError(err)
			return q
		}

		// grab subquery pointer
		buf := bytes.NewBuffer(pl.memory.Data()[subqptr : subqptr+4])
		if err := binary.Read(buf, binary.LittleEndian, &q.subquery); err != nil {
			q.setError(fmt.Errorf("trealla: couldn't read subquery pointer: %w", err))
			return q
		}

		stdout := string(pl.wasi.ReadStdout())
		stderr := string(pl.wasi.ReadStderr())
		ans, err := newAnswer(q.goal, stdout, stderr)
		if err == nil {
			q.push(ans)
		} else {
			q.setError(err)
		}
		return q
	}
}

func (q *query) redo(ctx context.Context) bool {
	q.pl.mu.Lock()
	defer q.pl.mu.Unlock()

	pl := q.pl
	ch := make(chan error, 2)
	var ret int32
	go func() {
		defer func() {
			if ex := recover(); ex != nil {
				ch <- fmt.Errorf("trealla: panic: %v", ex)
			}
		}()
		v, err := pl.pl_redo(q.subquery)
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
			q.setError(err)
			return false
		}

		stdout := string(pl.wasi.ReadStdout())
		stderr := string(pl.wasi.ReadStderr())
		ans, err := newAnswer(q.goal, stdout, stderr)
		switch {
		case errors.Is(err, ErrFailure):
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
	q.queue = append(q.queue, a)
}

func (q *query) pop() bool {
	if len(q.queue) == 0 {
		return false
	}
	q.cur = q.queue[0]
	q.queue = q.queue[1:]
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
		q.pl.mu.Lock()
		q.pl.pl_done(q.subquery)
		q.pl.mu.Unlock()
		q.done = true
	}
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
	return fmt.Sprintf(`js_ask(%s)`, escapeString(query))
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

var _ Query = (*query)(nil)
