package remotescript

import (
	"fmt"
	"strings"
)

type UserScript string

func NewUserScript(value string) UserScript {
	return UserScript(value)
}

func (script UserScript) String() string {
	return string(script)
}

func UserCommand(value string) string {
	return strings.TrimSpace(value)
}

func Quote(value string) string {
	if value == "" {
		return "''"
	}
	if strings.IndexFunc(value, func(r rune) bool {
		return !(r == '_' || r == '-' || r == '/' || r == '.' || r == ':' || r == '=' || r == ',' || r == '@' ||
			(r >= '0' && r <= '9') || (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z'))
	}) == -1 {
		return value
	}
	return SingleQuote(value)
}

func SingleQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", `'\''`) + "'"
}

func EnvName(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	for i, r := range value {
		if i == 0 {
			if !(r == '_' || r >= 'A' && r <= 'Z' || r >= 'a' && r <= 'z') {
				return ""
			}
			continue
		}
		if !(r == '_' || r >= 'A' && r <= 'Z' || r >= 'a' && r <= 'z' || r >= '0' && r <= '9') {
			return ""
		}
	}
	return value
}

func SudoFallback(command string, sudoCommand string) string {
	return fmt.Sprintf(`out=$(%s 2>&1)
code=$?
if [ "$code" -ne 0 ]; then
  first="$out"
  out=$(%s 2>&1)
  code=$?
  if [ "$code" -ne 0 ]; then
    case "$first $out" in
      *"permission denied"*|*"Permission denied"*|*"not in the docker group"*|*"password is required"*|*"a password is required"*|*"Authentication is required"*) echo "__SSHM_PERMISSION_DENIED__" ;;
    esac
  fi
fi
printf '%%s\n' "$out"
exit "$code"`, command, sudoCommand)
}
