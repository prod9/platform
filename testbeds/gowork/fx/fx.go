package fx

import (
	"github.com/mgutz/ansi"
)

func Hello() string {
	return (ansi.Magenta + "Hello, " +
		ansi.Cyan + "World!" +
		ansi.Reset)
}
