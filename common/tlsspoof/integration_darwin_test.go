//go:build darwin

package tlsspoof

const loopbackInterface = "lo0"

// forceMSS rewires a freshly constructed spoofer's chunk-size budget so a
// standard-sized ClientHello is guaranteed to split, even on loopback where
// TCP_MAXSEG returns a large value. Returns false if the spoofer type does
// not carry an mss field on this platform.
func forceMSS(spoofer Spoofer, mss int) bool {
	ds, ok := spoofer.(*darwinSpoofer)
	if !ok {
		return false
	}
	ds.mss = mss
	return true
}

// spooferSendNext exposes the captured snd_una reference point so wire-order
// tests can convert tcpdump absolute-seq output into the fake/real slot the
// builder assigned to each chunk.
func spooferSendNext(spoofer Spoofer) (uint32, bool) {
	ds, ok := spoofer.(*darwinSpoofer)
	if !ok {
		return 0, false
	}
	return ds.sendNext, true
}
