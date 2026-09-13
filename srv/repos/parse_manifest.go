package repos

import "platform.prodigy9.co/conf"

func ParseManifest(raw []byte, repo string) (*conf.Model, error) {
	model, err := conf.Parse(raw)
	if err != nil {
		return nil, err
	}

	policy, err := model.ResolvePublishPolicy(repo)
	if err != nil {
		return nil, err
	}
	model.Server.Publish = policy
	return model, nil
}
