package trealla

import (
	"context"
	"reflect"
	"testing"
)

func TestQuery(t *testing.T) {
	pl, err := New(WithPreopenDir("testdata"))
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name string
		want Answer
	}{
		{
			name: "true/0",
			want: Answer{
				Query:   `true.`,
				Result:  "success",
				Answers: []Solution{{}},
			},
		},
		{
			name: "member/2",
			want: Answer{
				Query:  `member(X, [1,foo(bar),4.2,"baz",'boop']).`,
				Result: "success",
				Answers: []Solution{
					{"X": int64(1)},
					{"X": Compound{Functor: "foo", Args: []Term{"bar"}}},
					{"X": 4.2},
					{"X": "baz"},
					{"X": "boop"},
				},
			},
		},
		{
			name: "false/0",
			want: Answer{
				Query:  `false.`,
				Result: "failure",
			},
		},
		{
			name: "tak",
			want: Answer{
				Query:   "consult('testdata/tak'), run",
				Result:  "success",
				Answers: []Solution{{}},
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
