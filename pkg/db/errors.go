package db

import "fmt"

// NotFoundError represents a document not found error.
type NotFoundError struct {
	Err error
}

// Error stringifies the error.
func (e NotFoundError) Error() string {
	if e.Err == nil {
		return "not found"
	}

	return fmt.Sprintf("not found: %v", e.Err.Error())
}

// Unwrap returns the underlying error.
func (e NotFoundError) Unwrap() error { return e.Err }
