package terms_test

import (
	"context"
	"fmt"
	"iter"

	"github.com/trealla-prolog/go/trealla"
	"github.com/trealla-prolog/go/trealla/terms"
)

func Example_nondet_predicate() {
	ctx := context.Background()
	pl, err := trealla.New()
	if err != nil {
		panic(err)
	}

	// Let's add a native equivalent of between/3.
	// betwixt(+Min, +Max, ?N).
	pl.RegisterNondet(ctx, "betwixt", 3, func(_ trealla.Prolog, _ trealla.Subquery, goal0 trealla.Term) iter.Seq[trealla.Term] {
		return func(yield func(trealla.Term) bool) {
			// goal is the goal called by Prolog, such as: betwixt(1, 10, X).
			// Guaranteed to match up with the registered arity and name.
			goal := goal0.(trealla.Compound)

			// Check Min and Max argument's type, must be integers (all integers are int64).
			// TODO: should throw instantiation_error instead of type_error if Min or Max are variables.
			min, ok := goal.Args[0].(int64)
			if !ok {
				// throw(error(type_error(integer, Min), betwixt/3)).
				yield(terms.Throw(terms.TypeError("integer", goal.Args[0], terms.PI(goal))))
				return
			}
			max, ok := goal.Args[1].(int64)
			if !ok {
				// throw(error(type_error(integer, Max), betwixt/3)).
				yield(terms.Throw(terms.TypeError("integer", goal.Args[1], terms.PI(goal))))
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
				yield(terms.Throw(terms.TypeError("integer", goal.Args[2], terms.PI(goal))))
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
