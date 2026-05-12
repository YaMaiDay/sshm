package sshconfig

import "os"

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
