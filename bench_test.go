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
		_, err = pl.Query(ctx, "X=1, write(X)")
		if err != nil {
			b.Fatal(err)
		}
	}
}

// func BenchmarkTak(b *testing.B) {
// 	pl, err := New(WithPreopenDir("./testdata"))
// 	if err != nil {
// 		b.Fatal(err)
// 	}
// 	ctx := context.Background()
// 	b.ResetTimer()
// 	for i := 0; i < b.N; i++ {
// 		ans, err := pl.Query(ctx, "consult(tak), run")
// 		println("a")
// 		_ = ans
// 		if err != nil {
// 			b.Fatal(err)
// 		}
// 	}
// }
