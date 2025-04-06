package postgres

import (
	"context"
	"reflect"
	"testing"

	"github.com/trealla-prolog/go/trealla"
)

func TestPG(t *testing.T) {
	pl, err := trealla.New()
	if err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()
	if err := Register(ctx, pl); err != nil {
		t.Fatal(err)
	}

	t.Run("open connection", func(t *testing.T) {
		ctx := context.Background()
		ans, err := pl.QueryOnce(ctx, `postgres_open_url("user=postgres password=password dbname=postgres sslmode=disable", Handle)`)
		if err != nil {
			t.Fatal(err)
		}
		want := trealla.Atom("pg").Of(int64(1))
		if !reflect.DeepEqual(ans.Solution["Handle"], want) {
			t.Error("bad handle. want:", want, "got:", ans.Solution["Handle"])
		}
	})

	t.Run("execute query", func(t *testing.T) {
		ctx := context.Background()
		ans, err := pl.QueryOnce(ctx, `postgres_open_url("user=postgres password=password dbname=postgres sslmode=disable", Handle), postgres_execute(Handle, "CREATE TABLE IF NOT EXISTS guestbook (id serial primary key, time timestamp default CURRENT_TIMESTAMP, author text, msg text);", [], Results).`)
		if err != nil {
			t.Fatal(err)
		}
		_ = ans
		// TODO
	})
}
