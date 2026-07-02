package cli

import (
	"fmt"
	"io"
	"os"
)

const (
	brandLead    = "Give yourself a little"
	brandTagline = "Write queries. Get an application. Add business logic."
)

func maybePrintInitWelcome() {
	if !isTerminalFile(os.Stdout) {
		return
	}
	printWelcomeBanner(os.Stdout, true)
}

func printWelcomeBanner(w io.Writer, color bool) {
	accent := ""
	muted := ""
	reset := ""
	if color {
		accent = "\x1b[36m"
		muted = "\x1b[2m"
		reset = "\x1b[0m"
	}
	fmt.Fprintf(w, `
%s  %s%s
%s   ____  _____ ____ _____%s
%s  |  _ \| ____/ ___|_   _|%s
%s  | |_) |  _| \___ \ | |%s     %s%s%s
%s  |  _ <| |___ ___) || |%s     %s%s%s
%s  |_| \_\_____|____/ |_|%s     %s%s%s

`, muted, brandLead, reset,
		accent, reset,
		accent, reset,
		accent, reset, muted, "Write queries.", reset,
		accent, reset, muted, "Get an application.", reset,
		accent, reset, muted, "Add business logic.", reset)
}
