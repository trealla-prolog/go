package trealla_test

import (
	"context"
	"encoding/base32"
	"fmt"
	"iter"

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
				trealla.Atom("type_error").Of("chars", goal.Args[0]),
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

func Example_register_nondet() {
	ctx := context.Background()
	pl, err := trealla.New()
	if err != nil {
		panic(err)
	}

	// Let's add a native equivalent of between/3.
	// betwixt(+Min, +Max, ?N).
	pl.RegisterNondet(ctx, "betwixt", 3, func(_ trealla.Prolog, _ trealla.Subquery, goal0 trealla.Term) iter.Seq[trealla.Term] {
		pi := trealla.Atom("/").Of(trealla.Atom("betwixt"), 2)
		return func(yield func(trealla.Term) bool) {
			// goal is the goal called by Prolog, such as: base32("hello", X).
			// Guaranteed to match up with the registered arity and name.
			goal := goal0.(trealla.Compound)

			// Check Min and Max argument's type, must be integers (all integers are int64).
			min, ok := goal.Args[0].(int64)
			if !ok {
				// throw(error(type_error(list, X), base32/2)).
				yield(trealla.Atom("throw").Of(trealla.Atom("error").Of(
					trealla.Atom("type_error").Of("integer", goal.Args[0]),
					pi,
				)))
				// See terms subpackage for an easier way:
				// yield(terms.Throw(terms.TypeError("integer", goal.Args[0], terms.PI(goal)))
				return
			}
			max, ok := goal.Args[1].(int64)
			if !ok {
				// throw(error(type_error(list, X), base32/2)).
				yield(trealla.Atom("throw").Of(trealla.Atom("error").Of(
					trealla.Atom("type_error").Of("integer", goal.Args[0]),
					pi,
				)))
				return
			}

			if min > max {
				// Since we haven't yielded anything, this will fail.
				return
			}

			switch x := goal.Args[2].(type) {
			case int64:
				// If the 3rd argument is bound, we can do a simple check and stop iterating.
				if x >= min && x <= max {
					yield(goal)
					return
				}
			case trealla.Variable:
				// Create choice points unifying N from min to max
				for n := min; n <= max; n++ {
					goal.Args[2] = n
					if !yield(goal) {
						break
					}
				}
			default:
				yield(trealla.Atom("throw").Of(trealla.Atom("error").Of(
					trealla.Atom("type_error").Of("integer", goal.Args[2]),
					trealla.Atom("/").Of(trealla.Atom("base32"), 2),
				)))
			}
		}
	})

	// Try it out.
	answer, err := pl.QueryOnce(ctx, `findall(N, betwixt(1, 5, N), Ns), write(Ns).`)
	if err != nil {
		panic(err)
	}
	fmt.Println(answer.Stdout)
	// Output: [1,2,3,4,5]
}
