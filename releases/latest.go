package releases

// Latest is the non-versioned strategy: a single constant name, "latest", that never
// increments. It exists for delivery that has no versions to cut — an infra repo, whose
// image is a moving `latest` tag Flux follows (publishing *is* the deploy). With no version
// there is nothing to tag in git, so publish resolves the name from the strategy directly
// instead of the latest git tag (see IsVersioned).
type Latest struct{}

var _ Strategy = Latest{}

const latestName = "latest"

func (Latest) IsValid(name string) bool { return name == latestName }

func (Latest) NextName(prevName string, bump Bump) (string, error) {
	return latestName, nil
}

func (Latest) IsVersioned() bool { return false }
