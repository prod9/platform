package gowork

import (
	"bufio"
	"errors"
	"io"
	"os"
	"strings"
)

var ErrBadGoWork = errors.New("malformed go.work file")

// A go.work file is small and only its module paths are wanted, so this scans lines
// rather than linking Go's own parser.
func ParseFile(filename string) (string, []string, error) {
	workfile, err := os.Open(filename)
	if err != nil {
		return "", nil, err
	}
	defer workfile.Close()
	return ParseReader(workfile)
}

func ParseString(str string) (string, []string, error) {
	return ParseReader(strings.NewReader(str))
}

func ParseReader(r io.Reader) (string, []string, error) {
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
