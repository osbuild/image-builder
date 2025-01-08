package unleash

import (
	"log/slog"
)

type LogListener struct{}

// OnError prints out errors.
func (l LogListener) OnError(err error) {
	slog.Error("unleash Error", "err", err)
}

// OnWarning prints out warning.
func (l LogListener) OnWarning(err error) {
	slog.Warn("unleash warning", "warn", err)
}

// OnReady prints to the console when the repository is ready.
func (l LogListener) OnReady() {
	slog.Info("unleash is ready")
}
