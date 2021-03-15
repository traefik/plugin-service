package db

import "fmt"

// ErrNotFound represents a document not found error.
type ErrNotFound struct {
	Err error
}

// Error stringifies the error.
func (e ErrNotFound) Error() string {
	return fmt.Sprintf("not found: %v", e.Err.Error())
}

// Unwrap returns the underlying error.
func (e ErrNotFound) Unwrap() error { return e.Err }
