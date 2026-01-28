package main

import (
	"fmt"
	"main/src/lib"
)

func main() {
	config := lib.LoadConfig()
	fmt.Printf("Config: %+v\n", config.MongoDBURI)
	
}