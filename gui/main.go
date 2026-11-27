package main

import (
 "embed"
 "log"

 "github.com/wailsapp/wails/v3/pkg/application"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
 app := application.New(application.Options{
  Name: "C Observer Preview",
  Description: "Offline execution document preview",
  Assets: application.AssetOptions{
   Handler: application.AssetFileServerFS(assets),
  },
  Mac: application.MacOptions{
   ApplicationShouldTerminateAfterLastWindowClosed: true,
  },
 })
 app.Window.NewWithOptions(application.WebviewWindowOptions{
  Title: "C Observer Preview",
  Width: 980,
  Height: 720,
  URL: "/",
 })
 if err := app.Run(); err != nil {
  log.Fatal(err)
 }
}
