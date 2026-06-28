package cachex

import "errors"

// Public sentinel errors.
var (
	// ErrNotFound is returned by Get when the key does not exist.
	// It is the cache equivalent of dbx.ErrNoRows.
	ErrNotFound = errors.New("cachex: key not found")

	// ErrNilConfig is returned by New when Config is missing required fields.
	ErrNilConfig = errors.New("cachex: config is missing required fields")

	// ErrUnknownDriver is returned when the driver name has not been registered.
	ErrUnknownDriver = errors.New("cachex: unknown driver (did you import cachex/redis?)")

	// ErrClosed is returned when a closed cache is used.
	ErrClosed = errors.New("cachex: cache is closed")

	// ErrTypeMismatch is returned when Get cannot deserialize into the target type.
	ErrTypeMismatch = errors.New("cachex: type mismatch during deserialization")

	// ErrNilValue is returned when attempting to cache a nil value via GetOrSet.
	ErrNilValue = errors.New("cachex: value function returned nil")
)
