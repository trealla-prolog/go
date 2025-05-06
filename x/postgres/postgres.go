package postgres

import (
	"database/sql"

	_ "github.com/lib/pq"

	"github.com/trealla-prolog/go/trealla"
	"github.com/trealla-prolog/go/trealla/terms"
)

func open_url_2(pl trealla.Prolog, _ trealla.Subquery, goal trealla.Term) trealla.Term {
	pi := terms.PI(goal)
	g, ok := goal.(trealla.Compound)
	if !ok {
		return terms.Throw(terms.TypeError("compound", goal, pi))
	}

	connStr, ok := g.Args[0].(string)
	if !ok {
		return terms.Throw(terms.TypeError("chars", g.Args[0], pi))
	}

	_, ok = g.Args[1].(trealla.Variable)
	if !ok {
		return terms.Throw(trealla.Atom("error").Of(trealla.Atom("uninstantiation_error").Of(g.Args[1]), pi))
	}

	db, err := sql.Open("postgres", connStr)
	if err != nil {
		return terms.Throw(dbError(err, pi))
	}

	id := nextID()
	connections.Store(id, db)

	g.Args[1] = trealla.Atom("pg").Of(id)
	return g
}

func execute_4(pl trealla.Prolog, _ trealla.Subquery, goal trealla.Term) trealla.Term {
	pi := terms.PI(goal)
	g, ok := goal.(trealla.Compound)
	if !ok {
		return terms.Throw(terms.TypeError("compound", goal, pi))
	}

	handle, _ := g.Args[0].(trealla.Compound)
	if handle.Functor != "pg" {
		return terms.Throw(terms.TypeError("db_connection", g.Args[0], pi))
	}

	rawDB, ok := connections.Load(handle.Args[0].(int64))
	if !ok {
		return terms.Throw(terms.DomainError("db_connection", handle.Args[0], pi))
	}
	db := rawDB.(*sql.DB)

	// TODO: apply query arguments

	result, err := db.Exec(g.Args[1].(string))
	_ = result
	if err != nil {
		return terms.Throw(dbError(err, pi))
	}

	// TODO: build result

	return g
}

func dbError(err error, pi trealla.Term) trealla.Term {
	return trealla.Atom("error").Of(trealla.Atom("db_error").Of(err.Error()), pi)
}
