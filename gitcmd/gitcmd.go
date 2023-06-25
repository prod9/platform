package gitcmd

import (
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func Log(wd string) (string, error) {
	return runCmd(wd, "git", "log", "--pretty=%h %s")
}
func LogRange(wd string, range_ string) (string, error) {
	return runCmd(wd, "git", "log", "--pretty=%h %s", range_)
}
func ListTags(wd string) (string, error) {
	return runCmd(wd, "git", "tag", "-l")
}
func Tag(wd string, tagname, message string) (string, error) {
	return runCmd(wd, "git", "tag", "-a", "-m", message, tagname)
}
func PushTag(wd string, tagname string) (string, error) {
	return runCmd(wd, "git", "push", "--porcelain", tagname)
}
func Status(wd string) (string, error) {
	return runCmd(wd, "git", "status", "--porcelain")
}
func Describe(wd string) (string, error) {
	return runCmd(wd, "git", "describe", "--always", "--all", "--dirty", "--broken")
}

func runCmd(wd, name string, args ...string) (string, error) {
	wd, err := filepath.Abs(wd)
	if err != nil {
		return "", err
	}

	log.Println(name + " " + strings.Join(args, " "))
	outbuf := &strings.Builder{}

	cmd := exec.Command(name, args...)
	cmd.Dir = wd
	cmd.Stdout = outbuf
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return "", err
	}

	return strings.TrimSpace(outbuf.String()), nil
}
