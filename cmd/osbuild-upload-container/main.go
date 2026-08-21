package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/user"
	"path/filepath"

	"github.com/osbuild/image-builder/pkg/container"
)

func upload(filename, destination, tag, username, password string, ignoreTLS bool) error {
	absPath, err := filepath.Abs(filename)
	if err != nil {
		return err
	}

	fmt.Println("Container to upload is:", filename)

	client, err := container.NewClient(destination)

	if err != nil {
		return fmt.Errorf("error creating the upload client: %w", err)
	}

	if password != "" {
		if username == "" {
			u, err := user.Current()
			if err != nil {
				return fmt.Errorf("error looking up current user: %w", err)
			}
			username = u.Username
		}
		client.SetCredentials(username, password)
	}

	if ignoreTLS {
		client.SkipTLSVerify()
	}

	ctx := context.Background()

	from := fmt.Sprintf("oci-archive://%s", absPath)

	digest, err := client.UploadImage(ctx, from, tag)

	if err != nil {
		return fmt.Errorf("error uploading: %w", err)
	}

	fmt.Printf("upload done; destination manifest: %s\n", digest.String())
	return nil
}

func main() {
	var filename string
	var destination string
	var username string
	var password string
	var tag string
	var ignoreTLS bool

	flag.StringVar(&filename, "container", "", "path to the oci-archive to upload (required)")
	flag.StringVar(&destination, "destination", "", "destination to upload to (required)")
	flag.StringVar(&tag, "tag", "", "destination tag to use for the container")
	flag.StringVar(&username, "username", "", "username to use for registry")
	flag.StringVar(&password, "password", "", "password to use for registry")
	flag.BoolVar(&ignoreTLS, "ignore-tls", false, "ignore tls verification for destination")
	flag.Parse()

	if filename == "" || destination == "" {
		flag.Usage()
		os.Exit(1)
	}

	if err := upload(filename, destination, tag, username, password, ignoreTLS); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
