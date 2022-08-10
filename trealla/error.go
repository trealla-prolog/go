package trealla

import "fmt"

// ErrFailure is returned when a query fails (when it finds no solutions).
var ErrFailure = fmt.Errorf("trealla: query failed")

// ErrThrow is returned when an exception is thrown during a query.
type ErrThrow struct {
	// Ball is the term thrown by throw/1.
	Ball Term
}

func (err ErrThrow) Error() string {
	return fmt.Sprintf("trealla: exception thrown: %v", err.Ball)
}
