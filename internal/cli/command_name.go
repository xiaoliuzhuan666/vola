package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	canonicalRootCommand = "vola"
	shortRootCommand     = "vol"
	legacyRootCommand    = "xlzdrive"
	neuRootCommand       = "neu"
)

func rootCommand() string {
	base := strings.TrimSpace(filepath.Base(os.Args[0]))
	switch base {
	case canonicalRootCommand, shortRootCommand, legacyRootCommand, neuRootCommand:
		return base
	default:
		return canonicalRootCommand
	}
}

func usageLine(args string) string {
	return fmt.Sprintf("usage: %s %s", rootCommand(), args)
}

func renderCLIText(text string) string {
	cmd := rootCommand()
	pairs := make([]string, 0, 128)
	for _, source := range []string{canonicalRootCommand, legacyRootCommand} {
		pairs = append(pairs,
			"usage: "+source+" ", "usage: "+cmd+" ",
			"Usage: "+source+" ", "Usage: "+cmd+" ",
			"\n  "+source+" ", "\n  "+cmd+" ",
			"\n       "+source+" ", "\n       "+cmd+" ",
			"\n"+source+" ", "\n"+cmd+" ",
			"`"+source+" ", "`"+cmd+" ",
			" "+source+" help", " "+cmd+" help",
			" "+source+" ls", " "+cmd+" ls",
			" "+source+" read", " "+cmd+" read",
			" "+source+" write", " "+cmd+" write",
			" "+source+" search", " "+cmd+" search",
			" "+source+" create", " "+cmd+" create",
			" "+source+" log", " "+cmd+" log",
			" "+source+" import", " "+cmd+" import",
			" "+source+" token", " "+cmd+" token",
			" "+source+" stats", " "+cmd+" stats",
			" "+source+" platform", " "+cmd+" platform",
			" "+source+" connect", " "+cmd+" connect",
			" "+source+" disconnect", " "+cmd+" disconnect",
			" "+source+" export", " "+cmd+" export",
			" "+source+" browse", " "+cmd+" browse",
			" "+source+" status", " "+cmd+" status",
			" "+source+" doctor", " "+cmd+" doctor",
			" "+source+" daemon", " "+cmd+" daemon",
			" "+source+" sync", " "+cmd+" sync",
			" "+source+" server", " "+cmd+" server",
			" "+source+" mcp", " "+cmd+" mcp",
			"with "+source+" ", "with "+cmd+" ",
		)
	}
	replacer := strings.NewReplacer(pairs...)
	return replacer.Replace(text)
}
