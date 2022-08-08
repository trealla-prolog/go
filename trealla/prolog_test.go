package trealla_test

import (
	"context"
	"reflect"
	"testing"

	"github.com/trealla-prolog/go/trealla"
)

func TestQuery(t *testing.T) {
	pl, err := trealla.New(trealla.WithPreopenDir("testdata"))
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name string
		want trealla.Answer
	}{
		{
			name: "true/0",
			want: trealla.Answer{
				Query:   `true.`,
				Result:  "success",
				Answers: []trealla.Solution{{}},
			},
		},
		{
			name: "member/2",
			want: trealla.Answer{
				Query:  `member(X, [1,foo(bar),4.2,"baz",'boop']).`,
				Result: "success",
				Answers: []trealla.Solution{
					{"X": int64(1)},
					{"X": trealla.Compound{Functor: "foo", Args: []trealla.Term{"bar"}}},
					{"X": 4.2},
					{"X": "baz"},
					{"X": "boop"},
				},
			},
		},
		{
			name: "false/0",
			want: trealla.Answer{
				Query:  `false.`,
				Result: "failure",
			},
		},
		{
			name: "tak",
			want: trealla.Answer{
				Query:   "consult('testdata/tak'), run",
				Result:  "success",
				Answers: []trealla.Solution{{}},
				Output:  "'<https://josd.github.io/eye/ns#tak>'([34,13,8],13).\n",
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			ans, err := pl.Query(ctx, tc.want.Query)
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(ans, tc.want) {
				t.Errorf("bad answer. want: %#v got: %#v", tc.want, ans)
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
	ans, err := pl.Query(ctx, `throw(ball).`)
	if err != nil {
		// TODO: might want to make this an error in the future instead of a status
		t.Fatal(err)
	}

	if ans.Result != trealla.ResultError {
		t.Error("unexpected result. want:", trealla.ResultError, "got:", ans.Result)
	}

	if ans.Error != "ball" {
		t.Error(`unexpected error value. want: "ball" got:`, ans.Error)
	}
}
