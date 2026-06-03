package main

import (
	"fmt"
	"io"
	// "runtime"
)

// Build information. These are overridden at build time via:
//
//	go build -ldflags "-X main.version=v0.2.0 -X main.buildDate=2026-06-03T12:00:00Z"
var (
	version   = "dev"
	buildDate = "undefined"
)

// printVersion writes the version, build date and Go toolchain to w.
func printVersion(w io.Writer) {
	fmt.Fprintf(w, "nextcloud-ical-backup %s\n", version)
	fmt.Fprintf(w, "build date: %s\n", buildDate)
	// fmt.Fprintf(w, "go:         %s (%s/%s)\n", runtime.Version(), runtime.GOOS, runtime.GOARCH)
}
