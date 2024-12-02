//go:build !badlinkname

package local

func getSystemDNSConfig() *dnsConfig {
	panic("stub")
}

func nameList(c *dnsConfig, name string) []string {
	panic("stub")
}

func lookupStaticHost(host string) ([]string, string) {
	panic("stub")
}

func splitHostZone(s string) (host, zone string) {
	panic("stub")
}
