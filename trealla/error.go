package trealla

import (
	"errors"
	"fmt"
)

// ErrFailure is returned when a query fails (when it finds no solutions).
type ErrFailure struct {
	// Query is the original query goal.
	Query string
	// Stdout output from the query.
	Stdout string
	// Stderr output from the query (useful for traces).
	Stderr string
}

// Error implements the error interface.
func (err ErrFailure) Error() string {
	return "trealla: query failed: " + err.Query
}

// IsFailure returns true if the given error is a failed query error (ErrFailure).
func IsFailure(err error) bool {
	return errors.As(err, &ErrFailure{})
}

// ErrThrow is returned when an exception is thrown during a query.
type ErrThrow struct {
	// Query is the original query goal.
	Query string
	// Ball is the term thrown by throw/1.
	Ball Term
	// Stdout output from the query.
	Stdout string
	// Stderr output from the query (useful for traces).
	Stderr string
}

// Error implements the error interface.
func (err ErrThrow) Error() string {
	return fmt.Sprintf("trealla: exception thrown: %v", err.Ball)
}

var (
	_ error = ErrFailure{}
	_ error = ErrThrow{}
)
