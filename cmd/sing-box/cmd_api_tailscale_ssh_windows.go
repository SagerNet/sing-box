package main

import (
	"errors"
	"os"
	"os/exec"
)

func executeSSH(path string, argv []string) error {
	command := exec.Command(path, argv[1:]...)
	command.Stdin = os.Stdin
	command.Stdout = os.Stdout
	command.Stderr = os.Stderr
	err := command.Run()
	var exitError *exec.ExitError
	if errors.As(err, &exitError) {
		os.Exit(exitError.ExitCode())
	}
	return err
}
