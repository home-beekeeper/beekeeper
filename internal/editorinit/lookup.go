package editorinit

import "os/exec"

// execLookPath is the real implementation of path-based executable lookup.
// It is called by defaultLookPath and is separated into its own file so that
// detect.go imports only os/filepath/runtime while tests can override lookPath
// without needing exec.
func execLookPath(name string) (string, error) {
	return exec.LookPath(name)
}
