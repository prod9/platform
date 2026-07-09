// Package buildinfo renders program output -- results and summaries -- to stdout, kept
// distinct from buildlog's diagnostic logging on stderr.
package buildinfo

import (
	"fmt"
	"io"
	"os"
)

func out() io.Writer { return os.Stdout }

// Header prints a top-level heading line.
func Header(text string) {
	fmt.Fprintln(out(), text)
}

// Item prints an indented item beneath a Header.
func Item(text string) {
	fmt.Fprintln(out(), "  "+text)
}
