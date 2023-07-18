package trealla_test

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"math/big"
	"os"
	"reflect"
	"runtime"
	"sort"
	"sync"
	"testing"

	"github.com/trealla-prolog/go/trealla"
)

var skipSlow = flag.Bool("skipslow", false, "skip slow tests")

func TestQuery(t *testing.T) {
	testdata := "./testdata"
	if _, err := os.Stat(testdata); os.IsNotExist(err) {
		testdata = "./trealla/testdata"
	}

	pl, err := trealla.New(
		trealla.WithPreopenDir("."),
		trealla.WithLibraryPath("testdata"), trealla.WithDebugLog(log.Default()))
	if err != nil {
		t.Fatal(err)
	}

	t.Run("files", func(t *testing.T) {
		ctx := context.Background()
		q, err := pl.QueryOnce(ctx, `directory_files("/", X)`)
		if err != nil {
			t.Fatal(err)
		}
		t.Logf("%+v", q)
		// spew.Dump(q)
	})

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
			name: "changing user_output",
			want: []trealla.Answer{
				{
					Query:    `tell('/testdata/test.txt'), write(hello), flush_output, X = 1, read_file_to_string("/testdata/test.txt", Content, []), delete_file("/testdata/test.txt")`,
					Solution: trealla.Substitution{"X": int64(1), "Content": "hello"},
				},
			},
		},
		{
			name: "residual goals",
			want: []trealla.Answer{
				{
					Query: "dif(X, Y).",
					Solution: trealla.Substitution{
						"X": trealla.Variable{Name: "X", Attr: []trealla.Term{trealla.Compound{Functor: ":", Args: []trealla.Term{trealla.Atom("dif"), trealla.Compound{Functor: "dif", Args: []trealla.Term{trealla.Variable{Name: "X"}, trealla.Variable{Name: "Y"}}}}}}},
						"Y": trealla.Variable{Name: "Y", Attr: []trealla.Term{[]trealla.Term{}}},
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
				t.Errorf("bad answer. \nwant: %#v\n got: %#v\n", tc.want, ans)
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

func TestPreopen(t *testing.T) {
	pl, err := trealla.New(trealla.WithPreopenDir("testdata"), trealla.WithMapDir("/foo", "testdata/subdirectory"))
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()

	t.Run("WithPreopenDir", func(t *testing.T) {
		if runtime.GOOS == "windows" {
			t.Skip("skipping unixy test")
		}

		q, err := pl.QueryOnce(ctx, `directory_files("/", X)`)
		if err != nil {
			t.Fatal(err)
		}
		want := []trealla.Term{".", "..", "subdirectory", "greeting.pl", "tak.pl"}
		got := q.Solution["X"].([]trealla.Term)
		sort.Slice(want, func(i, j int) bool {
			return want[i].(string) < want[j].(string)
		})
		sort.Slice(got, func(i, j int) bool {
			return got[i].(string) < got[j].(string)
		})
		if !reflect.DeepEqual(want, got) {
			t.Error("bad preopen. want:", want, "got:", got)
		}
	})

	t.Run("WithMapDir", func(t *testing.T) {
		q, err := pl.QueryOnce(ctx, `directory_files("/foo", X)`)
		if err != nil {
			t.Fatal(err)
		}
		want := []trealla.Term{".", "..", "foo.txt"}
		got := q.Solution["X"]
		if !reflect.DeepEqual(want, got) {
			t.Error("bad preopen. want:", want, "got:", got)
		}
	})
}

func TestSyntaxError(t *testing.T) {
	t.Parallel()

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
	t.Parallel()

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

func TestConcurrencySemidet100(t *testing.T) {
	pl, err := trealla.New()
	if err != nil {
		t.Fatal(err)
	}

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		// i := i
		wg.Add(1)
		go func() {
			defer wg.Done()
			ctx := context.Background()
			q := pl.Query(ctx, "between(1,10,X)")
			q.Next(ctx)
			q.Next(ctx)
			q.Next(ctx)
			if err := q.Err(); err != nil {
				panic(fmt.Sprintf("error: %v, %+v", err, pl.Stats()))
			}
			got := q.Current().Solution["X"]
			want := int64(3)
			if want != got {
				t.Error("bad answer. want:", want, "got:", got)
			}
			q.Close()
		}()
	}
	wg.Wait()
}

func TestConcurrencyDet10K(t *testing.T) {
	pl, _ := trealla.New()

	pl.ConsultText(context.Background(), "user", "test(123).")

	var wg sync.WaitGroup
	for i := 0; i < 10_000; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			pl.QueryOnce(context.Background(), "test(X).")
		}()
	}
	wg.Wait()
}

func TestConcurrencyDet100K(t *testing.T) {
	if *skipSlow {
		t.Skip("skipping slow tests")
	}

	pl, _ := trealla.New()

	pl.ConsultText(context.Background(), "user", "test(123).")

	var wg sync.WaitGroup
	for i := 0; i < 100_000; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			pl.QueryOnce(context.Background(), "test(X).")
		}()
	}
	wg.Wait()
}
