package trealla

import (
	"context"
	"encoding/base32"
	"errors"
	"fmt"
	"log"
	"reflect"
	"testing"
)

func TestInterop(t *testing.T) {
	pl, err := New(WithDebugLog(log.Default()) /*WithStderrLog(log.Default()), WithStdoutLog(log.Default()), WithTrace() */)
	if err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()
	pl.Register(ctx, "interop_test", 1, func(pl Prolog, _ Subquery, goal Term) Term {
		want := Atom("interop_test").Of(Variable{Name: "A"})
		if !reflect.DeepEqual(want, goal) {
			t.Error("bad goal. want:", want, "got:", goal)
		}

		ans, err := pl.QueryOnce(ctx, "X is 1 + 1.")
		if err != nil {
			t.Error(err)
		}
		return Atom("interop_test").Of(ans.Solution["X"])
	})

	tests := []struct {
		name string
		want []Answer
		err  error
	}{
		{
			name: "crypto_data_hash/3",
			want: []Answer{
				{
					Query:    `crypto_data_hash("foo", X, [algorithm(A)]).`,
					Solution: Substitution{"A": Atom("sha256"), "X": "2c26b46b68ffc68ff99b453c1d30413413422d706483bfa0f98a5e886266e7ae"},
				},
			},
		},
		{
			name: "http_consult/1",
			want: []Answer{
				{
					Query:    `http_consult("https://raw.githubusercontent.com/guregu/worker-prolog/978c956801ffff83f190450e5c0325a9d34b064a/src/views/examples/fizzbuzz.pl"), !, fizzbuzz(1, 21), !`,
					Solution: Substitution{},
					Stdout:   "1\n2\nfizz\n4\nbuzz\nfizz\n7\n8\nfizz\nbuzz\n11\nfizz\n13\n14\nfizzbuzz\n16\n17\nfizz\n19\nbuzz\nfizz\n",
				},
			},
		},
		{
			name: "custom function",
			want: []Answer{
				{
					Query:    `interop_test(X).`,
					Solution: Substitution{"X": int64(2)},
				},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			q := pl.Query(ctx, tc.want[0].Query)
			var ans []Answer
			for q.Next(ctx) {
				ans = append(ans, q.Current())
			}
			err := q.Err()
			if tc.err == nil && err != nil {
				t.Fatal(err)
			} else if tc.err != nil && !errors.Is(err, tc.err) {
				t.Errorf("unexpected error: %#v (%v) ", err, err)
			}
			if tc.err == nil && !reflect.DeepEqual(ans, tc.want) {
				t.Errorf("bad answer. \nwant: %#v\ngot: %#v\n", tc.want, ans)
			}
		})
	}
}

func Example_register() {
	ctx := context.Background()
	pl, err := New()
	if err != nil {
		panic(err)
	}

	// Let's add a base32 encoding predicate.
	// To keep it brief, this only handles one mode.
	// base32(+Input, -Output) is det.
	pl.Register(ctx, "base32", 2, func(_ Prolog, _ Subquery, goal0 Term) Term {
		// goal is the goal called by Prolog, such as: base32("hello", X).
		// Guaranteed to match up with the registered arity and name.
		goal := goal0.(Compound)

		// Check the Input argument's type, must be string.
		input, ok := goal.Args[0].(string)
		if !ok {
			// throw(error(type_error(list, X), base32/2)).
			return Atom("throw").Of(Atom("error").Of(
				Atom("type_error").Of("list", goal.Args[0]),
				Atom("/").Of(Atom("base32"), 2),
			))
		}

		// Check Output type, must be string or var.
		switch goal.Args[1].(type) {
		case string: // ok
		case Variable: // ok
		default:
			// throw(error(type_error(list, X), base32/2)).
			return Atom("throw").Of(Atom("error").Of(
				Atom("type_error").Of("list", goal.Args[0]),
				Atom("/").Of(Atom("base32"), 2),
			))
		}

		// Do the encoding actual work.
		output := base32.StdEncoding.EncodeToString([]byte(input))

		// Return a goal that Trealla will unify with its input:
		// base32(Input, "output_goes_here").
		return Atom("base32").Of(input, output)
	})

	// Try it out.
	answer, err := pl.QueryOnce(ctx, `base32("hello", Encoded).`)
	if err != nil {
		panic(err)
	}
	fmt.Println(answer.Solution["Encoded"])
	// Output: NBSWY3DP
}
