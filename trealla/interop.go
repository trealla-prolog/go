package trealla

import (
	"context"
	"fmt"
	"io"
	"iter"
)

// Predicate is a Prolog predicate implemented in Go.
// subquery is an opaque number representing the current query.
// goal is the goal called, which includes the arguments.
//
// Return value meaning:
//   - By default, the term returned will be unified with the goal.
//   - Return a throw/1 compound to throw instead.
//   - Return a call/1 compound to call a different goal instead.
//   - Return a 'fail' atom to fail instead.
//   - Return a 'true' atom to succeed without unifying anything.
type Predicate func(pl Prolog, subquery Subquery, goal Term) Term

// NondetPredicate works similarly to [Predicate], but can create multiple choice points.
type NondetPredicate func(pl Prolog, subquery Subquery, goal Term) iter.Seq[Term]

// Subquery is an opaque value representing an in-flight query.
// It is unique as long as the query is alive, but may be re-used later on.
type Subquery uint32

type coroutine struct {
	next func() (Term, bool)
	stop func()
}

type coroer interface {
	CoroStart(subq Subquery, seq iter.Seq[Term]) int64
	CoroNext(subq Subquery, id int64) (Term, bool)
	CoroStop(subq Subquery, id int64)
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

func (pl *prolog) RegisterNondet(ctx context.Context, name string, arity int, proc NondetPredicate) error {
	pl.mu.Lock()
	defer pl.mu.Unlock()
	if pl.instance == nil {
		return io.EOF
	}
	return pl.registerNondet(ctx, name, arity, proc)
}

func (pl *prolog) registerNondet(ctx context.Context, name string, arity int, proc NondetPredicate) error {
	shim := func(pl2 Prolog, subquery Subquery, goal Term) Term {
		plc := pl2.(coroer)
		seq := proc(pl2, subquery, goal)
		id := plc.CoroStart(subquery, seq)
		// call: call_cleanup('$coro_next'(ID), '$coro_stop'(ID))
		return Atom("call").Of(
			Atom("call_cleanup").Of(
				Atom("$coro_next").Of(id, goal),
				Atom("$coro_stop").Of(id),
			),
		)
	}
	return pl.register(ctx, name, arity, shim)
}

// '$coro_next'(+ID, ?Goal)
func sys_coro_next_2(pl Prolog, subquery Subquery, goal Term) Term {
	plc := pl.(coroer)
	g := goal.(Compound)
	id, ok := g.Args[0].(int64)
	if !ok {
		return throwTerm(domainError("integer", g.Args[0], g.pi()))
	}
	t, ok := plc.CoroNext(subquery, id)
	if !ok || t == nil {
		return Atom("fail")
	}
	// call(( Goal = Result ; '$coro_next'(ID, Goal) ))
	return Atom("call").Of(
		Atom(";").Of(
			Atom("=").Of(g.Args[1], t),
			Atom("$coro_next").Of(id, g.Args[1]),
		),
	)
}

// '$coro_stop'(+ID)
func sys_coro_stop_1(pl Prolog, subquery Subquery, goal Term) Term {
	plc := pl.(coroer)
	g := goal.(Compound)
	id, ok := g.Args[0].(int64)
	if !ok {
		return throwTerm(domainError("integer", g.Args[0], g.pi()))
	}
	plc.CoroStop(subquery, id)
	return goal
}

func (pl *prolog) CoroStart(subq Subquery, seq iter.Seq[Term]) int64 {
	pl.coron++
	id := pl.coron
	next, stop := iter.Pull(seq)
	pl.coros[id] = coroutine{
		next: next,
		stop: stop,
	}
	if query := pl.subquery(uint32(subq)); query != nil {
		if query.coros == nil {
			query.coros = make(map[int64]struct{})
		}
		query.coros[id] = struct{}{}
	}
	return id
}

func (pl *prolog) CoroNext(subq Subquery, id int64) (Term, bool) {
	coro, ok := pl.coros[id]
	if !ok {
		return Atom("false"), false
	}
	next, ok := coro.next()
	if !ok {
		delete(pl.coros, id)
		if query := pl.subquery(uint32(subq)); query != nil {
			delete(query.coros, id)
		}
	}
	return next, ok
}

func (pl *prolog) CoroStop(subq Subquery, id int64) {
	if query := pl.subquery(uint32(subq)); query != nil {
		delete(query.coros, id)
	}
	coro, ok := pl.coros[id]
	if !ok {
		return
	}
	coro.stop()
	delete(pl.coros, id)
}

func (pl *lockedProlog) CoroStart(subq Subquery, seq iter.Seq[Term]) int64 {
	return pl.prolog.CoroStart(subq, seq)
}

func (pl *lockedProlog) CoroNext(subq Subquery, id int64) (Term, bool) {
	return pl.prolog.CoroNext(subq, id)
}

func (pl *lockedProlog) CoroStop(subq Subquery, id int64) {
	pl.prolog.CoroStop(subq, id)
}

func hostCall(ctx context.Context, subquery, msgptr, msgsize, reply_pp, replysize_p uint32) uint32 {
	// extern int32_t host_call(int32_t subquery, const char *msg, size_t msg_size, char **reply, size_t *reply_size);
	pl := ctx.Value(prologKey{}).(*prolog)

	subq := pl.subquery(subquery)
	if subq == nil {
		panic(fmt.Sprintf("could not find subquery: %d", subquery))
	}

	msgraw, err := pl.gets(msgptr, msgsize)
	if err != nil {
		panic(err)
	}

	msg, err := unmarshalTerm([]byte(msgraw))
	if err != nil {
		err = fmt.Errorf("%w (raw msg: %s)", err, msgraw)
		panic(err)
	}

	reply := func(str string) error {
		msg, err := newCString(pl, str)
		if err != nil {
			return err
		}
		pl.memory.WriteUint32Le(reply_pp, msg.ptr)
		pl.memory.WriteUint32Le(replysize_p, uint32(msg.size-1))
		return nil
	}

	goal, ok := msg.(atomicTerm)
	if !ok {
		expr := typeError("atomic", msg, piTerm("$host_call", 2))
		if err := reply(expr.String()); err != nil {
			panic(err)
		}
		return wasmTrue
	}

	proc, ok := pl.procs[goal.Indicator()]
	if !ok {
		expr := Atom("throw").Of(
			Atom("error").Of(
				Atom("existence_error").Of(Atom("procedure"), goal.pi()),
				piTerm("$host_call", 2),
			))
		if err := reply(expr.String()); err != nil {
			panic(err)
		}
		return wasmTrue
	}

	if err := subq.readOutput(); err != nil {
		panic(err)
	}
	// log.Println("SAVING", subq.stderr.String())

	locked := &lockedProlog{prolog: pl}
	continuation := catch(proc, locked, Subquery(subquery), goal)
	locked.kill()
	expr, err := marshal(continuation)
	if err != nil {
		panic(err)
	}
	if err := reply(expr); err != nil {
		panic(err)
	}

	if err := subq.readOutput(); err != nil {
		panic(err)
	}
	return wasmTrue
}

func catch(pred Predicate, pl Prolog, subq Subquery, goal Term) (result Term) {
	defer func() {
		if threw := recover(); threw != nil {
			switch ball := threw.(type) {
			case Atom:
				result = throwTerm(ball)
			case Compound:
				if ball.Functor == "throw" && len(ball.Args) == 1 {
					result = ball
				} else {
					result = throwTerm(ball)
				}
			default:
				result = throwTerm(
					Atom("system_error").Of(
						Atom("panic").Of(fmt.Sprint(threw)),
						goal.(atomicTerm).pi(),
					),
				)
			}
		}
	}()
	result = pred(pl, subq, goal)
	return
}

func hostResume(_, _, _ uint32) uint32 {
	// extern int32_t host_resume(int32_t subquery, char **reply, size_t *reply_size);
	return wasmFalse
}

var (
	_ coroer = (*prolog)(nil)
	_ coroer = (*lockedProlog)(nil)
)
