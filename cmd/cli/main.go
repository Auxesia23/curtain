// Command curtain is a CLI client for the Curtain server. It fetches encrypted
// environment variables, decrypts them locally, and injects them into a child
// process, for example `curtain run -- npm run dev`.
//
// Projects, environments, and variables are managed through the web UI.
package main

import (
	"os"

	"curtain/internal/cli"
)

func main() {
	os.Exit(cli.Run(os.Args[1:]))
}
