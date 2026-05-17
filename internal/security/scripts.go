package security

func FailedLoginScript() string {
	return `if ! command -v lastb >/dev/null 2>&1; then
  echo "__SSHM_LASTB_UNAVAILABLE__"
  exit 0
fi
out=$(lastb -n 100 2>&1)
code=$?
if [ "$code" -ne 0 ]; then
  out=$(sudo -n lastb -n 100 2>&1)
  code=$?
fi
if [ "$code" -ne 0 ]; then
  echo "__SSHM_LASTB_PERMISSION__"
  printf '%s\n' "$out"
  exit 0
fi
printf '%s\n' "$out"`
}

func SSHDSecurityScript() string {
	return `if command -v sshd >/dev/null 2>&1; then
  sshd -T 2>/dev/null | awk '/^(passwordauthentication|permitrootlogin|pubkeyauthentication) / {print $1"="$2}'
elif [ -x /usr/sbin/sshd ]; then
  /usr/sbin/sshd -T 2>/dev/null | awk '/^(passwordauthentication|permitrootlogin|pubkeyauthentication) / {print $1"="$2}'
fi`
}
