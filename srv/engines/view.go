package engines

import (
	"net/http"
	"sort"
	"strings"
	"time"

	"platform.prodigy9.co/engine"
	"platform.prodigy9.co/srv/auth"
	"platform.prodigy9.co/srv/builds"
)

const (
	EndpointReady              EndpointState = "ready"
	EndpointUnreachable        EndpointState = "unreachable"
	EndpointVersionUnavailable EndpointState = "version_unavailable"
	EndpointAbsent             EndpointState = "absent"
	EndpointUnknown            EndpointState = "unknown"
	EndpointLocal              EndpointState = "local"
)

type EndpointState string

type FleetView struct {
	Source      string             `json:"source"`
	Port        int                `json:"port"`
	RosterState engine.RosterState `json:"roster_state"`
	ObservedAt  time.Time          `json:"observed_at"`
	Error       string             `json:"error"`
	Engines     []EndpointView     `json:"engines"`
}

type EndpointView struct {
	Address     string           `json:"address"`
	State       EndpointState    `json:"state"`
	Version     string           `json:"version"`
	ObservedAt  time.Time        `json:"observed_at"`
	Error       string           `json:"error"`
	Assignments []AssignmentView `json:"assignments"`
}

type InstanceView struct {
	Source      string             `json:"source"`
	Port        int                `json:"port"`
	RosterState engine.RosterState `json:"roster_state"`
	ObservedAt  time.Time          `json:"observed_at"`
	Error       string             `json:"error"`
	Engine      EndpointView       `json:"engine"`
	History     []AssignmentView   `json:"history"`
}

type AssignmentView struct {
	BuildID          int64              `json:"build_id"`
	BuildModuleID    int64              `json:"build_module_id"`
	Owner            string             `json:"owner"`
	Repo             string             `json:"repo"`
	Ref              string             `json:"ref"`
	SHA              string             `json:"sha"`
	Name             string             `json:"name"`
	Status           builds.Status      `json:"status"`
	ClaimedAt        time.Time          `json:"claimed_at"`
	EngineAssignedAt time.Time          `json:"engine_assigned_at"`
	LastEventKind    builds.EventKind   `json:"last_event_kind"`
	LastEventAt      time.Time          `json:"last_event_at"`
	Observation      builds.Observation `json:"observation"`
}

func ReadFleet(req *http.Request) (*FleetView, error) {
	keys, err := repositoryKeys(req)
	if err != nil {
		return nil, err
	}
	report, err := inspect(req.Context())
	if err != nil {
		return nil, err
	}
	view := fleetObservation(report)
	addresses := make([]string, len(view.Engines))
	for i, endpoint := range view.Engines {
		addresses[i] = endpoint.Address
	}
	assignments, err := unfinishedAssignments(req.Context(), keys, addresses, time.Now())
	if err != nil {
		return nil, err
	}
	for i := range view.Engines {
		for _, assignment := range assignments[view.Engines[i].Address] {
			if assignment.Status != builds.StatusSucceeded && assignment.Status != builds.StatusFailed {
				view.Engines[i].Assignments = append(view.Engines[i].Assignments, assignment)
			}
		}
	}
	return view, nil
}

func ReadInstance(req *http.Request, address string) (*InstanceView, error) {
	keys, err := repositoryKeys(req)
	if err != nil {
		return nil, err
	}
	report, err := inspect(req.Context())
	if err != nil {
		return nil, err
	}
	context := fleetObservation(report)
	evaluatedAt := time.Now()
	history, err := assignmentHistory(req.Context(), keys, address, evaluatedAt)
	if err != nil {
		return nil, err
	}
	view := &InstanceView{Source: context.Source, Port: context.Port, RosterState: context.RosterState,
		ObservedAt: context.ObservedAt, Error: context.Error, History: history}
	found := false
	for _, endpoint := range context.Engines {
		if endpoint.Address == address {
			view.Engine = endpoint
			found = true
			break
		}
	}
	if !found && len(history) == 0 {
		return nil, ErrUnknownEndpoint
	}
	if !found {
		view.Engine = EndpointView{Address: address, State: EndpointAbsent, ObservedAt: report.ObservedAt,
			Assignments: []AssignmentView{}}
		if report.State == engine.RosterLookupFailed {
			view.Engine.State = EndpointUnknown
			view.Engine.Error = context.Error
		}
	}
	if address == "local" {
		view.Engine.State = EndpointLocal
	}
	// History is bounded; the current assignment list is queried independently so a
	// long-running module cannot disappear behind 50 newer completed assignments.
	current, err := unfinishedAssignments(req.Context(), keys, []string{address}, evaluatedAt)
	if err != nil {
		return nil, err
	}
	for _, assignment := range current[address] {
		if assignment.Status != builds.StatusSucceeded && assignment.Status != builds.StatusFailed {
			view.Engine.Assignments = append(view.Engine.Assignments, assignment)
		}
	}
	return view, nil
}

func repositoryKeys(req *http.Request) ([]string, error) {
	repositories, err := auth.ReadableRepos(req)
	if err != nil {
		return nil, err
	}
	keys := make([]string, len(repositories))
	for i, repo := range repositories {
		keys[i] = strings.ToLower(repo.Owner) + "/" + strings.ToLower(repo.Name)
	}
	return keys, nil
}

func fleetObservation(report engine.Inspection) *FleetView {
	view := &FleetView{Source: report.Source, Port: report.Port, RosterState: report.State,
		ObservedAt: report.ObservedAt, Engines: []EndpointView{}}
	if report.State == engine.RosterLookupFailed {
		view.Error = "Engine discovery failed."
	}
	for _, endpoint := range report.Endpoints {
		observation := EndpointView{Address: endpoint.Address, State: EndpointState(endpoint.State), Version: endpoint.Version,
			ObservedAt: endpoint.ObservedAt, Assignments: []AssignmentView{}}
		switch endpoint.State {
		case engine.EndpointUnreachable:
			observation.Error = "Engine could not be reached."
		case engine.EndpointVersionUnavailable:
			observation.Error = "Engine version could not be read."
		}
		view.Engines = append(view.Engines, observation)
	}
	sort.Slice(view.Engines, func(i, j int) bool { return view.Engines[i].Address < view.Engines[j].Address })
	return view
}
