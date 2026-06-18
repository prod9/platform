// Package baseline turns the embedded cluster-baseline directive files into a
// concrete set to apply, gated by the operator's [ops.vars]. Gating is
// whole-file selection (the DSL itself stays branch-free): a filename encodes
// whether it is always applied, one variant of a mutually-exclusive choice, or
// an optional overlay.
//
//	name.dsl          always applied
//	name@variant.dsl  choice group `name`; applied when vars[name] == variant
//	name+flag.dsl     overlay; applied when vars[flag] == "true"
package baseline

import (
	"fmt"
	"slices"
	"sort"
	"strings"
)

const dslExt = ".dsl"

// OptionKind is the shape of a selectable baseline option: pick one variant, or
// flip an overlay on/off.
type OptionKind int

const (
	OptionChoice OptionKind = iota
	OptionToggle
)

// Option is one operator-selectable knob discovered in the baseline file set.
// Key is the [ops.vars] key it reads. For a choice, Variants lists the allowed
// values; for a toggle, Variants is empty and the value is a string-bool.
type Option struct {
	Key      string
	Kind     OptionKind
	Variants []string
	Default  string
}

// ScanOptions derives the selectable options from a baseline file set, sorted by
// key. Plain files contribute no option.
func ScanOptions(files []string) []Option {
	choices := map[string][]string{}
	toggles := map[string]bool{}
	for _, f := range files {
		switch e := parse(f); e.kind {
		case entryChoice:
			choices[e.key] = append(choices[e.key], e.variant)
		case entryToggle:
			toggles[e.key] = true
		}
	}

	var opts []Option
	for group, variants := range choices {
		sort.Strings(variants)
		opts = append(opts, Option{
			Key:      group,
			Kind:     OptionChoice,
			Variants: variants,
			Default:  variants[0],
		})
	}
	for flag := range toggles {
		opts = append(opts, Option{Key: flag, Kind: OptionToggle, Default: "false"})
	}

	sort.Slice(opts, func(i, j int) bool { return opts[i].Key < opts[j].Key })
	return opts
}

// Select resolves the baseline file set against the operator's vars, returning
// the files to apply in deterministic order: every plain file, the chosen
// variant of each choice group (its default when unset), and each enabled
// overlay. An unknown choice value is a hard error rather than a silent
// fallback.
func Select(files []string, vars map[string]string) ([]string, error) {
	chosen, err := resolveChoices(files, vars)
	if err != nil {
		return nil, err
	}

	sorted := append([]string{}, files...)
	sort.Strings(sorted)

	var selected []string
	for _, f := range sorted {
		switch e := parse(f); e.kind {
		case entryPlain:
			selected = append(selected, f)
		case entryChoice:
			if e.variant == chosen[e.key] {
				selected = append(selected, f)
			}
		case entryToggle:
			if vars[e.key] == "true" {
				selected = append(selected, f)
			}
		}
	}
	return selected, nil
}

// resolveChoices picks the active variant for every choice group: the operator's
// value when set (validated against the group's variants), otherwise the group
// default.
func resolveChoices(files []string, vars map[string]string) (map[string]string, error) {
	chosen := map[string]string{}
	for _, opt := range ScanOptions(files) {
		if opt.Kind != OptionChoice {
			continue
		}

		want, set := vars[opt.Key]
		if !set {
			chosen[opt.Key] = opt.Default
			continue
		}
		if !slices.Contains(opt.Variants, want) {
			return nil, fmt.Errorf("baseline: %q is not a variant of %q (have %v)",
				want, opt.Key, opt.Variants)
		}
		chosen[opt.Key] = want
	}
	return chosen, nil
}

type entryKind int

const (
	entryPlain entryKind = iota
	entryChoice
	entryToggle
)

type entry struct {
	kind    entryKind
	key     string // choice group, or toggle flag
	variant string // choice only
}

// parse classifies a baseline filename by its marker: '@' a choice variant, '+'
// an overlay toggle, neither a plain always-on file.
func parse(file string) entry {
	stem := strings.TrimSuffix(file, dslExt)

	if group, variant, ok := strings.Cut(stem, "@"); ok {
		return entry{kind: entryChoice, key: group, variant: variant}
	}
	if _, flag, ok := strings.Cut(stem, "+"); ok {
		return entry{kind: entryToggle, key: flag}
	}
	return entry{kind: entryPlain}
}
