package postgres

import (
	"context"
	"fmt"

	"github.com/trealla-prolog/go/trealla"
)

var predicates = []struct {
	name  string
	arity int
	proc  trealla.Predicate
}{
	{"postgres_open_url", 2, open_url_2},
	{"postgres_execute", 4, execute_4},
}

func Register(ctx context.Context, pl trealla.Prolog) error {
	for _, pred := range predicates {
		if err := pl.Register(ctx, pred.name, pred.arity, pred.proc); err != nil {
			return fmt.Errorf("failed to register predicate %s/%d: %w", pred.name, pred.arity, err)
		}
	}
	return nil
}
