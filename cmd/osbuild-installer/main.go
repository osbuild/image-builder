package main

import (
	"github.com/osbuild/osbuild-installer/internal/server"
)

func main() {
	server.Run("localhost:8086")
}
