//go:build experiment

package trealla

import (
	"context"
	"log"
	"sync"
	"testing"
	"time"
)

const concurrency = 10

func TestDB(t *testing.T) {
	db, err := NewDB()
	if err != nil {
		t.Fatal(err)
	}

	err = db.WriteTx(func(pl Prolog) error {
		return pl.ConsultText(context.Background(), "user", "test(123).")
	})
	if err != nil {
		t.Fatal(err)
	}

	var wg sync.WaitGroup
	for i := 0; i < 5000; i++ {
		i := i
		time.Sleep(100 * time.Microsecond)
		// runtime.Gosched()
		wg.Add(1)
		go func() {
			defer wg.Done()
			db.ReadTx(func(p Prolog) error {
				ans, err := p.QueryOnce(context.Background(), "test(X)")
				if err != nil {
					return err
				}
				log.Println(i, ans)
				return nil
			})
		}()
	}
	wg.Wait()

	t.Fail()
}

func TestDBXX(t *testing.T) {
	pl, _ := New()

	pl.ConsultText(context.Background(), "user", "test(123).")

	var wg sync.WaitGroup
	for i := 0; i < 5000; i++ {
		// time.Sleep(1 * time.Millisecond)
		wg.Add(1)
		go func() {
			defer wg.Done()
			pl.QueryOnce(context.Background(), "test(X)")
		}()
	}
	wg.Wait()

	t.Fail()
}

func BenchmarkDB(b *testing.B) {
	db, err := NewDB()
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
			i := i
			wg.Add(1)
			go func() {
				defer wg.Done()
				db.ReadTx(func(p Prolog) error {
					ans, err := p.QueryOnce(context.Background(), "test(X)")
					if err != nil {
						return err
					}
					_ = ans
					_ = i
					// log.Println(i, ans)
					return nil
				})
			}()
		}
		wg.Wait()
	}
}

func BenchmarkMutex(b *testing.B) {
	pl, _ := New()
	pl.ConsultText(context.Background(), "user", "test(123).")
	// if err != nil {
	// 	b.Fatal(err)
	// }
	b.ResetTimer()

	for n := 0; n < b.N; n++ {
		var wg sync.WaitGroup
		for i := 0; i < concurrency; i++ {
			i := i
			wg.Add(1)
			go func() {
				defer wg.Done()
				ans, err := pl.QueryOnce(context.Background(), "test(X)")
				if err != nil {
					panic(err)
				}
				_ = ans
				_ = i
				// log.Println(i, ans)
			}()
		}
		wg.Wait()
	}
}
