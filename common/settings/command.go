package settings

import (
	"os"
	"os/exec"
)

func runCommand(name string, args ...string) error {
	command := exec.Command(name, args...)
	command.Env = os.Environ()
	command.Stdin = os.Stdin
	command.Stdout = os.Stderr
	command.Stderr = os.Stderr
	return command.Run()
}

func readCommand(name string, args ...string) ([]byte, error) {
	command := exec.Command(name, args...)
	command.Env = os.Environ()
	return command.CombinedOutput()
}
