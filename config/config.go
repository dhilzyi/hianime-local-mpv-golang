package config

import (
	"encoding/json"
	"fmt"
	"os"
)

var FileName string = "config.json"

type Settings struct {
	JimakuEnable     bool `json:"jimaku_enable"`
	AutoSelectServer bool `json:"auto_selectserver"`
}

func LoadConfig() (Settings, error) {
	var config_session Settings

	if _, err := os.Stat(FileName); err == nil {
		fmt.Println("File config load success.")
		jsonData, err := os.ReadFile(FileName)
		if err != nil {
			return config_session, fmt.Errorf("Failed to open json files: %w", err)
		}

		if err = json.Unmarshal(jsonData, &config_session); err != nil {
			return config_session, fmt.Errorf("Failed to convert to struct: %w", err)
		}

	} else if os.IsNotExist(err) {
		_, err := os.Create(FileName)

		SaveConfig(config_session)

		if err != nil {
			return config_session, fmt.Errorf("Failed to create history json file: %w", err)
		}
	} else {
		return config_session, fmt.Errorf("Error accessing path %s: %w\n", FileName, err)
	}

	return config_session, nil
}

func SaveConfig(rawData Settings) error {
	jsonData, err := json.MarshalIndent(rawData, "", " ")
	if err != nil {
		return fmt.Errorf("Failed to save the history files: %w", err)
	}

	if err = os.WriteFile(FileName, jsonData, os.ModePerm); err != nil {
		return fmt.Errorf("Failed to write history files: %w", err)
	}

	return nil

}
