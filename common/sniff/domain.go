package sniff

import _ "unsafe" // for linkname

//go:linkname IsDomainName net.isDomainName
func IsDomainName(domain string) bool
