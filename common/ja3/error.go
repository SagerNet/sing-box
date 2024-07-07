// Copyright (c) 2018, Open Systems AG. All rights reserved.
//
// Use of this source code is governed by a BSD-style license
// that can be found in the LICENSE file in the root of the source
// tree.

package ja3

import "fmt"

// Error types
const (
	LengthErr        string = "length check %v failed"
	ContentTypeErr   string = "content type not matching"
	VersionErr       string = "version check %v failed"
	HandshakeTypeErr string = "handshake type not matching"
	SNITypeErr       string = "SNI type not supported"
)

// ParseError can be encountered while parsing a segment
type ParseError struct {
	errType string
	check   int
}

func (e *ParseError) Error() string {
	if e.errType == LengthErr || e.errType == VersionErr {
		return fmt.Sprintf(e.errType, e.check)
	}
	return fmt.Sprint(e.errType)
}
