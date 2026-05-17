package sshconfig

import (
	"os"
	"os/exec"
	"strings"
)

func TempPasswordFile(password string) (string, error) {
	file, err := os.CreateTemp("", "sshm-pass-*")
	if err != nil {
		return "", err
	}
	defer file.Close()
	if err := file.Chmod(0600); err != nil {
		_ = os.Remove(file.Name())
		return "", err
	}
	if _, err := file.WriteString(password + "\n"); err != nil {
		_ = os.Remove(file.Name())
		return "", err
	}
	return file.Name(), nil
}

func SSHPassArgs(password string, command string, passwordArgs []string, args []string) ([]string, func(), bool) {
	if strings.TrimSpace(password) == "" {
		return nil, func() {}, false
	}
	if _, err := exec.LookPath("sshpass"); err != nil {
		return nil, func() {}, false
	}
	file, err := TempPasswordFile(password)
	if err != nil {
		return nil, func() {}, false
	}
	cleanup := func() { _ = os.Remove(file) }
	fullArgs := append([]string{"-f", file, command}, passwordArgs...)
	fullArgs = append(fullArgs, args...)
	return fullArgs, cleanup, true
}
