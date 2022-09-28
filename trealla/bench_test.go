package trealla

import (
	"context"
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
			b.Fatal("no answer")
		}
		if q.Err() != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkTak(b *testing.B) {
	pl, err := New(WithPreopenDir("testdata"))
	if err != nil {
		b.Fatal(err)
	}
	ctx := context.Background()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		q := pl.Query(ctx, "consult('testdata/tak'), run")
		if !q.Next(ctx) {
			b.Fatal("no answer")
		}
		if q.Err() != nil {
			b.Fatal(err)
		}
	}
}
