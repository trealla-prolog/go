package trealla

import (
	"context"
	"errors"
	"iter"
	"log"
	"reflect"
	"testing"
)

func TestInterop(t *testing.T) {
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

		// clone will have its own stack, making reentrancy less scary
		clone, err := pl.Clone()
		if err != nil {
			t.Error(err)
			return throwTerm(systemError(err.Error()))
		}

		ans1, err := clone.QueryOnce(ctx, "X is 1 + 1.")
		if err != nil {
			t.Error(err)
			return throwTerm(systemError(err.Error()))
		}

		ans2, err := clone.QueryOnce(ctx, "Y is X + 1.", WithBind("X", ans1.Solution["X"]))
		if err != nil {
			t.Error(err)
			return throwTerm(systemError(err.Error()))
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
					Query:    `http_consult(fizzbuzz:"https://raw.githubusercontent.com/guregu/worker-prolog/978c956801ffff83f190450e5c0325a9d34b064a/src/views/examples/fizzbuzz.pl"), fizzbuzz:fizzbuzz(1, 21), !`,
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
		// TODO: these (used to be) flakey on Linux
		// seems to be concurrency causing too much wasm stack to be used
		// cloning the pl instance "fixes" it
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

func TestInteropNondet(t *testing.T) {
	pred := func(pl Prolog, subquery Subquery, goal Term) iter.Seq[Term] {
		return func(yield func(Term) bool) {
			g := goal.(Compound)
			n, ok := goal.(Compound).Args[0].(int64)
			if !ok {
				yield(throwTerm(typeError("integer", goal.(Compound).Args[0], goal.(atomicTerm).pi())))
				return
			}
			for i := int64(0); i < n; i++ {
				g.Args[1] = i
				if !yield(g) {
					break
				}
			}
		}
	}
	ctx := context.Background()
	pl, err := New(WithDebugLog(log.Default()))
	if err != nil {
		t.Fatal(err)
	}
	pl.RegisterNondet(ctx, "countdown", 2, pred)

	t.Run("success", func(t *testing.T) {
		q := pl.Query(ctx, "countdown(10, X)")
		var n int64
		for answer := range q.All(ctx) {
			t.Logf("got: %v", answer)
			if answer.Solution["X"] != n {
				t.Error("unexpected solution:", answer)
			}
			n++
		}
		if n != 10 {
			t.Error("not enough iterations")
		}
		if q.Err() != nil {
			t.Error(q.Err())
		}
	})

	t.Run("bad arg", func(t *testing.T) {
		q := pl.Query(ctx, "countdown(foobar, X)")
		for q.Next(ctx) {
		}
		if q.Err() == nil {
			t.Error("expected error")
		}
	})

	t.Run("abandoned", func(t *testing.T) {
		q := pl.Query(ctx, "countdown(10, X)")
		if !q.Next(ctx) {
			t.Fatal("expected success")
		}
		if err := q.Close(); err != nil {
			t.Fatal(err)
		}
		if q.Err() != nil {
			t.Error(err)
		}
	})

	if leftovers := len(pl.(*prolog).coros); leftovers > 0 {
		t.Error("coroutines weren't cleaned up:", leftovers)
	}
}

func BenchmarkInteropNondet(b *testing.B) {
	pred := func(pl Prolog, subquery Subquery, goal Term) iter.Seq[Term] {
		return func(yield func(Term) bool) {
			for i := 0; ; i++ {
				goal.(Compound).Args[0] = i
				if !yield(goal) {
					break
				}
			}
		}
	}
	ctx := context.Background()
	pl, err := New()
	if err != nil {
		b.Fatal(err)
	}
	pl.RegisterNondet(ctx, "churn", 1, pred)
	query := pl.Query(ctx, "churn(X).")
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if !query.Next(ctx) {
			b.Fatal("query failed", query.Err())
		}
	}
	if err := query.Close(); err != nil {
		b.Error("close error:", err)
	}
	if leftovers := len(pl.(*prolog).coros); leftovers > 0 {
		b.Error("coroutines weren't cleaned up:", leftovers)
	}
}
