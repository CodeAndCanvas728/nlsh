package main

import (
	_ "embed"
)

//go:embed shells/zsh.sh
var zshIntegrationScript string

// shellIntegration returns the zsh integration script.
//
// It is embedded rather than read from disk so that the binary is
// self-contained and the script always matches the binary.
func shellIntegration() string {
	return zshIntegrationScript
}
