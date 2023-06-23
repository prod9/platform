package bootstrapper

import (
	_ "embed"
	"errors"
	"log"
	"os"
	"path/filepath"
	"text/template"

	"platform.prodigy9.co/config"
)

var (
	//go:embed github-workflow.yaml.template
	githubWorkflowTemplate string
	//go:embed platform.toml.template
	platformTomlTemplate string

	templates = map[string]string{
		".github/workflows/platform.yaml": githubWorkflowTemplate,
		"platform.toml":                   platformTomlTemplate,
	}
)

var ErrPlatformAlreadyExist = errors.New("platform.toml already exists")

type Info struct {
	ProjectName     string
	Maintainer      string
	MaintainerEmail string
	GoVersion       string
}

func Bootstrap(dir string, info *Info) error {
	wd, err := os.Getwd()
	if err != nil {
		return err
	}

	existing, err := config.ResolvePath(wd)
	if errors.Is(err, config.ErrNoPlatformConfig) {
		// expected, since we're bootstrapping

	} else if err != nil { // unrelated error
		log.Fatalln(err)

	} else { // err == nil, we found existing platform.toml
		log.Println("found:", existing)
		return ErrPlatformAlreadyExist
	}

	// applying...
	for filename, content := range templates {
		outfilename := filepath.Join(wd, filename)
		if err := writeTemplate(content, outfilename, info); err != nil {
			return err
		} else {
			log.Println("wrote", outfilename)
		}
	}

	return nil
}

func writeTemplate(content, dest string, info *Info) error {
	if err := os.MkdirAll(filepath.Dir(dest), 0755); err != nil {
		return err
	}

	file, err := os.Create(dest)
	if err != nil {
		log.Fatalln(err)
	}
	defer file.Close()

	err = template.Must(template.New("").Parse(content)).
		Execute(file, info)
	if err != nil {
		return err
	}

	return nil
}
