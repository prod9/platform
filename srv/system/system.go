// Package system owns the installed server's operational state and remediation.
package system

import (
	"fx.prodigy9.co/app"
	"platform.prodigy9.co/srv/install"
)

var App = app.Build().
	Name("system").
	Middlewares(install.ProductGate).
	Controllers(SystemCtr{})
