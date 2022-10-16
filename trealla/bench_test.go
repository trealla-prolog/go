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
			b.Fatal("no answer", q.Err())
		}
		if q.Err() != nil {
			b.Fatal(err)
		}
		q.Close()
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
	pl, err := New(WithPreopenDir("testdata"))
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
