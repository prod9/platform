package dateref

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

var testcases = []struct {
	str string
	ref DateRef
	err error
}{
	{"v20180101", DateRef{date: time.Date(2018, 1, 1, 0, 0, 0, 0, time.UTC), counter: 0}, nil},
	{"v20180101-0", DateRef{date: time.Date(2018, 1, 1, 0, 0, 0, 0, time.UTC), counter: 0}, nil},
	{"v20180101-1", DateRef{date: time.Date(2018, 1, 1, 0, 0, 0, 0, time.UTC), counter: 1}, nil},
	{"v20180101-2", DateRef{date: time.Date(2018, 1, 1, 0, 0, 0, 0, time.UTC), counter: 2}, nil},
	{"vabc", DateRef{}, ErrInvalidDateRef},
	{"v20180101-abc", DateRef{}, ErrInvalidDateRef},
	{"20180101", DateRef{}, ErrInvalidDateRef},
}

func TestParse(t *testing.T) {
	for _, testcase := range testcases {
		t.Run(testcase.str, func(t *testing.T) {
			ref, err := Parse(testcase.str)
			if testcase.err != nil {
				require.Equal(t, testcase.err, err)
			} else {
				require.NoError(t, err)
				require.Equal(t, testcase.ref, ref)
			}
		})
	}
}

func TestString(t *testing.T) {
	for _, testcase := range testcases {
		if testcase.err != nil {
			continue
		}

		t.Run(testcase.str, func(t *testing.T) {
			if strings.HasSuffix(testcase.str, "-0") { // special case since -0 are removed
				require.Equal(t, testcase.str[:len(testcase.str)-2], testcase.ref.String())
			} else {
				require.Equal(t, testcase.str, testcase.ref.String())
			}
		})
	}
}
