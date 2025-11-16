package repository

import "errors"

// ErrUserNotFound is returned when a user lookup fails to find a record.
var ErrUserNotFound = errors.New("user not found")
