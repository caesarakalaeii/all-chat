package repository

import "errors"

// ErrUserNotFound is returned when a user lookup fails to find a record.
var ErrUserNotFound = errors.New("user not found")

// ErrNotFound is returned when a requested record does not exist.
var ErrNotFound = errors.New("not found")

// ErrPlatformAlreadyLinked is returned when trying to link a platform identity
// that is already associated with a different viewer account.
var ErrPlatformAlreadyLinked = errors.New("platform already linked to a different account")

// ErrLastPlatform is returned when trying to unlink the last remaining platform
// identity for a viewer. The viewer must always retain at least one connection.
var ErrLastPlatform = errors.New("cannot unlink the last platform — viewer must have at least one connection")
