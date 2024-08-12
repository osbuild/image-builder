package unleash

import (
	"github.com/sirupsen/logrus"
)

type LogListener struct{}

// OnError prints out errors.
func (l LogListener) OnError(err error) {
	logrus.Errorf("Unleash Error: %v", err)
}

// OnWarning prints out warning.
func (l LogListener) OnWarning(err error) {
	logrus.Warnf("Unleash warning: %v", err)
}

// OnReady prints to the console when the repository is ready.
func (l LogListener) OnReady() {
	logrus.Info("Unleash is ready")
}
