package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
)

func main() {
	input, err := io.ReadAll(os.Stdin)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(90)
	}
	workingDirectory, err := os.Getwd()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(91)
	}
	arguments, err := json.Marshal(os.Args[1:])
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(92)
	}

	fmt.Printf("cwd=%s\nargs=%s\nphprc=%s\nscan=%s\nstdin=%s", workingDirectory, arguments, os.Getenv("PHPRC"), os.Getenv("PHP_INI_SCAN_DIR"), input)
	fmt.Fprintln(os.Stderr, "fake php stderr")

	for _, argument := range os.Args[1:] {
		if !strings.HasPrefix(argument, "--exit=") {
			continue
		}
		code, err := strconv.Atoi(strings.TrimPrefix(argument, "--exit="))
		if err != nil {
			os.Exit(93)
		}
		os.Exit(code)
	}
}
