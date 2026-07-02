package cli

import (
	"fmt"
	"io"
	"os"
)

const (
	brandName    = "rest"
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
	spark := ""
	reset := ""
	if color {
		accent = "\x1b[36m"
		muted = "\x1b[2m"
		spark = "\x1b[32m"
		reset = "\x1b[0m"
	}
	fmt.Fprintf(w, `
%s  ✦ %s%s%s
%s    %s%s

`, spark, accent, brandName, reset, muted, brandTagline, reset)
}
