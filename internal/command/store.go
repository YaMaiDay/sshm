package command

import "github.com/YaMaiDay/sshm/internal/config"

func LoadTemplates(home string) (config.CommandsFile, bool, error) {
	return config.LoadCommands(home)
}

func SaveTemplates(home string, file config.CommandsFile) error {
	return config.SaveCommands(home, file)
}

func LoadHistory(home string) (config.CommandHistoryFile, bool, error) {
	return config.LoadCommandHistory(home)
}

func AppendHistory(home string, entry config.CommandHistoryEntry) error {
	return config.AppendCommandHistory(home, entry)
}

func DeleteHistoryEntry(home string, id string) error {
	return config.DeleteCommandHistoryEntry(home, id)
}
