package main

import (
	"os"

	"github.com/osbuild/osbuild-installer/internal/server"
)

func main() {
	// Look for a custom listen address in environment variables.
	listenAddress, ok := os.LookupEnv("LISTEN_ADDRESS")

	// Listen on localhost otherwise.
	if !ok {
		listenAddress = "localhost:8086"
	}

	// Run the server and listen for requests.
	server.Run(listenAddress)
}
