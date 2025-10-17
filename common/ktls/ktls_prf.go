// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build linux && go1.25 && badlinkname

package ktls

import "unsafe"

//go:linkname cipherSuiteByID github.com/metacubex/utls.cipherSuiteByID
func cipherSuiteByID(id uint16) unsafe.Pointer

//go:linkname keysFromMasterSecret github.com/metacubex/utls.keysFromMasterSecret
func keysFromMasterSecret(version uint16, suite unsafe.Pointer, masterSecret, clientRandom, serverRandom []byte, macLen, keyLen, ivLen int) (clientMAC, serverMAC, clientKey, serverKey, clientIV, serverIV []byte)

//go:linkname cipherSuiteTLS13ByID github.com/metacubex/utls.cipherSuiteTLS13ByID
func cipherSuiteTLS13ByID(id uint16) unsafe.Pointer

//go:linkname nextTrafficSecret github.com/metacubex/utls.(*cipherSuiteTLS13).nextTrafficSecret
func nextTrafficSecret(cs unsafe.Pointer, trafficSecret []byte) []byte

//go:linkname trafficKey github.com/metacubex/utls.(*cipherSuiteTLS13).trafficKey
func trafficKey(cs unsafe.Pointer, trafficSecret []byte) (key, iv []byte)
