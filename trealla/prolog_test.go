package trealla

import (
	"context"
	"io"
	"reflect"
	"testing"
)

func TestClose(t *testing.T) {
	pl, err := New()
	if err != nil {
		t.Fatal(err)
	}
	pl.Close()
	_, err = pl.QueryOnce(context.Background(), "true")
	if err != io.EOF {
		t.Error("unexpected error", err)
	}
}

func TestClone(t *testing.T) {
	pl, err := New()
	if err != nil {
		t.Fatal(err)
	}
	if err := pl.ConsultText(context.Background(), "user", `abc("xyz").`); err != nil {
		t.Fatal(err)
	}
	clone, err := pl.Clone()
	if err != nil {
		t.Fatal(err)
	}
	ans, err := clone.QueryOnce(context.Background(), "abc(X).")
	if err != nil {
		t.Fatal(err)
	}
	want := Term("xyz")
	got := ans.Solution["X"]
	if want != got {
		t.Error("want:", want, "got:", got)
	}
	t.Log(ans)

	if err := pl.ConsultText(context.Background(), "user", `foo(bar).`); err != nil {
		t.Error(err)
	}

	_, err = clone.QueryOnce(context.Background(), "foo(X).")
	if err == nil {
		t.Error("expected error, got:", err)
	}
	if _, ok := err.(ErrThrow); !ok {
		t.Error("expected throw, got:", err)
	}
}

func TestLeakCheck(t *testing.T) {
	check := func(goal string, limit int) func(t *testing.T) {
		return func(t *testing.T) {
			t.Parallel()
			ctx := context.Background()

			pl, err := New()
			if err != nil {
				t.Fatal(err)
			}
			defer pl.Close()

			pl.Register(ctx, "interop_simple", 1, func(pl Prolog, subquery Subquery, goal Term) Term {
				return Atom("interop_simple").Of(int64(42))
			})
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
			size := 0
			for i := 0; i < 2048; i++ {
				q := pl.Query(ctx, goal)
				n := 0
				for q.Next(ctx) {
					n++
					if limit > 0 && n >= limit {
						break
					}
				}
				if err := q.Err(); err != nil && !IsFailure(err) {
					t.Fatal(err, "iter=", i)
				}
				q.Close()

				current := pl.Stats().MemorySize
				if size == 0 {
					size = current
				}
				if current > size {
					t.Fatal(goal, "possible leak: memory grew to:", current, "initial:", size)
				}
			}
			t.Logf("goal: %s size: %d", goal, size)
		}
	}
	t.Run("true", check("true.", 0))
	t.Run("between(1,3,X)", check("between(1,3,X).", 0))
	t.Run("between(1,3,X) limit 1", check("between(1,3,X).", 1))

	// BUG(guregu): queries ending in a ; fail branch leak for some reason
	// but it's not enough to trigger the leak check (~20B/query)
	t.Run("failing branch", check("true ; true ; true ; foo=bar.", 0))

	t.Run("fail", check("fail ; fail", 0))

	t.Run("simple interop", check("interop_simple(X)", 0))
	// t.Run("complex interop", check("interop_test(X)"))
}
