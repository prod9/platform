package bootstrapper

import (
	_ "embed"
	"errors"
	"log"
	"os"
	"path/filepath"
	"text/template"

	"platform.prodigy9.co/project"
)

var (
	//go:embed platform.toml.template
	platformTomlTemplate string

	// TODO: Detect and set default strategy
	//  i.e. go.mod -> go/basic, go.work -> go/workspace
	// TODO: Set the first module to the app's dirname
	templates = map[string]string{
		"platform.toml": platformTomlTemplate,
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

	existing, err := project.ResolvePath(wd)
	if errors.Is(err, project.ErrNoPlatformConfig) {
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
