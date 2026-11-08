package main

import (
	"github.com/egoist/mygo"
	"github.com/egoist/mygo/plugins/updater"
	updatenative "github.com/egoist/mygo/plugins/updater/native"
)

// checkForUpdates is the plugin's check, which tests replace: it panics
// without the plugin, which App.Run sets up.
var checkForUpdates = updater.CheckForUpdates

// useUpdater checks the releases of the tool's repository for a newer
// version soon after launch and then once a day, and offers it in a window
// of native UI, in Chinese. It must run before App.Run.
func useUpdater() {
	mygo.Use(updatenative.New(updater.Options{Language: "zh-Hans"}))
}
