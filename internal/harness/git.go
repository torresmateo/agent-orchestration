package harness

import (
	"fmt"
	"os/exec"
)

type Git struct {
	dir string
}

func NewGit(dir string) *Git {
	return &Git{dir: dir}
}

func (g *Git) run(args ...string) error {
	cmd := exec.Command("git", args...)
	cmd.Dir = g.dir
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git %v: %w\n%s", args, err, output)
	}
	return nil
}

func (g *Git) Clone(url, dest string) error {
	cmd := exec.Command("git", "clone", url, dest)
	cmd.Dir = g.dir
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git clone: %w\n%s", err, output)
	}
	return nil
}

func (g *Git) CreateBranch(name string) error {
	return g.run("checkout", "-b", name)
}

func (g *Git) AddAll() error {
	return g.run("add", "-A")
}

func (g *Git) Commit(message string) error {
	return g.run("commit", "-m", message, "--allow-empty")
}

func (g *Git) Push(branch string) error {
	return g.run("push", "origin", branch)
}

func (g *Git) CurrentBranch() (string, error) {
	cmd := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD")
	cmd.Dir = g.dir
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return string(output), nil
}
