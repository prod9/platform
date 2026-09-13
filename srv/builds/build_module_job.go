package builds

import (
	"context"
	"errors"
	"os"
	"strings"

	"fx.prodigy9.co/config"
	"fx.prodigy9.co/data"
	"fx.prodigy9.co/worker"
	"platform.prodigy9.co/engine"
	"platform.prodigy9.co/engine/observer"
	"platform.prodigy9.co/srv/github"
	"platform.prodigy9.co/srv/install"
)

var (
	_ worker.Interface  = (*BuildModuleJob)(nil)
	_ worker.Resetter   = (*BuildModuleJob)(nil)
	_ observer.Observer = (*transcriber)(nil)
)

// BuildModuleJob executes one guarded module claim; engine failure belongs in its events.
type BuildModuleJob struct {
	BuildModuleID int64 `json:"build_module_id"`
}

type moduleRequest struct {
	Name      string `db:"name"`
	ImageName string `db:"image_name"`
	CloneURL  string `db:"clone_url"`
	SHA       string `db:"sha"`
	Ref       string `db:"ref"`
}

func (*BuildModuleJob) Name() string { return "build-module" }

func (j *BuildModuleJob) Reset() { *j = BuildModuleJob{} }

func (j *BuildModuleJob) Run(ctx context.Context) (err error) {
	hostname, err := os.Hostname()
	if err != nil {
		return err
	}
	claim := &ClaimModule{ID: j.BuildModuleID, Hostname: hostname}
	module := &BuildModule{}
	if err := claim.Execute(ctx, module); data.IsNoRows(err) {
		return nil
	} else if err != nil {
		return err
	}

	request, err := readModuleRequest(ctx, module.ID)
	if err != nil {
		return err
	}
	token, _, err := install.Token(ctx)
	if err != nil {
		return err
	}
	tag := ""
	if name, isTag := strings.CutPrefix(request.Ref, "refs/tags/"); isTag {
		tag = name
	}
	if tag != "" {
		ctx, err = publicationContext(ctx, request.ImageName)
		if err != nil {
			return err
		}
	}

	session := engine.NewSession(ctx)
	defer func() { err = errors.Join(err, session.Close()) }()
	scribe := newTranscriber(ctx, module.ID)
	source := engine.Source{URL: request.CloneURL, Revision: request.SHA,
		Username: "x-access-token", Password: token}
	if tag == "" {
		_, err = session.Build(ctx, source, []string{request.Name}, scribe)
	} else {
		_, err = session.BuildAndPublish(ctx, source, []string{request.Name}, tag, scribe)
	}
	if err != nil && !scribe.Complete() {
		return errors.Join(err, scribe.Err())
	}
	// Engine errors have already been transcribed; only persistence and cleanup fail the job.
	return scribe.Err()
}

func readModuleRequest(ctx context.Context, id int64) (moduleRequest, error) {
	var request moduleRequest
	err := data.Get(ctx, &request, `SELECT mm.name, mm.image_name, b.clone_url, b.sha, b.ref
		FROM build_modules bm
		JOIN builds b ON b.id = bm.build_id
		JOIN repo_manifest_modules mm ON mm.id = bm.manifest_module_id
		WHERE bm.id = $1`, id)
	return request, err
}

func publicationContext(ctx context.Context, imageName string) (context.Context, error) {
	host, _, _ := strings.Cut(imageName, "/")
	token, err := github.LoadRegistryToken(ctx, host)
	if err != nil && !errors.Is(err, github.ErrNoRegistryToken) {
		return nil, err
	}
	record, err := install.Load(ctx)
	if err != nil {
		return nil, err
	}

	ambient := config.FromContext(ctx)
	provider := &config.MemProvider{}
	source := config.NewSource(provider, ambient.Vars())
	for _, variable := range ambient.Vars() {
		value, exists, err := ambient.Provider().Get(variable.Name())
		if err != nil {
			return nil, err
		}
		if exists {
			if err := provider.Set(variable.Name(), value); err != nil {
				return nil, err
			}
		}
	}
	config.Set(source, engine.RegistryConfig, host)
	config.Set(source, engine.RegistryUsernameConfig, record.InstalledByLogin)
	config.Set(source, engine.RegistryPasswordConfig, token)
	return config.NewContext(ctx, source), nil
}
