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
func ParseFile(filename string) (string, []string, error) {
	if workfile, err := os.Open(filename); err != nil {
		return "", nil, err
	} else {
		defer workfile.Close()
		return ParseReader(workfile)
	}
}

func ParseString(str string) (string, []string, error) {
	return ParseReader(strings.NewReader(str))
}

func ParseReader(r io.Reader) (string, []string, error) {
	if closer, ok := r.(io.Closer); ok {
		defer closer.Close()
	}

	scanner := bufio.NewScanner(r)
	scanner.Split(bufio.ScanWords)
	version := ""

	var mods []string
	for scanner.Scan() {
		txt := scanner.Text()
		if strings.HasPrefix(txt, "./") {
			mods = append(mods, txt[2:])
		} else if strings.HasPrefix(txt, "1.") {
			version = txt
			if len(version) < 5 { // 1.2.3 is at least 5 chars
				version += ".0"
			}
		}
	}

	return version, mods, scanner.Err()
}
