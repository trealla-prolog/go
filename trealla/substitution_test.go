package trealla

import (
	"context"
	"fmt"
	"reflect"
	"testing"
)

func TestScan(t *testing.T) {
	cases := []struct {
		sub  Substitution
		want any
	}{
		{
			sub:  Substitution{"X": Atom("foo")},
			want: struct{ X string }{X: "foo"},
		},
		{
			sub:  Substitution{"X": int64(123)},
			want: struct{ X int }{X: 123},
		},
		{
			sub:  Substitution{"X": []Term{Atom("foo"), Atom("bar")}},
			want: struct{ X []string }{X: []string{"foo", "bar"}},
		},
		{
			sub: Substitution{"X": "abc"},
			want: struct {
				ABC string `prolog:"X"`
			}{
				ABC: "abc",
			},
		},
		{
			sub:  Substitution{"X": []Term{}},
			want: struct{ X string }{X: ""},
		},
		{
			sub:  Substitution{"X": []Term{}},
			want: struct{ X Atom }{X: ""},
		},
		{
			sub:  Substitution{"X": []Term{}},
			want: struct{ X, Y []Term }{X: []Term{}},
		},
		{
			sub:  Substitution{"X": "x", "Y": "y"},
			want: map[string]any{"X": "x", "Y": "y"},
		},
		// strings → slices
		{
			sub:  Substitution{"X": "xyz"},
			want: struct{ X []Atom }{X: []Atom{"x", "y", "z"}},
		},
		{
			sub:  Substitution{"X": "xyzあ"},
			want: struct{ X []Term }{X: []Term{Atom("x"), Atom("y"), Atom("z"), Atom("あ")}},
		},
	}

	for _, tc := range cases {
		t.Run(fmt.Sprintf("%v", tc.sub), func(t *testing.T) {
			got := reflect.New(reflect.TypeOf(tc.want)).Elem().Interface()
			if err := tc.sub.Scan(&got); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(tc.want, got) {
				t.Errorf("bad scan result. want: %#v, got: %#v", tc.want, got)
			}
		})
	}
}

func ExampleSubstitution_Scan() {
	ctx := context.Background()
	pl, err := New()
	if err != nil {
		panic(err)
	}

	answer, err := pl.QueryOnce(ctx, `X = 123, Y = abc, Z = ["hello", "world"].`)
	if err != nil {
		panic(err)
	}
	var result struct {
		X  int
		Y  string
		Hi []string `prolog:"Z"`
	}
	if err := answer.Solution.Scan(&result); err != nil {
		panic(err)
	}

	fmt.Printf("%+v", result)
	// Output: {X:123 Y:abc Hi:[hello world]}
}
