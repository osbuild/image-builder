package container

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// Inspect a target ref, either local or remote, returning its manifest. Uses
// 'skopeo'.
func (cl *Client) skopeoInspect(ctx context.Context, target string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, "skopeo", "inspect", "--raw")

	if tls := cl.GetTLSVerify(); tls != nil && !*tls {
		cmd.Args = append(cmd.Args, "--tls-verify=false")
	}

	if authfile := cl.GetAuthFilePath(); authfile != "" {
		cmd.Args = append(cmd.Args, fmt.Sprintf("--authfile=%s", authfile))
	}

	cmd.Args = append(cmd.Args, target)

	stdout := new(bytes.Buffer)
	stderr := new(bytes.Buffer)
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	cmd.WaitDelay = 5 * time.Second

	if err := cmd.Run(); err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			// the base error in this case isn't informative, so wrap and return the context error instead
			err = ctxErr
		}
		retErr := fmt.Errorf("skopeo inspect %s: %w", target, err)
		if errMsg := strings.TrimSpace(stderr.String()); errMsg != "" {
			retErr = fmt.Errorf("skopeo inspect %s: %s (%w)", target, errMsg, err)
		}
		return nil, retErr
	}
	return stdout.Bytes(), nil
}

// skopeoInspectLocalStorage constructs an image name with the
// containers-storage transport for the given target, inspects it using skopeo,
// and returns the manifest.
func (cl *Client) skopeoInspectLocalStorage(ctx context.Context, target string) ([]byte, error) {
	return cl.skopeoInspect(ctx, fmt.Sprintf("containers-storage:[overlay@%s+%s]%s", cl.store, cl.runroot, target))
}
