package trealla

import (
	"context"
	"runtime"
	"sync"
	"testing"
)

const concurrency = 100

func TestPool(t *testing.T) {
	pool, err := NewPool()
	if err != nil {
		t.Fatal(err)
	}

	err = pool.WriteTx(func(pl Prolog) error {
		return pl.ConsultText(context.Background(), "user", "test(123).")
	})
	if err != nil {
		t.Fatal(err)
	}

	var wg sync.WaitGroup
	for i := 0; i < 5000; i++ {
		i := i
		wg.Add(1)
		go func() {
			defer wg.Done()
			pool.ReadTx(func(p Prolog) error {
				ans, err := p.QueryOnce(context.Background(), "test(X).")
				if err != nil {
					return err
				}
				t.Log(i, ans)
				return nil
			})
		}()
	}
	wg.Wait()
}

func BenchmarkPool4(b *testing.B) {
	benchmarkPool(b, 4)
}

func BenchmarkPool16(b *testing.B) {
	benchmarkPool(b, 16)
}

func BenchmarkPool256(b *testing.B) {
	benchmarkPool(b, 256)
}

func BenchmarkPoolCPUs(b *testing.B) {
	benchmarkPool(b, runtime.NumCPU())
}

func benchmarkPool(b *testing.B, size int) {
	b.Helper()

	db, err := NewPool(WithPoolSize(size))
	if err != nil {
		b.Fatal(err)
	}

	err = db.WriteTx(func(p Prolog) error {
		return p.ConsultText(context.Background(), "user", "test(123).")
	})
	if err != nil {
		b.Fatal(err)
	}
	b.ResetTimer()

	for n := 0; n < b.N; n++ {
		var wg sync.WaitGroup
		for i := 0; i < concurrency; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				db.ReadTx(func(p Prolog) error {
					_, err := p.QueryOnce(context.Background(), "test(X).")
					if err != nil {
						return err
					}
					return nil
				})
			}()
		}
		wg.Wait()
	}
}

func BenchmarkContendedMutex(b *testing.B) {
	pl, _ := New()
	pl.ConsultText(context.Background(), "user", "test(123).")
	b.ResetTimer()

	for n := 0; n < b.N; n++ {
		var wg sync.WaitGroup
		for i := 0; i < concurrency; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				_, err := pl.QueryOnce(context.Background(), "test(X).")
				if err != nil {
					panic(err)
				}
			}()
		}
		wg.Wait()
	}
}
