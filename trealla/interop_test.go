package trealla

import (
	"context"
	"errors"
	"flag"
	"log"
	"reflect"
	"testing"
)

var skipInterop = flag.Bool("skipinterop", false, "skip interop test")

func TestInterop(t *testing.T) {
	if *skipInterop {
		t.Skip("skipping interop tests")
	}

	pl, err := New(WithDebugLog(log.Default()))

	if err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()
	pl.Register(ctx, "interop_test", 1, func(pl Prolog, _ Subquery, goal Term) Term {
		want := Atom("interop_test").Of(Variable{Name: "A"})
		if !reflect.DeepEqual(want, goal) {
			t.Error("bad goal. want:", want, "got:", goal)
		}

		ans1, err := pl.QueryOnce(ctx, "X is 1 + 1.")
		if err != nil {
			t.Error(err)
		}

		ans2, err := pl.QueryOnce(ctx, "Y is X + 1.", WithBind("X", ans1.Solution["X"]))
		if err != nil {
			t.Error(err)
		}

		return Atom("interop_test").Of(ans2.Solution["Y"])
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
					Solution: Substitution{"X": int64(3)},
				},
			},
		},
		// {
		// 	name: "http_fetch/3",
		// 	want: []Answer{
		// 		{
		// 			Query:    `http_fetch("https://jsonplaceholder.typicode.com/todos/1", Result, [as(json)]).`,
		// 			Solution: Substitution{"Result": Compound{Functor: "{}", Args: []Term{Compound{Functor: ",", Args: []Term{Compound{Functor: ":", Args: []Term{"userId", int64(1)}}, Compound{Functor: ",", Args: []Term{Compound{Functor: ":", Args: []Term{"id", int64(1)}}, Compound{Functor: ",", Args: []Term{Compound{Functor: ":", Args: []Term{"title", "delectus aut autem"}}, Compound{Functor: ":", Args: []Term{"completed", "false"}}}}}}}}}}},
		// 		},
		// 	},
		// },
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			q := pl.Query(ctx, tc.want[0].Query)
			var ans []Answer
			for q.Next(ctx) {
				ans = append(ans, q.Current())
			}
			q.Close()
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
