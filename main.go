package main

import (
	"embed"
	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
)

//go:embed all:frontend
var assets embed.FS

func main() {
	app := &App{}
	err := wails.Run(&options.App{Title: "Molotov Tree Mapper", Width: 1200, Height: 820, MinWidth: 760, MinHeight: 580,
		AssetServer: &assetserver.Options{Assets: assets}, OnStartup: app.startup, Bind: []interface{}{app},
		DragAndDrop: &options.DragAndDrop{EnableFileDrop: true}, BackgroundColour: &options.RGBA{R: 19, G: 23, B: 24, A: 255}})
	if err != nil {
		println(err.Error())
	}
}
