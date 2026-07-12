package releases

// Rolling is the non-versioned strategy: it never increments a version. It exists for
// delivery that has no versions to cut — an infra repo, whose image is a moving tag Flux
// follows (publishing *is* the deploy). With no version there is nothing to tag in git, so
// publish resolves the emitted name from the strategy directly instead of the latest git
// tag (see IsVersioned). The emitted name is the conventional Docker moving tag, "latest".
type Rolling struct{}

var _ Strategy = Rolling{}

const movingTag = "latest"

func (Rolling) IsValid(name string) bool { return name == movingTag }

func (Rolling) NextName(prevName string, bump Bump) (string, error) {
	return movingTag, nil
}

func (Rolling) IsVersioned() bool { return false }
