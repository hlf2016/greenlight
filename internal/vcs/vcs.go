package vcs

import (
	"fmt"
	"runtime/debug"
)

func Version() string {

	var revision string
	var modified bool

	bi, ok := debug.ReadBuildInfo()
	if ok {
		for _, s := range bi.Settings {
			switch {
			case s.Key == "vcs.revision":
				revision = s.Value
			case s.Key == "vcs.modified":
				if s.Value == "true" {
					modified = true
				}
			}
		}
	}

	if modified {
		return fmt.Sprintf("%s-dirty", revision)
	}
	return revision
}
