package db

import (
	"errors"
	"fmt"
)

// ErrEmptyPatch is returned for a patch that updates nothing.
var ErrEmptyPatch = errors.New("nothing to update")

// ErrAlreadyExists is returned when a plugin with the same name already exists, disabled or hidden included.
var ErrAlreadyExists = errors.New("a plugin with this name already exists")

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
