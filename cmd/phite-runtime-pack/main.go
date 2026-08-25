package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/ohmiler/phite/internal/runtimepack"
)

func main() {
	if len(os.Args) == 1 || os.Args[1] != "assemble" {
		fmt.Fprintln(os.Stderr, "Usage: phite-runtime-pack assemble --recipe <path> --artifact <path> --output <directory>")
		os.Exit(2)
	}

	flags := flag.NewFlagSet("assemble", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	recipe := flags.String("recipe", "", "path to the runtime assembly recipe")
	artifact := flags.String("artifact", "", "path to the downloaded runtime artifact")
	output := flags.String("output", "", "directory for release assets")
	if err := flags.Parse(os.Args[2:]); err != nil {
		os.Exit(2)
	}
	if flags.NArg() != 0 || *recipe == "" || *artifact == "" || *output == "" {
		fmt.Fprintln(os.Stderr, "Usage: phite-runtime-pack assemble --recipe <path> --artifact <path> --output <directory>")
		os.Exit(2)
	}

	identity, err := runtimepack.Assemble(*recipe, *artifact, *output)
	if err != nil {
		fmt.Fprintf(os.Stderr, "assemble runtime: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("assembled %s\n", identity)
}
