package progress

import (
	"os"
	"os/signal"
	"sync"
	"syscall"

	"github.com/mattn/go-isatty"
	"golang.org/x/term"
)

var isattyIsTerminal = isatty.IsTerminal

var (
	terminalWidth  int
	terminalHeight int

	sizeMutex sync.RWMutex
)

func init() {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGWINCH)

	go func() {
		for {
			<-sigChan
			updateTerminalSize(os.Stdout.Fd())
		}
	}()
}

// updateTerminalSize updates the current terminal size.
func updateTerminalSize(fd uintptr) {
	width, height, err := term.GetSize(int(fd))
	if err != nil {
		width = 0
		height = 0
	}

	sizeMutex.Lock()
	defer sizeMutex.Unlock()

	terminalWidth = width
	terminalHeight = height
}

// getTerminalSize safely reads the stored terminal size.
var getTerminalSize = func() (int, int) {
	sizeMutex.RLock()
	defer sizeMutex.RUnlock()

	return terminalWidth, terminalHeight
}
