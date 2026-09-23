// Command protogen writes client/src/protocol.gen.ts from internal/protocol.
// Run it with `go generate ./internal/protocol`.
package main

import (
	"flag"
	"log"
	"os"

	"nytrpg/internal/tsgen"
)

func main() {
	src := flag.String("src", ".", "Go package directory to read")
	out := flag.String("out", "", "TypeScript file to write")
	flag.Parse()
	if *out == "" {
		log.Fatal("-out is required")
	}

	ts, err := tsgen.Generate(*src)
	if err != nil {
		log.Fatal(err)
	}
	if err := os.WriteFile(*out, ts, 0644); err != nil {
		log.Fatal(err)
	}
}
