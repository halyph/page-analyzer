package main

import (
	"fmt"
	"os"
)

var (
	version = "dev"
	commit  = "unknown"
	date    = "unknown"
)

func main() {
	fmt.Printf("Page Analyzer %s (%s) built at %s\n", version, commit, date)
	fmt.Println("Ready for implementation!")
	os.Exit(0)
}
