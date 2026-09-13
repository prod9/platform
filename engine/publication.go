package engine

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	fxconfig "fx.prodigy9.co/config"
	"platform.prodigy9.co/conf"
	"platform.prodigy9.co/framework"
)

// publication completes an operation on the connection that built its container.
type publication struct {
	tag   string
	creds registryCreds
}
type registryCreds struct{ registry, username, password string }

func (publication) arch(_ *Session, cfg *conf.Model) string { return cfg.PublishArch }

func (p publication) prepare(unit *framework.BuildUnit) error {
	if p.creds.username != "" {
		if p.creds.password == "" {
			return errors.New("engine: registry password is required")
		}
		host, _, found := strings.Cut(unit.ImageName, "/")
		if !found || p.creds.registry == "" || host != p.creds.registry {
			return fmt.Errorf("engine: registry %q does not match image %q", p.creds.registry, unit.ImageName)
		}
	}

	unit.ImageName += ":" + p.tag
	return nil
}

func (p publication) finish(ctx context.Context, cursor *Run) (string, error) {
	obs, unit := cursor.obs, cursor.unit
	obs.PublishStart(unit.Name, time.Now())
	container := cursor.container
	if p.creds.username != "" {
		secret := cursor.client.SetSecret(RegistryPasswordConfig.Name(), p.creds.password)
		container = container.WithRegistryAuth(p.creds.registry, p.creds.username, secret)
	}
	hash, err := container.Publish(ctx, unit.ImageName)
	obs.PublishDone(unit.Name, time.Now(), err)
	if err != nil {
		return "", err
	}
	return hash, nil
}

func registryCredsFrom(cfg *fxconfig.Source) registryCreds {
	return registryCreds{
		registry: fxconfig.Get(cfg, RegistryConfig),
		username: fxconfig.Get(cfg, RegistryUsernameConfig),
		password: fxconfig.Get(cfg, RegistryPasswordConfig),
	}
}
