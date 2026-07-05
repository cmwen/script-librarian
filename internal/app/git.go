package app

import (
	"bytes"
	"errors"
	"os/exec"
)

func gitAvailable() bool {
	_, err := exec.LookPath("git")
	return err == nil
}

func gitRun(dir string, args ...string) (string, error) {
	if !gitAvailable() {
		return "", errors.New("git is not available on PATH")
	}
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	var out bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err != nil {
		if stderr.Len() > 0 {
			return out.String() + stderr.String(), err
		}
		return out.String(), err
	}
	return out.String(), nil
}

func gitInit(dir string) error {
	_, err := gitRun(dir, "rev-parse", "--is-inside-work-tree")
	if err == nil {
		return nil
	}
	_, err = gitRun(dir, "init")
	return err
}

func gitCommitFile(dir, path, message string) error {
	if !gitAvailable() {
		return nil
	}
	if _, err := gitRun(dir, "add", path); err != nil {
		return err
	}
	_, err := gitRun(dir, "-c", "user.name=Script Librarian", "-c", "user.email=script-librarian@localhost", "commit", "-m", message)
	return err
}
