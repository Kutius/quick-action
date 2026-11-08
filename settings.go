package main

import (
	"encoding/json"
	"log"
	"os"
	"path/filepath"

	"github.com/egoist/mygo"
)

// settings are what the tool remembers between runs, besides where its card
// is: the output the user switched away from, so that a click on the card
// switches back to it, and the shortcuts the tool answers to.
type settings struct {
	PreviousOutput string `json:"previousOutput"`
	// Hotkeys are the shortcuts of the microphone and of the output, in
	// that order, as accelerators. None means the user never picked any,
	// which the tool fills in with its own; a blank one was cleared.
	Hotkeys []string `json:"hotkeys,omitempty"`
}

// settingsFile is the file settings are kept in, in the app's user data
// directory.
func settingsFile() string {
	dir, err := mygo.App.Path(mygo.PathUserData)
	if err != nil {
		return ""
	}
	return filepath.Join(dir, "settings.json")
}

func loadSettings() settings {
	var s settings
	path := settingsFile()
	if path == "" {
		return s
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return s
	}
	if json.Unmarshal(data, &s) != nil {
		return settings{}
	}
	return s
}

func saveSettings(s settings) {
	path := settingsFile()
	if path == "" {
		return
	}
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		log.Print("saving settings: ", err)
	}
}
