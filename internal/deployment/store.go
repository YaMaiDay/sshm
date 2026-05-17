package deployment

import "github.com/YaMaiDay/sshm/internal/config"

func LoadFile(home string) (config.DeploymentsFile, bool, error) {
	return config.LoadDeployments(home)
}

func SaveFile(home string, file config.DeploymentsFile) error {
	return config.SaveDeployments(home, file)
}

func SaveApp(home string, file config.DeploymentsFile, index int, app config.DeploymentApp) (config.DeploymentsFile, error) {
	if err := config.ValidateDeploymentApp(app); err != nil {
		return file, err
	}
	if index >= 0 && index < len(file.Apps) {
		file.Apps[index] = app
	} else {
		file.Apps = append(file.Apps, app)
	}
	if err := SaveFile(home, file); err != nil {
		return file, err
	}
	return file, nil
}

func DeleteApp(home string, file config.DeploymentsFile, index int) (config.DeploymentsFile, bool, error) {
	if index < 0 || index >= len(file.Apps) {
		return file, false, nil
	}
	file.Apps = append(file.Apps[:index], file.Apps[index+1:]...)
	if err := SaveFile(home, file); err != nil {
		return file, false, err
	}
	return file, true, nil
}

func AppendRecord(home string, record config.DeploymentRecord) (config.DeploymentsFile, error) {
	if err := config.AppendDeploymentRecord(home, record); err != nil {
		return config.DeploymentsFile{}, err
	}
	file, _, err := LoadFile(home)
	return file, err
}
