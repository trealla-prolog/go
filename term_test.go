package trealla

import "testing"

func TestCompound(t *testing.T) {
	c0 := Compound{
		Functor: "foo",
		Args:    []Term{"bar", 4.2},
	}
	want := "foo(bar, 4.2)"
	if c0.String() != want {
		t.Errorf("bad string. want: %v got: %v", want, c0.String())
	}
}
