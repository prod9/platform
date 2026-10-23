package engine

import (
	"context"
	"slices"
	"strings"
	"time"

	"fx.prodigy9.co/config"
)

const (
	RosterUnconfigured RosterState = "unconfigured"
	RosterEmpty        RosterState = "empty"
	RosterResolved     RosterState = "resolved"
	RosterLookupFailed RosterState = "lookup_failed"

	EndpointUnreachable        EndpointState = "unreachable"
	EndpointVersionUnavailable EndpointState = "version_unavailable"
	EndpointReady              EndpointState = "ready"

	inspectionTimeout = 3 * time.Second
)

type RosterState string
type EndpointState string

// Inspection reports current endpoint observations, not build or worker liveness.
type Inspection struct {
	Source     string
	Port       int
	State      RosterState
	ObservedAt time.Time
	Endpoints  []EndpointObservation
}

// EndpointObservation identifies an address, not a durable engine process.
type EndpointObservation struct {
	Address    string
	ObservedAt time.Time
	State      EndpointState
	Version    string
	Err        error
}

// Inspect observes only configured remote endpoints; Close owns its connections.
func (s *Session) Inspect(caller context.Context) (Inspection, error) {
	cfg := cfgFrom(s.ctx)
	report := Inspection{
		Source:     config.Get(cfg, DaggerEngineConfig),
		Port:       config.Get(cfg, DaggerEnginePortConfig),
		State:      RosterUnconfigured,
		ObservedAt: time.Now(),
		Endpoints:  []EndpointObservation{},
	}
	ctx, cancel := context.WithTimeout(config.NewContext(caller, cfg), inspectionTimeout)
	defer cancel()
	if report.Source == "" {
		return report, ctx.Err()
	}
	if err := ctx.Err(); err != nil {
		report.State = RosterLookupFailed
		return report, err
	}

	endpoints, err := hosts(ctx)
	if err != nil {
		report.State = RosterLookupFailed
		return report, err
	}
	if len(endpoints) == 0 {
		report.State = RosterEmpty
		return report, ctx.Err()
	}

	report.State = RosterResolved
	probes := &multiplexer[string, EndpointObservation]{}
	probes.Reset(slices.Compact(endpoints))
	report.Endpoints = probes.Start(func(host string) EndpointObservation {
		return s.inspectEndpoint(ctx, host)
	})
	return report, caller.Err()
}

func (s *Session) inspectEndpoint(ctx context.Context, host string) EndpointObservation {
	observation := EndpointObservation{
		Address:    strings.TrimPrefix(host, "tcp://"),
		ObservedAt: time.Now(),
		State:      EndpointUnreachable,
	}
	client, err := dialHost(ctx, host)
	if err != nil {
		observation.Err = err
		return observation
	}
	s.keep(client)

	observation.Version, observation.Err = client.Version(ctx)
	observation.State = EndpointReady
	if observation.Err != nil {
		observation.State = EndpointVersionUnavailable
	}
	return observation
}
