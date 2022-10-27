package trealla_test

import (
	"context"
	"errors"
	"log"
	"math/big"
	"os"
	"reflect"
	"testing"

	"github.com/trealla-prolog/go/trealla"
)

func TestQuery(t *testing.T) {
	testdata := "./testdata"
	if _, err := os.Stat(testdata); os.IsNotExist(err) {
		testdata = "./trealla/testdata"
	}

	pl, err := trealla.New(trealla.WithMapDir("testdata", testdata), trealla.WithLibraryPath("/testdata"), trealla.WithDebugLog(log.Default()))
	if err != nil {
		t.Fatal(err)
	}

	t.Run("consult", func(t *testing.T) {
		if err := pl.Consult(context.Background(), "/testdata/greeting.pl"); err != nil {
			t.Error(err)
		}
	})

	tests := []struct {
		name string
		want []trealla.Answer
		err  error
	}{
		{
			name: "true/0",
			want: []trealla.Answer{
				{
					Query:    `true.`,
					Solution: trealla.Substitution{},
				},
			},
		},
		{
			name: "false/0",
			want: []trealla.Answer{
				{
					Query: `false.`,
				},
			},
			err: trealla.ErrFailure{Query: "false."},
		},
		{
			name: "failure with output",
			want: []trealla.Answer{
				{
					Query: `write(foo), write(user_error, bar), fail.`,
				},
			},
			err: trealla.ErrFailure{
				Query:  "write(foo), write(user_error, bar), fail.",
				Stdout: "foo",
				Stderr: "bar",
			},
		},
		{
			name: "write to stdout",
			want: []trealla.Answer{
				{
					Query:    `write(hello), nl.`,
					Solution: trealla.Substitution{},
					Stdout:   "hello\n",
				},
			},
		},
		{
			name: "write to stderr",
			want: []trealla.Answer{
				{
					Query:    `write(user_error, hello).`,
					Solution: trealla.Substitution{},
					Stderr:   "hello",
				},
			},
		},
		{
			name: "consulted",
			want: []trealla.Answer{
				{
					Query: `hello(X).`,
					Solution: trealla.Substitution{
						"X": trealla.Atom("world"),
					},
				},
				{
					Query: `hello(X).`,
					Solution: trealla.Substitution{
						"X": trealla.Atom("Welt"),
					},
				},
				{
					Query: `hello(X).`,
					Solution: trealla.Substitution{
						"X": trealla.Atom("世界"),
					},
				},
			},
		},
		{
			name: "assertz/1",
			want: []trealla.Answer{
				{
					Query:    `assertz(こんにちは(世界)).`,
					Solution: trealla.Substitution{},
				},
			},
		},
		{
			name: "assertz/1 (did it persist?)",
			want: []trealla.Answer{
				{
					Query:    `こんにちは(X).`,
					Solution: trealla.Substitution{"X": trealla.Atom("世界")},
				},
			},
		},
		{
			name: "member/2",
			want: []trealla.Answer{
				{
					Query:    `member(X, [1,foo(bar),4.2,"baz",'boop', [q, '"x'], '\\', '\n']).`,
					Solution: trealla.Substitution{"X": int64(1)},
				},
				{
					Query:    `member(X, [1,foo(bar),4.2,"baz",'boop', [q, '"x'], '\\', '\n']).`,
					Solution: trealla.Substitution{"X": trealla.Compound{Functor: "foo", Args: []trealla.Term{trealla.Atom("bar")}}}},
				{
					Query:    `member(X, [1,foo(bar),4.2,"baz",'boop', [q, '"x'], '\\', '\n']).`,
					Solution: trealla.Substitution{"X": 4.2}},
				{
					Query:    `member(X, [1,foo(bar),4.2,"baz",'boop', [q, '"x'], '\\', '\n']).`,
					Solution: trealla.Substitution{"X": "baz"}},
				{
					Query:    `member(X, [1,foo(bar),4.2,"baz",'boop', [q, '"x'], '\\', '\n']).`,
					Solution: trealla.Substitution{"X": trealla.Atom("boop")}},
				{
					Query:    `member(X, [1,foo(bar),4.2,"baz",'boop', [q, '"x'], '\\', '\n']).`,
					Solution: trealla.Substitution{"X": []trealla.Term{trealla.Atom("q"), trealla.Atom(`"x`)}}},
				{
					Query:    `member(X, [1,foo(bar),4.2,"baz",'boop', [q, '"x'], '\\', '\n']).`,
					Solution: trealla.Substitution{"X": trealla.Atom(`\`)}},
				{
					Query:    `member(X, [1,foo(bar),4.2,"baz",'boop', [q, '"x'], '\\', '\n']).`,
					Solution: trealla.Substitution{"X": trealla.Atom("\n")}},
			},
		},
		{
			name: "tak & WithLibraryPath",
			want: []trealla.Answer{
				{
					Query:    "use_module(library(tak)), run.",
					Solution: trealla.Substitution{},
					Stdout:   "'<https://josd.github.io/eye/ns#tak>'([34,13,8],13).\n",
				},
			},
		},
		{
			name: "bigint",
			want: []trealla.Answer{
				{
					Query:    "X=9999999999999999, Y = -9999999999999999, Z = 123.",
					Solution: trealla.Substitution{"X": big.NewInt(9999999999999999), "Y": big.NewInt(-9999999999999999), "Z": int64(123)},
				},
			},
		},
		{
			name: "empty list",
			want: []trealla.Answer{
				{
					Query:    "X = [].",
					Solution: trealla.Substitution{"X": []trealla.Term{}},
				},
			},
		},
		{
			name: "empty atom",
			want: []trealla.Answer{
				{
					Query:    "X = foo(bar, '').",
					Solution: trealla.Substitution{"X": trealla.Compound{Functor: "foo", Args: []trealla.Term{trealla.Atom("bar"), trealla.Atom("")}}},
				},
			},
		},
		{
			name: "residual goals",
			want: []trealla.Answer{
				{
					Query: "dif(X, Y).",
					Solution: trealla.Substitution{
						"X": trealla.Variable{
							Name: "X",
							Attr: []trealla.Term{
								trealla.Compound{
									Functor: "dif",
									Args:    []trealla.Term{trealla.Variable{Name: "X"}, trealla.Variable{Name: "Y"}},
								},
							},
						},
						"Y": trealla.Variable{
							Name: "Y",
							Attr: []trealla.Term{
								trealla.Compound{
									Functor: "dif",
									Args:    []trealla.Term{trealla.Variable{Name: "X"}, trealla.Variable{Name: "Y"}},
								},
							},
						},
					},
				},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			q := pl.Query(ctx, tc.want[0].Query)
			var ans []trealla.Answer
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

func TestThrow(t *testing.T) {
	pl, err := trealla.New(trealla.WithPreopenDir("testdata"))
	if err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()
	q := pl.Query(ctx, `write(hello), throw(ball).`)
	if q.Next(ctx) {
		t.Error("unexpected result", q.Current())
	}
	err = q.Err()

	var ex trealla.ErrThrow
	if !errors.As(err, &ex) {
		t.Fatal("unexpected error:", err, "want ErrThrow")
	}

	if ex.Ball != trealla.Atom("ball") {
		t.Error(`unexpected error value. want: "ball" got:`, ex.Ball)
	}
	if ex.Stdout != "hello" {
		t.Error("unexpected stdout:", ex.Stdout)
	}
}

func TestSyntaxError(t *testing.T) {
	pl, err := trealla.New(trealla.WithPreopenDir("testdata"))
	if err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()
	q := pl.Query(ctx, `hello(`)
	if q.Next(ctx) {
		t.Error("unexpected result", q.Current())
	}
	err = q.Err()

	var ex trealla.ErrThrow
	if !errors.As(err, &ex) {
		t.Fatal("unexpected error:", err, "want ErrThrow")
	}
	want := trealla.Compound{Functor: "error", Args: []trealla.Term{
		trealla.Compound{
			Functor: "syntax_error",
			Args:    []trealla.Term{trealla.Atom("mismatched_parens_or_brackets_or_braces")},
		},
		trealla.Compound{
			Functor: "/",
			Args:    []trealla.Term{trealla.Atom("read_term_from_chars"), int64(3)},
		},
	}}

	if !reflect.DeepEqual(ex.Ball, want) {
		t.Error(`unexpected error value. want:`, want, `got:`, ex.Ball)
	}
}

func TestBind(t *testing.T) {
	ctx := context.Background()
	pl, err := trealla.New()
	if err != nil {
		t.Fatal(err)
	}

	want := int64(123)
	atom := trealla.Atom("abc")
	validate := func(t *testing.T, ans trealla.Answer) {
		t.Helper()
		if x := ans.Solution["X"]; x != want {
			t.Error("unexpected value. want:", want, "got:", x)
		}
		if y := ans.Solution["Y"]; y != want {
			t.Error("unexpected value. want:", want, "got:", y)
		}
		if z := ans.Solution["Z"]; z != atom {
			t.Error("unexpected value. want:", atom, "got:", z)
		}
	}

	t.Run("WithBind", func(t *testing.T) {
		ans, err := pl.QueryOnce(ctx, "Y = X.", trealla.WithBind("X", 123), trealla.WithBind("Z", trealla.Atom("abc")))
		if err != nil {
			t.Fatal(err)
		}
		validate(t, ans)
	})

	t.Run("WithBinding", func(t *testing.T) {
		ans, err := pl.QueryOnce(ctx, "Y = X.", trealla.WithBinding(trealla.Substitution{"X": want, "Z": atom}))
		if err != nil {
			t.Fatal(err)
		}
		validate(t, ans)
	})

	t.Run("overwriting", func(t *testing.T) {
		ans, err := pl.QueryOnce(ctx, "Y = X.", trealla.WithBinding(trealla.Substitution{"X": -1, "Z": atom}), trealla.WithBind("X", want))
		if err != nil {
			t.Fatal(err)
		}
		validate(t, ans)
	})

	t.Run("lists", func(t *testing.T) {
		ans, err := pl.QueryOnce(ctx, "Y = X.", trealla.WithBind("X", []trealla.Term{int64(555)}))
		if err != nil {
			t.Fatal(err)
		}
		want := []trealla.Term{int64(555)}
		if x := ans.Solution["X"]; !reflect.DeepEqual(x, want) {
			t.Error("unexpected value. want:", want, "got:", x)
		}
	})
}
