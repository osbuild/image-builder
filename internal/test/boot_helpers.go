package test

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

func run(c string, args ...string) ([]byte, []byte, error) {
	fmt.Printf("> %s %s\n", c, strings.Join(args, " "))
	cmd := exec.Command(c, args...)

	var cmdout, cmderr bytes.Buffer
	cmd.Stdout = &cmdout
	cmd.Stderr = &cmderr
	err := cmd.Run()

	// print any output even if the call failed
	stdout := cmdout.Bytes()
	if len(stdout) > 0 {
		fmt.Println(string(stdout))
	}

	stderr := cmderr.Bytes()
	if len(stderr) > 0 {
		fmt.Fprintf(os.Stderr, "%s\n", string(stderr))
	}
	return stdout, stderr, err
}

func SshRun(ip, user, key, hostsfile string, command ...string) error {
	sshargs := []string{"-i", key, "-o", fmt.Sprintf("UserKnownHostsFile=%s", hostsfile), "-l", user, ip}
	sshargs = append(sshargs, command...)
	_, _, err := run("ssh", sshargs...)
	if err != nil {
		return err
	}
	return nil
}

// SshRunUntilReady runs an SSH command repeatedly until user authentication
// succeeds. It is useful while cloud-init or a guest agent is still
// provisioning the user and its authorized key.
func SshRunUntilReady(ip, user, key, hostsfile string, command ...string) error {
	const retryInterval = 5 * time.Second
	const retries = 24

	var err error
	for attempt := 0; attempt <= retries; attempt++ {
		err = SshRun(ip, user, key, hostsfile, command...)
		if err == nil {
			return nil
		}
		if attempt < retries {
			time.Sleep(retryInterval)
		}
	}
	return fmt.Errorf("SSH authentication did not succeed within 2 minutes: %w", err)
}

func IsRegularFile(path string) bool {
	fileInfo, err := os.Stat(path)
	if err != nil {
		return false
	}
	return fileInfo.Mode().IsRegular()
}

func ScpFile(ip, user, key, hostsfile, source, dest string) error {
	_, _, err := run("scp", "-i", key, "-o", fmt.Sprintf("UserKnownHostsFile=%s", hostsfile), "--", source, fmt.Sprintf("%s@%s:%s", user, ip, dest))
	if err != nil {
		return err
	}
	return nil
}

func Keyscan(ip, filepath string) error {
	var keys []byte
	maxTries := 30 // wait for at least 5 mins
	var keyscanErr error
	for try := 0; try < maxTries; try++ {
		keys, _, keyscanErr = run("ssh-keyscan", ip)
		if keyscanErr == nil {
			break
		}
		time.Sleep(10 * time.Second)
	}
	if keyscanErr != nil {
		return keyscanErr
	}

	fmt.Printf("Creating known hosts file: %s\n", filepath)
	hostsFile, err := os.Create(filepath)
	if err != nil {
		return err
	}

	fmt.Printf("Writing to known hosts file: %s\n", filepath)
	if _, err := hostsFile.Write(keys); err != nil {
		return err
	}
	return nil
}
