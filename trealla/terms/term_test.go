package terms_test

import (
	"reflect"
	"testing"

	"github.com/trealla-prolog/go/trealla"
	"github.com/trealla-prolog/go/trealla/terms"
)

func TestPI(t *testing.T) {
	table := []struct {
		name string
		in   trealla.Term
		out  trealla.Term
	}{
		{
			name: "atom",
			in:   trealla.Atom("hello"),
			out:  trealla.Atom("/").Of(trealla.Atom("hello"), int64(0)),
		},
		{
			name: "compound",
			in:   trealla.Atom("hello").Of("world"),
			out:  trealla.Atom("/").Of(trealla.Atom("hello"), int64(1)),
		},
		{
			name: "string",
			in:   "foo",
			out:  trealla.Atom("/").Of(trealla.Atom("."), int64(2)),
		},
		{
			name: "list",
			in:   []trealla.Term{trealla.Atom("hello"), trealla.Atom("world")},
			out:  trealla.Atom("/").Of(trealla.Atom("."), int64(2)),
		},
	}

	for _, tc := range table {
		want := tc.out
		got := terms.PI(tc.in)
		if !reflect.DeepEqual(want, got) {
			t.Error(tc.name, "bad pi. want:", want, "got:", got)
		}
	}
}
