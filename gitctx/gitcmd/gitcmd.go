package gitcmd

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"platform.prodigy9.co/internal/plog"
)

func Log(wd string) (string, error) {
	return runCmd(wd, "git", "log", "--pretty=%h %s")
}
func LogRange(wd string, range_ string) (string, error) {
	return runCmd(wd, "git", "log", "--pretty=%h %s", range_)
}

func FetchFTags(wd, origin string, tags []string) (string, error) {
	args := []string{"fetch", "-f", origin}
	for _, tag := range tags {
		args = append(args, "refs/tags/"+tag+":refs/tags/"+tag)
	}

	return runCmd(wd, "git", args...)
}
func FetchTags(wd, origin string) (string, error) {
	return runCmd(wd, "git", "fetch", "--tags", origin)
}
func ListTags(wd, pattern string) (string, error) {
	return runCmd(wd, "git", "tag", "-l", pattern)
}
func Tag(wd string, tagname, message string) (string, error) {
	return runCmd(wd, "git", "tag", "-a", "-m", message, tagname)
}
func TagF(wd string, tagname string) (string, error) {
	return runCmd(wd, "git", "tag", "-f", tagname)
}
func PushTag(wd string, remote, tagname string) (string, error) {
	return runCmd(wd, "git", "push", "--porcelain", remote, tagname)
}
func PushTagF(wd string, remote, tagname string) (string, error) {
	return runCmd(wd, "git", "push", "--porcelain", "-f", remote, tagname)
}
func TagMessage(wd string, tagname string) (string, error) {
	return runCmd(wd, "git", "tag", "-l", "--format=%(contents)", tagname)
}

func Status(wd string) (string, error) {
	return runCmd(wd, "git", "status", "--porcelain")
}
func Describe(wd string) (string, error) {
	return runCmd(wd, "git", "describe", "--always", "--dirty", "--broken")
}
func CurrentBranch(wd string) (string, error) {
	return runCmd(wd, "git", "branch", "--show-current")
}
func TrackingRemote(wd string, branch string) (string, error) {
	if branch == "" {
		branch = "main"
	}
	return runCmd(wd, "git", "config", "branch."+branch+".remote")
}

func runCmd(wd, name string, args ...string) (string, error) {
	wd, err := filepath.Abs(wd)
	if err != nil {
		return "", err
	}

	plog.Command(name, args...)
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
