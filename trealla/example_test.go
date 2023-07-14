package trealla_test

import (
	"context"
	"encoding/base32"
	"fmt"

	"github.com/trealla-prolog/go/trealla"
)

func Example() {
	ctx := context.Background()

	// create a new Prolog interpreter
	pl, err := trealla.New()
	if err != nil {
		panic(err)
	}

	// start a new query
	query := pl.Query(ctx, "member(X, [1, foo(bar), c]).")
	// calling Close is not necessary if you iterate through the whole query, but it doesn't hurt
	defer query.Close()

	// iterate through answers
	for query.Next(ctx) {
		answer := query.Current()
		x := answer.Solution["X"]
		fmt.Println(x)
	}

	// make sure to check the query for errors
	if err := query.Err(); err != nil {
		panic(err)
	}
	// Output: 1
	// foo(bar)
	// c
}

func ExampleWithBind() {
	ctx := context.Background()
	pl, err := trealla.New()
	if err != nil {
		panic(err)
	}

	// bind the variable X to the atom 'hello world' through query options
	answer, err := pl.QueryOnce(ctx, "write(X).", trealla.WithBind("X", trealla.Atom("hello world")))
	if err != nil {
		panic(err)
	}

	fmt.Println(answer.Stdout)
	// Output: hello world
}

func Example_register() {
	ctx := context.Background()
	pl, err := trealla.New()
	if err != nil {
		panic(err)
	}

	// Let's add a base32 encoding predicate.
	// To keep it brief, this only handles one mode.
	// base32(+Input, -Output) is det.
	pl.Register(ctx, "base32", 2, func(_ trealla.Prolog, _ trealla.Subquery, goal0 trealla.Term) trealla.Term {
		// goal is the goal called by Prolog, such as: base32("hello", X).
		// Guaranteed to match up with the registered arity and name.
		goal := goal0.(trealla.Compound)

		// Check the Input argument's type, must be string.
		input, ok := goal.Args[0].(string)
		if !ok {
			// throw(error(type_error(list, X), base32/2)).
			return trealla.Atom("throw").Of(trealla.Atom("error").Of(
				trealla.Atom("type_error").Of("list", goal.Args[0]),
				trealla.Atom("/").Of(trealla.Atom("base32"), 2),
			))
		}

		// Check Output type, must be string or var.
		switch goal.Args[1].(type) {
		case string: // ok
		case trealla.Variable: // ok
		default:
			// throw(error(type_error(list, X), base32/2)).
			// See: terms subpackage for convenience functions to create these errors.
			return trealla.Atom("throw").Of(trealla.Atom("error").Of(
				trealla.Atom("type_error").Of("list", goal.Args[0]),
				trealla.Atom("/").Of(trealla.Atom("base32"), 2),
			))
		}

		// Do the actual encoding work.
		output := base32.StdEncoding.EncodeToString([]byte(input))

		// Return a goal that Trealla will unify with its input:
		// base32(Input, "output_goes_here").
		return trealla.Atom("base32").Of(input, output)
	})

	// Try it out.
	answer, err := pl.QueryOnce(ctx, `base32("hello", Encoded).`)
	if err != nil {
		panic(err)
	}
	fmt.Println(answer.Solution["Encoded"])
	// Output: NBSWY3DP
}
