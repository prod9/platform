package github

import "fx.prodigy9.co/app"

// Fragment declares the config-only GitHub concern in the server application tree.
// App already names the GitHub credential domain object in this package.
var Fragment = app.Build().Name("github")
