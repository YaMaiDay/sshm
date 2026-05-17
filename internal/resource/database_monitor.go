package resource

import (
	"strings"

	"github.com/YaMaiDay/sshm/internal/config"
	"github.com/YaMaiDay/sshm/internal/dbmonitor"
)

func DatabaseDefaultPort(engine string) string {
	return dbmonitor.DefaultPort(engine)
}

func DatabaseEngineChoices() []string {
	return dbmonitor.EngineChoices()
}

func NormalizeDatabaseEngine(engine string) string {
	return dbmonitor.NormalizeEngine(engine)
}

func DatabaseDefaultUser(engine string) string {
	return dbmonitor.DefaultUser(engine)
}

func DatabaseDefaultName(engine string) string {
	return dbmonitor.DefaultName(engine)
}

func DatabaseMissingCredentialHint(item config.ManagedResource, errText string) bool {
	return dbmonitor.MissingCredentialHint(item, errText)
}

func DatabaseEngineFromDefaultPort(port string) string {
	switch strings.TrimSpace(port) {
	case "3306":
		return "MySQL"
	case "5432":
		return "PostgreSQL"
	case "6379":
		return "Redis"
	case "27017", "27018", "27019":
		return "MongoDB"
	default:
		return ""
	}
}
