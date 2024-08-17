// Package terms contains utilities for manipulating Prolog terms.
// It should be helpful for writing Prolog predicates in Go.
package terms

import (
	"math/big"
	"slices"

	"github.com/trealla-prolog/go/trealla"
)

// TypeError returns a term in the form of error(type_error(Want, Got), Ctx).
func TypeError(want trealla.Atom, got trealla.Term, ctx trealla.Term) trealla.Compound {
	return trealla.Atom("error").Of(trealla.Atom("type_error").Of(want, got), ctx)
}

// DomainError returns a term in the form of error(domain_error(Domain, Got), Ctx).
func DomainError(domain trealla.Atom, got trealla.Term, ctx trealla.Term) trealla.Compound {
	return trealla.Atom("error").Of(trealla.Atom("domain_error").Of(domain, got), ctx)
}

// ExistenceError returns a term in the form of error(existence_error(What, Got), Ctx).
func ExistenceError(what trealla.Atom, got trealla.Term, ctx trealla.Term) trealla.Compound {
	return trealla.Atom("error").Of(trealla.Atom("existence_error").Of(what, got), ctx)
}

// PermissionError returns a term in the form of error(permission_error(What, Got), Ctx).
func PermissionError(what trealla.Atom, got trealla.Term, ctx trealla.Term) trealla.Compound {
	return trealla.Atom("error").Of(trealla.Atom("permission_error").Of(what, got), ctx)
}

// ResourceError returns a term in the form of error(resource_error(What), Ctx).
func ResourceError(what trealla.Atom, ctx trealla.Term) trealla.Compound {
	return trealla.Atom("error").Of(trealla.Atom("resource_error").Of(what), ctx)
}

// SystemError returns a term in the form of error(system_error(What), Ctx).
func SystemError(what, ctx trealla.Term) trealla.Compound {
	return trealla.Atom("error").Of(trealla.Atom("system_error").Of(what), ctx)
}

// Throw returns a term in the form of throw(Ball).
func Throw(ball trealla.Term) trealla.Compound {
	return trealla.Compound{Functor: "throw", Args: []trealla.Term{ball}}
}

// PI returns the predicate indicator for the given term as a compound of //2, such as some_atom/0.
// Returns nil for incompatible terms.
func PI(atomic trealla.Term) trealla.Term {
	switch x := atomic.(type) {
	case trealla.Atom:
		return trealla.Compound{Functor: "/", Args: []trealla.Term{x, int64(0)}}
	case trealla.Compound:
		return trealla.Compound{Functor: "/", Args: []trealla.Term{x.Functor, int64(len(x.Args))}}
	case string, []trealla.Term, []any, []string, []int64, []int, []float64, []*big.Int, []trealla.Atom, []trealla.Compound, []trealla.Variable:
		return trealla.Compound{Functor: "/", Args: []trealla.Term{trealla.Atom("."), int64(2)}}
	}
	return nil
}

// ResolveOption searches through "options lists" in the form of `[foo(V1), bar(V2), ...]`
// as seen in open/4. It returns the argument of the compound matching functor,
// or if not found returns fallback.
// If the argument is a variable, it is replaced with fallback.
func ResolveOption[T trealla.Term](opts trealla.Term, functor trealla.Atom, fallback T) T {
	if empty, ok := opts.(trealla.Atom); ok && empty == "[]" {
		return fallback
	}
	list, ok := opts.([]trealla.Term)
	if !ok {
		var empty T
		return empty
	}
	for i, x := range list {
		switch x := x.(type) {
		case trealla.Compound:
			if x.Functor != functor || len(x.Args) != 1 {
				continue
			}
			switch arg := x.Args[0].(type) {
			case T:
				return arg
			case trealla.Variable:
				list[i] = functor.Of(fallback)
				return fallback
			}
		}
	}
	return fallback
}

func IsList(x trealla.Term) bool {
	switch x := x.(type) {
	case string, []trealla.Term, []any, []string, []int64, []int, []float64, []*big.Int, []trealla.Atom, []trealla.Compound, []trealla.Variable:
		return true
	case trealla.Atom:
		return x == "[]"
	}
	return false
}

// Substitute returns a copy of the term x with its arguments replaced by args.
// A nil argument will be kept as-is.
// If len(args) > 0, x must be [trealla.Compound].
func Substitute(x trealla.Term, args ...trealla.Term) trealla.Term {
	if len(args) == 0 {
		return x
	}
	cmp, ok := x.(trealla.Compound)
	if !ok {
		pi := PI(x)
		if pi == nil {
			pi = trealla.Atom("/").Of(trealla.Atom("go$terms.Substitute"), int64(len(args)))
		}
		return Throw(TypeError("compound", x, pi))
	}
	goal := trealla.Compound{Functor: cmp.Functor, Args: slices.Clone(cmp.Args)}
	for i, arg := range args {
		if arg != nil {
			goal.Args[i] = arg
		}
	}
	return goal
}
