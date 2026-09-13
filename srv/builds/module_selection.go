package builds

import (
	"encoding/json"
	"errors"
	"fmt"

	"platform.prodigy9.co/srv/repos"
)

var ErrInvalidModules = errors.New("builds: invalid module selection")

// ModuleSelection distinguishes an omitted selection from an explicit list at admission.
type ModuleSelection []string

func (s *ModuleSelection) UnmarshalJSON(raw []byte) error {
	var names []string
	if err := json.Unmarshal(raw, &names); err != nil {
		return err
	}
	if len(names) == 0 {
		return ErrInvalidModules
	}
	seen := map[string]bool{}
	for _, name := range names {
		if name == "" || seen[name] {
			return ErrInvalidModules
		}
		seen[name] = true
	}
	*s = names
	return nil
}

func (s ModuleSelection) selectModules(modules []repos.ManifestModule) ([]repos.ManifestModule, error) {
	if len(modules) == 0 {
		return nil, ErrInvalidModules
	}
	for _, module := range modules {
		if module.Name == "" {
			return nil, ErrInvalidModules
		}
	}
	if s == nil {
		return modules, nil
	}
	if len(s) == 0 {
		return nil, ErrInvalidModules
	}

	byName := make(map[string]repos.ManifestModule, len(modules))
	for _, module := range modules {
		byName[module.Name] = module
	}
	selected := make([]repos.ManifestModule, 0, len(s))
	for _, name := range s {
		module, exists := byName[name]
		if !exists {
			return nil, fmt.Errorf("%w: unknown or duplicate name %q", ErrInvalidModules, name)
		}
		selected = append(selected, module)
		delete(byName, name)
	}
	return selected, nil
}
