package bootstrapper

import (
	_ "embed"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"text/template"

	"github.com/BurntSushi/toml"
	"platform.prodigy9.co/builder"
	"platform.prodigy9.co/project"
)

var (
	//go:embed buildkite.pipeline.yaml.template
	buildkitePipelineYamlTemplate string
	//go:embed platform.template
	platformTemplate string
)

type Info struct {
	ProjectName     string
	Maintainer      string
	MaintainerEmail string
	Repository      string
	ImagePrefix     string
	GoVersion       string // TODO: Probably should detect from user's environment
}

func Bootstrap(dir string, info *Info) error {
	dir, err := resolveWD(dir)
	if err != nil {
		return err
	}

	// generate platform.toml
	proj := *project.ProjectDefaults
	proj.Maintainer = fmt.Sprintf("%s <%s>", info.Maintainer, info.MaintainerEmail)
	proj.Repository = info.Repository

	mods, err := builder.Discover(dir)
	for name, builder := range mods {
		mod := *project.ModuleDefaults
		mod.WorkDir = "./" + name
		mod.Builder = builder.Name()
		proj.Modules[name] = &mod
	}

	outfilename := filepath.Join(dir, "platform.toml")
	projfile, err := os.Create(outfilename)
	if err != nil {
		return err
	}
	defer projfile.Close()
	if err := toml.NewEncoder(projfile).Encode(&proj); err != nil {
		return err
	}
	log.Println("wrote: platform.toml")

	// generate platform script
	outfilename = filepath.Join(dir, "platform")
	if err := writeTemplate(platformTemplate, outfilename, info); err != nil {
		return err
	} else if err := os.Chmod(outfilename, 0744); err != nil { // make executable
		return err
	}
	log.Println("wrote: platform")

	// generate .buildkite/pipeline.yaml
	outfilename = filepath.Join(dir, ".buildkite", "pipeline.yaml")
	if err := writeTemplate(buildkitePipelineYamlTemplate, outfilename, info); err != nil {
		return err
	}
	log.Println("wrote: .buildkite/pipeline.yaml")

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

func resolveWD(wd string) (string, error) {
	if wd == "" {
		if wd_, err := os.Getwd(); err != nil {
			return "", err
		} else {
			wd = wd_
		}
	}

	if !filepath.IsAbs(wd) {
		if abs, err := filepath.Abs(wd); err != nil {
			return "", err
		} else {
			wd = abs
		}
	}

	return wd, nil
}
