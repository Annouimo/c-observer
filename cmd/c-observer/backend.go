package main

import "github.com/Routilet/c-observer/internal/backend"

func supportedBackends(platform string) []string {return backend.SupportedBackends(platform)}
func hasTool(name string) bool {return backend.HasTool(name)}
func chooseBackend(platform,requested,compiler,debugger string,exists func(string)bool)(string,error){
 return backend.ChooseBackend(platform,requested,compiler,debugger,exists)
}
type selectedEngine = backend.SelectedEngine
