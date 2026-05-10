package sshconfig

import (
	"os/exec"
	"sync"
)

var (
	warnWeakCryptoOnce      sync.Once
	warnWeakCryptoSupported bool
)

func WarnWeakCryptoNoPQKexArgs() []string {
	warnWeakCryptoOnce.Do(func() {
		cmd := exec.Command("ssh", "-G", "-o", "WarnWeakCrypto=no-pq-kex", "example.com")
		warnWeakCryptoSupported = cmd.Run() == nil
	})
	if !warnWeakCryptoSupported {
		return nil
	}
	return []string{"-o", "WarnWeakCrypto=no-pq-kex"}
}
