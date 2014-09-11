package ipc

import "time"

const (
	// The publisher re-mounts on this period.
	publishPeriod = time.Minute

	// The server uses this timeout for incoming calls before the real timeout is known.
	// The client uses this as the default max time for connecting to the server including
	// name resolution.
	defaultCallTimeout = time.Minute

	// The client uses this as the maximum time between retry attempts when starting a call.
	maxBackoff = time.Minute
)
