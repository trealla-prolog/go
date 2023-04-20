package trealla

import (
	"context"
	"io"
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
