package main

import (
	"main/src/lib"
)

func main() {
	config := lib.LoadConfig()
	
	lib.AppLogger.Init()
	lib.AppLogger.SetLogLevel(config.LogLevel)
	
}