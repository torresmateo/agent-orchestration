package main

import (
	"os"
	"os/exec"
	"syscall"
)

// runInteractive replaces the current process with an interactive command.
func runInteractive(binary string, args ...string) error {
	cmd := exec.Command(binary, args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// execReplace replaces the current process entirely (unix exec).
func execReplace(binary string, args ...string) error {
	argv := append([]string{binary}, args...)
	return syscall.Exec(binary, argv, os.Environ())
}
