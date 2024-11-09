package main

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"

	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing/common/bufio"
	E "github.com/sagernet/sing/common/exceptions"
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

var (
	httpClient  *http.Client
	http3Client *http.Client
)

func fetch(args []string) error {
	instance, err := createPreStartedClient()
	if err != nil {
		return err
	}
	defer instance.Close()
	httpClient = &http.Client{
		Transport: &http.Transport{
			DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
				dialer, err := createDialer(instance, commandToolsFlagOutbound)
				if err != nil {
					return nil, err
				}
				return dialer.DialContext(ctx, network, M.ParseSocksaddr(addr))
			},
			ForceAttemptHTTP2: true,
		},
	}
	defer httpClient.CloseIdleConnections()
	if C.WithQUIC {
		err = initializeHTTP3Client(instance)
		if err != nil {
			return err
		}
		defer http3Client.CloseIdleConnections()
	}
	for _, urlString := range args {
		var parsedURL *url.URL
		parsedURL, err = url.Parse(urlString)
		if err != nil {
			return err
		}
		switch parsedURL.Scheme {
		case "":
			parsedURL.Scheme = "http"
			fallthrough
		case "http", "https":
			err = fetchHTTP(httpClient, parsedURL)
			if err != nil {
				return err
			}
		case "http3":
			if !C.WithQUIC {
				return C.ErrQUICNotIncluded
			}
			parsedURL.Scheme = "https"
			err = fetchHTTP(http3Client, parsedURL)
			if err != nil {
				return err
			}
		default:
			return E.New("unsupported scheme: ", parsedURL.Scheme)
		}
	}
	return nil
}

func fetchHTTP(httpClient *http.Client, parsedURL *url.URL) error {
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
