package unleash

import (
	"fmt"
	"net/http"

	"github.com/Unleash/unleash-client-go/v4"
	ucontext "github.com/Unleash/unleash-client-go/v4/context"
)

type FeatureFlag string

const (
	unleashProjectName = "default"
	unleashAppName     = "image-builder"

	CompliancePolicies FeatureFlag = "image-builder.compliance-policies.enabled"
)

type Config struct {
	URL   string
	Token string
}

func Initialize(conf Config) error {
	err := unleash.Initialize(
		unleash.WithProjectName(unleashProjectName),
		unleash.WithAppName(unleashAppName),
		unleash.WithListener(LogListener{}),
		unleash.WithUrl(conf.URL),
		unleash.WithCustomHeaders(http.Header{"Authorization": {conf.Token}}),
	)
	if err != nil {
		return fmt.Errorf("Unleash error: %w", err)
	}

	return nil
}

func Enabled(flag FeatureFlag) bool {
	ctx := ucontext.Context{}
	return unleash.IsEnabled(string(flag), unleash.WithContext(ctx), unleash.WithFallback(true))
}
