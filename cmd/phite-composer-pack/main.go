package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/ohmiler/phite/internal/composerpack"
)

func main() {
	if len(os.Args) < 2 || os.Args[1] != "assemble" {
		fmt.Fprintln(os.Stderr, "Usage: phite-composer-pack assemble --recipe <path> --artifact <path> --output <directory>")
		os.Exit(2)
	}
	flags := flag.NewFlagSet("assemble", flag.ContinueOnError)
	recipe := flags.String("recipe", "", "path to the Composer assembly recipe")
	artifact := flags.String("artifact", "", "path to the downloaded Composer PHAR")
	output := flags.String("output", "", "directory for assembled release assets")
	if err := flags.Parse(os.Args[2:]); err != nil {
		os.Exit(2)
	}
	if *recipe == "" || *artifact == "" || *output == "" || flags.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "Usage: phite-composer-pack assemble --recipe <path> --artifact <path> --output <directory>")
		os.Exit(2)
	}
	version, err := composerpack.Assemble(*recipe, *artifact, *output)
	if err != nil {
		fmt.Fprintf(os.Stderr, "assemble Composer: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("assembled Composer %s\n", version)
}
