package main

import (
	"os/exec"
	"os/user"
	"strings"

	E "github.com/sagernet/sing/common/exceptions"

	"github.com/spf13/cobra"
)

var commandAPITailscaleSSH = &cobra.Command{
	Use:   "ssh [user@]<peer>",
	Short: "SSH into a Tailscale peer",
	Long: "SSH into a Tailscale peer.\n\n" +
		"The local ssh binary is executed against the Tailscale address of the peer, so the machine running " +
		"this command must be able to route Tailscale addresses into the Tailscale endpoint itself, " +
		"usually by running behind a sing-box instance with a tun inbound.\n\n" +
		"The user defaults to the current user, and Tailscale SSH authentication is not available: " +
		"the peer is reached as an ordinary SSH server.",
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return runAPITailscaleSSH(args[0])
	},
}

func init() {
	commandAPITailscaleSSH.Flags().StringVar(&commandAPITailscaleFlagEndpoint, "endpoint", "", commandAPITailscaleEndpointUsage)
	commandAPITailscale.AddCommand(commandAPITailscaleSSH)
}

func runAPITailscaleSSH(target string) error {
	loginName := ""
	selector := target
	nameIndex := strings.LastIndex(target, "@")
	if nameIndex != -1 {
		loginName = target[:nameIndex]
		selector = target[nameIndex+1:]
	}
	if loginName == "" {
		currentUser, userErr := user.Current()
		if userErr == nil {
			loginName = currentUser.Username
			_, domainUserName, isDomainUser := strings.Cut(loginName, "\\")
			if isDomainUser {
				loginName = domainUserName
			}
		}
	}
	clientConn, client, err := createAPIClient()
	if err != nil {
		return err
	}
	defer clientConn.Close()
	endpoint, err := fetchTailscaleEndpoint(client)
	if err != nil {
		return err
	}
	entry, err := resolveTailscalePeer(tailscalePeerEntries(endpoint), selector)
	if err != nil {
		return err
	}
	peerAddress := tailscalePeerAddress(entry.peer)
	if peerAddress == "" {
		return E.New("peer has no tailscale address: ", tailscalePeerName(entry.peer))
	}
	clientConn.Close()
	sshPath, err := exec.LookPath("ssh")
	if err != nil {
		return E.New("ssh not found in PATH")
	}
	destination := peerAddress
	if loginName != "" {
		destination = loginName + "@" + peerAddress
	}
	return executeSSH(sshPath, []string{"ssh", destination})
}
