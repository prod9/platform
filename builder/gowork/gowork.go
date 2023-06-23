package gowork

import (
	"bufio"
	"errors"
	"io"
	"os"
	"strings"
)

var ErrBadGoWork = errors.New("malformed go.work file")

// dumb scanning parser should suffice since go.work file should be pretty small and we
// only need the module path names
func ParseFile(filename string) ([]string, error) {
	if workfile, err := os.Open(filename); err != nil {
		return nil, err
	} else {
		defer workfile.Close()
		return ParseReader(workfile)
	}
}

func ParseString(str string) ([]string, error) {
	return ParseReader(strings.NewReader(str))
}

func ParseReader(r io.Reader) ([]string, error) {
	if closer, ok := r.(io.Closer); ok {
		defer closer.Close()
	}

	scanner := bufio.NewScanner(r)
	scanner.Split(bufio.ScanWords)

	var mods []string
	for scanner.Scan() {
		txt := scanner.Text()
		if strings.HasPrefix(txt, "./") {
			mods = append(mods, txt[2:])
		}
	}
	return mods, scanner.Err()
}
