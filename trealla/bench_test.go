package trealla

import (
	"context"
	"strings"
	"testing"
)

func BenchmarkQuery(b *testing.B) {
	pl, err := New()
	if err != nil {
		b.Fatal(err)
	}
	ctx := context.Background()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		q := pl.Query(ctx, "X=1, write(X)")
		if !q.Next(ctx) {
			b.Fatal("no answer", q.Err())
		}
		if q.Err() != nil {
			b.Fatal(err)
		}
		q.Close()
	}
}

func BenchmarkNewProlog(b *testing.B) {
	for i := 0; i < b.N; i++ {
		pl, err := New()
		if err != nil {
			b.Fatal(err)
		}
		_ = pl
		pl.Close()
	}
}

func BenchmarkClone(b *testing.B) {
	pl, err := New()
	if err != nil {
		b.Fatal(err)
	}
	if err := pl.ConsultText(context.Background(), "user", strings.Repeat("hello(world). ", 1024)); err != nil {
		b.Fatal(err)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		clone, err := New()
		if err != nil {
			b.Fatal(err)
		}
		_ = clone
	}
}

func BenchmarkRedo(b *testing.B) {
	pl, err := New()
	if err != nil {
		b.Fatal(err)
	}
	ctx := context.Background()
	q := pl.Query(ctx, "repeat.")
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		q.Next(ctx)
		if q.Err() != nil {
			b.Fatal(err)
		}
	}
	q.Close()
}

func BenchmarkTak(b *testing.B) {
	pl, err := New(WithPreopenDir("."))
	if err != nil {
		b.Fatal(err)
	}
	ctx := context.Background()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := pl.QueryOnce(ctx, "consult('testdata/tak'), run")
		if err != nil {
			b.Fatal(err)
		}
	}
}
