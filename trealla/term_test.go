package trealla

import (
	"math/big"
	"testing"
)

func TestCompound(t *testing.T) {
	c0 := Compound{
		Functor: "foo",
		Args:    []Term{Atom("bar"), 4.2},
	}
	want := "foo(bar, 4.2)"
	if c0.String() != want {
		t.Errorf("bad string. want: %v got: %v", want, c0.String())
	}
	pi := "foo/2"
	if c0.Indicator() != pi {
		t.Errorf("bad indicator. want: %v got: %v", pi, c0.Indicator())
	}
}

func TestMarshal(t *testing.T) {
	cases := []struct {
		term Term
		want string
	}{
		{
			term: Atom("foo"),
			want: "foo",
		},
		{
			term: Atom("Bar"),
			want: "'Bar'",
		},
		{
			term: Atom("hello world"),
			want: "'hello world'",
		},
		{
			term: Atom("under_score"),
			want: "under_score",
		},
		{
			term: Atom("123"),
			want: "'123'",
		},
		{
			term: Atom("x1"),
			want: "x1",
		},
		{
			term: "string",
			want: `"string"`,
		},
		{
			term: `foo\bar`,
			want: `"foo\\bar"`,
		},
		{
			term: big.NewInt(9999999999999999),
			want: "9999999999999999",
		},
		{
			term: Variable{Name: "X", Attr: []Term{Compound{Functor: ":", Args: []Term{Atom("dif"), Compound{Functor: "dif", Args: []Term{Variable{Name: "X"}, Variable{Name: "Y"}}}}}}},
			want: "':'(dif, dif(X, Y))",
		},
		{
			term: []Term{int64(1), int64(2)},
			want: "[1, 2]",
		},
		{
			term: []int64{int64(1), int64(2)},
			want: "[1, 2]",
		},
		{
			term: []any{int64(1), int64(2)},
			want: "[1, 2]",
		},
	}

	for _, tc := range cases {
		t.Run(tc.want, func(t *testing.T) {
			text, err := marshal(tc.term)
			if err != nil {
				t.Fatal(err)
			}
			if text != tc.want {
				t.Error("bad result. want:", tc.want, "got:", text)
			}
		})
	}
}
