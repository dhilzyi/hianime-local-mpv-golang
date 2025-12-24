package config

import (
	"encoding/json"
	"fmt"
	"os"
)

var FileName string = "config.json"
var DebugMode bool

type Settings struct {
	JimakuEnable     bool   `json:"jimaku_enable"`     // for enabling jimaku
	AutoSelectServer bool   `json:"auto_selectserver"` // whether user want use auto select server or manual input server
	MpvPath          string `json:"mpv_path"`          // manually set mpv path command
	EnglishOnly      bool   `json:"english_only"`      // whether user want importing english subtitle only or not into mpv
}

func LoadConfig() (Settings, error) {
	var configSession Settings

	if _, err := os.Stat(FileName); err == nil {
		fmt.Println("File config load success.")
		jsonData, err := os.ReadFile(FileName)
		if err != nil {
			return configSession, fmt.Errorf("Failed to open json files: %w", err)
		}

		if err = json.Unmarshal(jsonData, &configSession); err != nil {
			return configSession, fmt.Errorf("Failed to convert to struct: %w", err)
		}

	} else if os.IsNotExist(err) {
		_, err := os.Create(FileName)

		configSession = Settings{
			JimakuEnable:     true,
			AutoSelectServer: true,
			MpvPath:          "",
			EnglishOnly:      true,
		}

		SaveConfig(configSession)

		if err != nil {
			return configSession, fmt.Errorf("Failed to create history json file: %w", err)
		}
	} else {
		return configSession, fmt.Errorf("Error accessing path %s: %w\n", FileName, err)
	}

	return configSession, nil
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
