package main

import (
	"fmt"

	"github.com/mgutz/ansi"
)

func main() {
	fmt.Println(ansi.Magenta + "Hello, " +
		ansi.Cyan + "World!" +
		ansi.Reset)
}
