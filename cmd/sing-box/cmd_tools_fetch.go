package main

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"

	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing/common/bufio"
	M "github.com/sagernet/sing/common/metadata"

	"github.com/spf13/cobra"
)

var commandFetch = &cobra.Command{
	Use:   "fetch",
	Short: "Fetch an URL",
	Args:  cobra.MinimumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		err := fetch(args)
		if err != nil {
			log.Fatal(err)
		}
	},
}

func init() {
	commandTools.AddCommand(commandFetch)
}

var httpClient *http.Client

func fetch(args []string) error {
	instance, err := createPreStartedClient()
	if err != nil {
		return err
	}
	defer instance.Close()
	httpClient = &http.Client{
		Transport: &http.Transport{
			DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
				dialer, err := createDialer(instance, network, commandToolsFlagOutbound)
				if err != nil {
					return nil, err
				}
				return dialer.DialContext(ctx, network, M.ParseSocksaddr(addr))
			},
			ForceAttemptHTTP2: true,
		},
	}
	defer httpClient.CloseIdleConnections()
	for _, urlString := range args {
		parsedURL, err := url.Parse(urlString)
		if err != nil {
			return err
		}
		switch parsedURL.Scheme {
		case "":
			parsedURL.Scheme = "http"
			fallthrough
		case "http", "https":
			err = fetchHTTP(parsedURL)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func fetchHTTP(parsedURL *url.URL) error {
	request, err := http.NewRequest("GET", parsedURL.String(), nil)
	if err != nil {
		return err
	}
	request.Header.Add("User-Agent", "curl/7.88.0")
	response, err := httpClient.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	_, err = bufio.Copy(os.Stdout, response.Body)
	if errors.Is(err, io.EOF) {
		return nil
	}
	return err
}
