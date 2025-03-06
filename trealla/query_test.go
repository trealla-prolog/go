package trealla_test

import (
	"context"
	"errors"
	"fmt"
	"log"
	"math/big"
	"os"
	"reflect"
	"runtime"
	"sort"
	"sync"
	"testing"
	"testing/fstest"

	"github.com/trealla-prolog/go/trealla"
)

var testfs = fstest.MapFS{
	"fs.pl": &fstest.MapFile{
		Data: []byte(`go_fs(works).`),
		Mode: 0600,
	},
}

func TestQuery(t *testing.T) {
	testdata := "./testdata"
	if _, err := os.Stat(testdata); os.IsNotExist(err) {
		testdata = "./trealla/testdata"
	}

	pl, err := trealla.New(
		trealla.WithPreopenDir("."),
		trealla.WithMapFS("/custom_fs", testfs),
		trealla.WithLibraryPath("testdata"),
		trealla.WithDebugLog(log.Default()))
	if err != nil {
		t.Fatal(err)
	}

	t.Run("files", func(t *testing.T) {
		if runtime.GOOS == "windows" {
			t.Skip("wonky on windows")
		}

		ctx := context.Background()
		q, err := pl.QueryOnce(ctx, `directory_files("/", X)`)
		if err != nil {
			t.Fatal(err)
		}
		t.Logf("%+v", q.Solution)
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
					// TODO: flake? need to retry once for 'run' to be found
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
			name: "fs.FS support",
			want: []trealla.Answer{
				{
					Query:    `consult('/custom_fs/fs.pl'), go_fs(X), directory_files("/custom_fs", Files).`,
					Solution: trealla.Substitution{"X": trealla.Atom("works"), "Files": []trealla.Term{".", "..", "fs.pl"}},
				},
			},
		},
		// TODO: this is flaking atm, reporting `dif(X, _)` instead of `dif(X, Y)`
		//		 need to investigate
		// {
		// 	name: "residual goals",
		// 	want: []trealla.Answer{
		// 		{
		// 			Query: "dif(X, Y).",
		// 			Solution: trealla.Substitution{
		// 				"X": trealla.Variable{Name: "X", Attr: []trealla.Term{trealla.Compound{Functor: ":", Args: []trealla.Term{trealla.Atom("dif"), trealla.Compound{Functor: "dif", Args: []trealla.Term{trealla.Variable{Name: "X"}, trealla.Variable{Name: "Y"}}}}}}},
		// 				"Y": trealla.Variable{Name: "Y", Attr: nil},
		// 			},
		// 		},
		// 	},
		// },
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
				if trealla.IsFailure(err) {
					if stderr := err.(trealla.ErrFailure).Stderr; stderr != "" {
						fmt.Println(stderr)
					}
				}
				t.Fatal(err)
			} else if tc.err != nil && !errors.Is(err, tc.err) {
				t.Errorf("unexpected error: %#v (%v) ", err, err)
			}
			if tc.err == nil && !reflect.DeepEqual(ans, tc.want) {
				t.Errorf("bad answer. \nwant: %#v\n got: %#v\n", tc.want, ans)
			}
			q.Close()
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

// func TestInterpError(t *testing.T) {
// 	pl, err := trealla.New()
// 	if err != nil {
// 		t.Fatal(err)
// 	}

// 	ctx := context.Background()
// 	q := pl.Query(ctx, `abort.`)
// 	if q.Next(ctx) {
// 		t.Error("unexpected result", q.Current())
// 	}
// 	err = q.Err()
// 	t.Fatal(err)
// 	if err == nil {
// 		t.Fatal("expected error")
// 	}
// }

func TestPreopen(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("skipping unixy test")
	}

	pl, err := trealla.New(trealla.WithPreopenDir("testdata"), trealla.WithMapDir("/foo", "testdata/subdirectory"))
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()

	t.Run("WithPreopenDir", func(t *testing.T) {
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

	t.Run("tricky json atoms", func(t *testing.T) {
		ans, err := pl.QueryOnce(ctx, "X=true(true, aaaa, '', false(a), null, ''(q), _).")
		if err != nil {
			t.Fatal(err)
		}
		want := trealla.Compound{Functor: "true", Args: []trealla.Term{
			trealla.Atom("true"),
			trealla.Atom("aaaa"),
			trealla.Atom(""),
			trealla.Compound{Functor: "false", Args: []trealla.Term{trealla.Atom("a")}},
			trealla.Atom("null"),
			trealla.Compound{Functor: "", Args: []trealla.Term{trealla.Atom("q")}},
			trealla.Variable{Name: "_"},
		}}
		if x := ans.Solution["X"]; !reflect.DeepEqual(x, want) {
			t.Error("unexpected value. want:", want, "got:", x)
		}
	})

	t.Run("appended string", func(t *testing.T) {
		ans, err := pl.QueryOnce(ctx, `Y = "ar", append("foo", [b|Y], X).`)
		if err != nil {
			t.Fatal(err)
		}
		want := "foobar"
		if x := ans.Solution["X"]; !reflect.DeepEqual(x, want) {
			t.Error("unexpected value. want:", want, "got:", x)
		}
	})

	t.Run("rationals", func(t *testing.T) {
		ans, err := pl.QueryOnce(ctx, `A is 1 rdiv 3, B is 9999999999999999 rdiv 2, C is 1 rdiv 9999999999999999.`)
		if err != nil {
			t.Fatal(err)
		}
		want := trealla.Substitution{"A": big.NewRat(1, 3), "B": big.NewRat(9999999999999999, 2), "C": big.NewRat(1, 9999999999999999)}
		for k, v := range ans.Solution {
			if v.(*big.Rat).Cmp(want[k].(*big.Rat)) != 0 {
				t.Error("bad", k, "want:", want[k], "got:", v)
			}
		}
	})
}

func TestConcurrencySemidet(t *testing.T) {
	t.Run("10", testConcurrencySemidet(10))
	// t.Run("100", testConcurrencySemidet(100))
	// t.Run("1k", testConcurrencySemidet(1000))
	// t.Run("10k", testConcurrencySemidet(10000))
}

// testConcurrencySemidet returns a test case that runs N semidet queries
// against the same interpreter and waits for them to finish.
func testConcurrencySemidet(count int) func(*testing.T) {
	return func(t *testing.T) {
		pl, err := trealla.New()
		if err != nil {
			t.Fatal(err)
		}

		var wg sync.WaitGroup
		wg.Add(count)
		for i := 0; i < count; i++ {
			go func() {
				defer wg.Done()
				ctx := context.Background()
				q := pl.Query(ctx, "between(1,10,X)")
				for i := 0; i < 3; i++ {
					if !q.Next(ctx) {
						t.Fatal("next failed at", i)
					}
				}
				if err := q.Err(); err != nil {
					panic(fmt.Sprintf("error: %v, %+v", err, pl.Stats()))
				}
				got := q.Current().Solution["X"]
				want := int64(3)
				if want != got {
					t.Error("bad answer. want:", want, "got:", got, pl.Stats())
				}
				q.Close()
			}()
		}
		wg.Wait()
	}
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
	if !testing.Short() {
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
