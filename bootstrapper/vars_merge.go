package bootstrapper

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
)

// VarChange records the disposition of one baseline default var during a
// re-bootstrap merge: Appended means it was newly added, otherwise the
// operator's existing value was preserved. Value keeps the default's TOML type.
type VarChange struct {
	Key      string
	Value    any
	Appended bool
}

const opsVarsHeader = "[ops.vars]"

var varKeyPattern = regexp.MustCompile(`^([A-Za-z0-9_-]+)\s*=`)

// mergeOpsVars folds the baseline's default [ops.vars] into an existing
// platform.toml *textually* — new keys are appended under the section, existing
// keys keep the operator's value, and everything else (comments, ordering,
// other tables) is left byte-for-byte. A decode/re-encode would lose the
// operator's formatting, so the merge is a surgical line insert instead.
func mergeOpsVars(existing []byte, defaults map[string]any) ([]byte, []VarChange) {
	if len(defaults) == 0 {
		return existing, nil
	}

	lines := strings.Split(string(existing), "\n")
	headerIdx := indexOfHeader(lines)

	present := map[string]bool{}
	if headerIdx >= 0 {
		for _, line := range lines[headerIdx+1 : sectionEnd(lines, headerIdx)] {
			if key := varKey(line); key != "" {
				present[key] = true
			}
		}
	}

	changes := classifyVars(defaults, present)
	newLines := appendedLines(changes)
	if len(newLines) == 0 {
		return existing, changes
	}

	if headerIdx < 0 {
		merged := strings.TrimRight(string(existing), "\n") +
			"\n\n" + opsVarsHeader + "\n" + strings.Join(newLines, "\n") + "\n"
		return []byte(merged), changes
	}

	at := insertionPoint(lines, headerIdx)
	merged := append([]string{}, lines[:at]...)
	merged = append(merged, newLines...)
	merged = append(merged, lines[at:]...)
	return []byte(strings.Join(merged, "\n")), changes
}

// classifyVars reports, for each default key in sorted order, whether it would
// be appended (absent) or preserved (already present).
func classifyVars(defaults map[string]any, present map[string]bool) []VarChange {
	keys := make([]string, 0, len(defaults))
	for k := range defaults {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	changes := make([]VarChange, len(keys))
	for i, k := range keys {
		changes[i] = VarChange{Key: k, Value: defaults[k], Appended: !present[k]}
	}
	return changes
}

func appendedLines(changes []VarChange) []string {
	var lines []string
	for _, c := range changes {
		if c.Appended {
			lines = append(lines, c.Key+" = "+tomlValue(c.Value))
		}
	}
	return lines
}

// indexOfHeader returns the line index of the [ops.vars] table header, or -1.
func indexOfHeader(lines []string) int {
	for i, line := range lines {
		if strings.TrimSpace(line) == opsVarsHeader {
			return i
		}
	}
	return -1
}

// sectionEnd returns the line index where the [ops.vars] section ends — the
// next table header, or the end of the file.
func sectionEnd(lines []string, headerIdx int) int {
	for i := headerIdx + 1; i < len(lines); i++ {
		if strings.HasPrefix(strings.TrimSpace(lines[i]), "[") {
			return i
		}
	}
	return len(lines)
}

// insertionPoint returns the index to insert appended vars at: just after the
// last non-blank line of the section body, so new keys sit with the existing
// ones rather than after trailing blanks.
func insertionPoint(lines []string, headerIdx int) int {
	end := sectionEnd(lines, headerIdx)
	at := headerIdx + 1
	for i := headerIdx + 1; i < end; i++ {
		if strings.TrimSpace(lines[i]) != "" {
			at = i + 1
		}
	}
	return at
}

func varKey(line string) string {
	if m := varKeyPattern.FindStringSubmatch(strings.TrimSpace(line)); m != nil {
		return m[1]
	}
	return ""
}

// tomlValue renders a baseline default as a TOML scalar: strings are quoted and
// escaped, bools and numbers are emitted bare, preserving their type on the
// re-bootstrap append.
func tomlValue(v any) string {
	switch x := v.(type) {
	case string:
		return quoteTOML(x)
	case bool:
		if x {
			return "true"
		}
		return "false"
	default:
		return fmt.Sprint(x)
	}
}

func quoteTOML(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `"`, `\"`)
	return `"` + s + `"`
}
