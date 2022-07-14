package settings

import (
	"os"
	"os/exec"
	"strings"

	"github.com/sagernet/sing-box/log"
)

func runCommand(name string, args ...string) error {
	log.Debug(name, " ", strings.Join(args, " "))
	command := exec.Command(name, args...)
	command.Env = os.Environ()
	command.Stdin = os.Stdin
	command.Stdout = os.Stderr
	command.Stderr = os.Stderr
	return command.Run()
}
