package trealla

import (
	"context"
	"fmt"
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

// Subquery is an opaque value representing an in-flight query.
// It is unique as long as the query is alive, but may be re-used later on.
type Subquery uint32

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
	continuation := proc(locked, Subquery(subquery), goal)
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
	if _, err := pl.pl_capture.Call(pl.ctx, uint64(pl.ptr)); err != nil {
		panic(err)
	}

	return wasmTrue
}

func hostResume(_, _, _ uint32) uint32 {
	// extern int32_t host_resume(int32_t subquery, char **reply, size_t *reply_size);
	return wasmFalse
}
