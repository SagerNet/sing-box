//go:build badlinkname

package local

import (
	_ "unsafe"
)

//go:linkname getSystemDNSConfig net.getSystemDNSConfig
func getSystemDNSConfig() *dnsConfig

//go:linkname nameList net.(*dnsConfig).nameList
func nameList(c *dnsConfig, name string) []string

//go:linkname lookupStaticHost net.lookupStaticHost
func lookupStaticHost(host string) ([]string, string)

//go:linkname splitHostZone net.splitHostZone
func splitHostZone(s string) (host, zone string)
