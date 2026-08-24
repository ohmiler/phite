package main

import (
	"fmt"
	"os"
)

var version = "dev"

func main() {
	if len(os.Args) == 2 && os.Args[1] == "version" {
		fmt.Printf("Phite CLI %s\n", version)
		return
	}

	fmt.Fprintln(os.Stderr, "Usage: phite version")
	os.Exit(2)
}
