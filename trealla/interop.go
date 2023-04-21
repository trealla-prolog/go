package trealla

import (
	"encoding/binary"

	"github.com/bytecodealliance/wasmtime-go/v8"
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
type Subquery int32

// func (pl *prolog) exports() map[string]wasmer.IntoExtern {
// 	return map[string]wasmer.IntoExtern{
// 		"host-call":   pl.host_call(),
// 		"host-resume": pl.host_resume(),
// 	}
// }

// func (pl *prolog) exports() []*wasmtime.ImportType {

// 	// return []*wasmtime.ImportType{
// 	// 	wasmtime.NewImportType("trealla", "host-call", pl.host_call()),
// 	// }
// 	// return map[string]wasmtime.ImportType{
// 	// 	wasmtime.NewImportType(),
// 	// 	"host-call":   pl.host_call(),
// 	// 	"host-resume": pl.host_resume(),
// 	// }
// }

// func (pl *prolog) host_call() *wasmtime.FuncType {
// 	// extern int32_t host_call(int32_t subquery, const char *msg, size_t msg_size, char **reply, size_t *reply_size);
// 	return wasmtime.NewFuncType([]*wasmtime.ValType{
// 		wasmtime.NewValType(wasmtime.KindI32),
// 		wasmtime.NewValType(wasmtime.KindI32),
// 		wasmtime.NewValType(wasmtime.KindI32),
// 		wasmtime.NewValType(wasmtime.KindI32),
// 		wasmtime.NewValType(wasmtime.KindI32),
// 	}, []*wasmtime.ValType{
// 		wasmtime.NewValType(wasmtime.KindI32),
// 	})
// }

// func (pl *prolog) host_resume() *wasmtime.FuncType {
// 	return wasmtime.NewFuncType([]*wasmtime.ValType{
// 		wasmtime.NewValType(wasmtime.KindI32),
// 		wasmtime.NewValType(wasmtime.KindI32),
// 		wasmtime.NewValType(wasmtime.KindI32),
// 		wasmtime.NewValType(wasmtime.KindI32),
// 		wasmtime.NewValType(wasmtime.KindI32),
// 	}, []*wasmtime.ValType{
// 		wasmtime.NewValType(wasmtime.KindI32),
// 	})
// 	return wasmer.NewFunctionWithEnvironment(wasmStore,
// 		// extern bool host_resume(int32_t subquery, char **reply, size_t *reply_size);
// 		wasmer.NewFunctionType(
// 			wasmer.NewValueTypes(wasmer.I32, wasmer.I32, wasmer.I32),
// 			wasmer.NewValueTypes(wasmer.I32),
// 		), pl, hostResume)
// }

func (pl *prolog) hostCall( /*c *wasmtime.Caller,*/ subquery, msgptr, msgsize, reply_pp, replysize_p int32) (int32, *wasmtime.Trap) {
	// subquery := args[0].I32()
	// msgptr := args[1].I32()
	// msgsize := args[2].I32()
	// reply_pp := args[3].I32()
	// replysize_p := args[4].I32()

	msgraw, err := pl.gets(msgptr, msgsize)
	if err != nil {
		return 0, wasmtime.NewTrap(err.Error())
	}

	msg, err := unmarshalTerm([]byte(msgraw))
	if err != nil {
		return 0, wasmtime.NewTrap(err.Error())
	}

	memory := pl.memory.UnsafeData(pl.store)
	reply := func(str string) error {
		msg, err := newCString(pl, str)
		if err != nil {
			return err
		}
		binary.LittleEndian.PutUint32(memory[uint32(reply_pp):], uint32(msg.ptr))
		binary.LittleEndian.PutUint32(memory[uint32(replysize_p):], uint32(msg.size-1))
		return nil
	}

	goal, ok := msg.(atomic)
	if !ok {
		expr := typeError("atomic", msg, piTerm("$host_call", 2))
		if err := reply(expr.String()); err != nil {
			return 0, wasmtime.NewTrap(err.Error())
		}
		return wasmTrue, nil
	}

	proc, ok := pl.procs[goal.Indicator()]
	if !ok {
		expr := Atom("throw").Of(
			Atom("error").Of(
				Atom("existence_error").Of(Atom("procedure"), goal.pi()),
				piTerm("$host_call", 2),
			))
		if err := reply(expr.String()); err != nil {
			return 0, wasmtime.NewTrap(err.Error())
		}
		return wasmTrue, nil
	}

	locked := &lockedProlog{prolog: pl}
	continuation := proc(locked, Subquery(subquery), goal)
	locked.kill()
	expr, err := marshal(continuation)
	if err != nil {
		return 0, wasmtime.NewTrap(err.Error())
	}
	if err := reply(expr); err != nil {
		return 0, wasmtime.NewTrap(err.Error())
	}
	return wasmTrue, nil
}

func hostResume(_, _, _ int32) (int32, *wasmtime.Trap) {
	// extern int32_t host_resume(int32_t subquery, char **reply, size_t *reply_size);
	return wasmFalse, nil
}
