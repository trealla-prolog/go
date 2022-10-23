package trealla

import (
	"encoding/binary"

	"github.com/wasmerio/wasmer-go/wasmer"
)

func (pl *prolog) exports() map[string]wasmer.IntoExtern {
	return map[string]wasmer.IntoExtern{
		"host-call":   pl.host_call(),
		"host-resume": pl.host_resume(),
	}
}

func (pl *prolog) host_call() *wasmer.Function {
	return wasmer.NewFunctionWithEnvironment(wasmStore,
		// extern int32_t host_call(int32_t subquery, const char *msg, size_t msg_size, char **reply, size_t *reply_size);
		wasmer.NewFunctionType(
			wasmer.NewValueTypes(wasmer.I32, wasmer.I32, wasmer.I32, wasmer.I32, wasmer.I32),
			wasmer.NewValueTypes(wasmer.I32),
		),
		pl, hostCall)
}

func (pl *prolog) host_resume() *wasmer.Function {
	return wasmer.NewFunctionWithEnvironment(wasmStore,
		// extern bool host_resume(int32_t subquery, char **reply, size_t *reply_size);
		wasmer.NewFunctionType(
			wasmer.NewValueTypes(wasmer.I32, wasmer.I32, wasmer.I32),
			wasmer.NewValueTypes(wasmer.I32),
		), pl, hostResume)
}

type procedure func(pl *prolog, subquery int32, arg Term) Term

func hostCall(env any, args []wasmer.Value) ([]wasmer.Value, error) {
	pl := env.(*prolog)
	_ = pl
	subquery := args[0].I32()
	msgptr := args[1].I32()
	msgsize := args[2].I32()
	reply_pp := args[3].I32()
	replysize_p := args[4].I32()

	msgraw, err := pl.gets(msgptr, msgsize)
	if err != nil {
		return nil, err
	}

	msg, err := unmarshalTerm([]byte(msgraw))
	if err != nil {
		return nil, err
	}

	memory := pl.memory.Data()
	reply := func(str string) error {
		msg, err := newCString(pl, str)
		if err != nil {
			return err
		}
		binary.LittleEndian.PutUint32(memory[reply_pp:], uint32(msg.ptr))
		binary.LittleEndian.PutUint32(memory[replysize_p:], uint32(msg.size-1))
		return nil
	}

	goal, ok := msg.(atomic)
	if !ok {
		expr := typeError("atomic", msg, piTerm("$host_call", 2))
		if err := reply(expr.String()); err != nil {
			return nil, err
		}
		return []wasmer.Value{wasm_true}, nil
	}

	proc, ok := pl.procs[goal.Indicator()]
	if !ok {
		expr := Atom("throw").Of(
			Atom("error").Of(
				Atom("existence_error").Of(
					Atom("procedure"), goal.pi(),
				),
				piTerm("$host_call", 2),
			))
		if err := reply(expr.String()); err != nil {
			return nil, err
		}
		return []wasmer.Value{wasm_true}, nil
	}

	continuation := proc(pl, subquery, goal)
	expr, err := marshal(continuation)
	if err != nil {
		return nil, err
	}
	if err := reply(expr); err != nil {
		return nil, err
	}
	return []wasmer.Value{wasm_true}, nil
}

func hostResume(_ any, args []wasmer.Value) ([]wasmer.Value, error) {
	return []wasmer.Value{wasm_false}, nil
}
