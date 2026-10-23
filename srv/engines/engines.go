// Package engines owns authenticated observations of the configured engine fleet.
package engines

import (
	"errors"

	"fx.prodigy9.co/app"
	"platform.prodigy9.co/srv/install"
)

var (
	ErrUnknownEndpoint = errors.New("engines: unknown endpoint")
	App                = app.Build().Name("engines").
				Middlewares(install.ProductGate, install.RecordContext).
				Controllers(EngineCtr{})
)
