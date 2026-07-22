package cmd

import (
	"errors"
	"fmt"
	"sort"
	"strings"

	"fx.prodigy9.co/cmd/prompts"
	"platform.prodigy9.co/conf"
)

// Single-container commands (exec, preview) build exactly one module, so they resolve a
// name before handing it to the engine. They differ only in how they break a tie: a
// scriptable command errors, a human-facing one asks.

// selectModule resolves the one module to operate on, erroring when the project has
// several and the operator named none — keeps the command usable from scripts.
func selectModule(cfg *conf.Model, selectors []string) (string, error) {
	if len(selectors) > 1 {
		return "", fmt.Errorf("multiple modules; select one: %s", strings.Join(selectors, ", "))
	}
	if len(selectors) == 1 {
		return selectors[0], nil
	}

	names := moduleNames(cfg)
	switch len(names) {
	case 0:
		return "", errors.New("no modules configured")
	case 1:
		return names[0], nil
	default:
		return "", fmt.Errorf("multiple modules; select one: %s", strings.Join(names, ", "))
	}
}

// promptModule resolves the one module to operate on, prompting the operator when the
// project has several and none was named.
func promptModule(cfg *conf.Model, selectors []string) (string, error) {
	if len(selectors) > 0 {
		return selectors[0], nil
	}

	names := moduleNames(cfg)
	switch len(names) {
	case 0:
		return "", errors.New("no modules configured")
	case 1:
		return names[0], nil
	default:
		p := prompts.New(nil, nil)
		return p.List("which module?", names[0], names), nil
	}
}

// moduleNames lists the configured modules in a stable order — map iteration order would
// make both the single-module pick and the operator-facing list nondeterministic.
func moduleNames(cfg *conf.Model) []string {
	var names []string
	for modname := range cfg.Modules {
		names = append(names, modname)
	}
	sort.Strings(names)
	return names
}
