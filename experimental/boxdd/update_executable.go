//go:build windows

package main

import (
	"debug/pe"
	"encoding/binary"
	"os"

	E "github.com/sagernet/sing/common/exceptions"
)

const (
	peSecurityDirectoryIndex = 4
	nsisFirstHeaderSize      = 28
	nsisSignature            = 0xDEADBEEF
	nsisIdentity             = "NullsoftInst"
	nsisMaximumAlignmentTail = 8
)

func validateNSISExecutable(path string) error {
	executable, err := os.Open(path)
	if err != nil {
		return err
	}
	defer executable.Close()
	information, err := executable.Stat()
	if err != nil {
		return err
	}
	peFile, err := pe.NewFile(executable)
	if err != nil {
		return err
	}
	defer peFile.Close()
	var overlayOffset uint64
	for _, section := range peFile.Sections {
		sectionEnd := uint64(section.Offset) + uint64(section.Size)
		if sectionEnd > overlayOffset {
			overlayOffset = sectionEnd
		}
	}
	contentEnd := uint64(information.Size())
	var securityOffset uint64
	switch optionalHeader := peFile.OptionalHeader.(type) {
	case *pe.OptionalHeader32:
		securityOffset = uint64(optionalHeader.DataDirectory[peSecurityDirectoryIndex].VirtualAddress)
	case *pe.OptionalHeader64:
		securityOffset = uint64(optionalHeader.DataDirectory[peSecurityDirectoryIndex].VirtualAddress)
	}
	if securityOffset != 0 {
		if securityOffset > contentEnd {
			return E.New("invalid Authenticode directory offset")
		}
		contentEnd = securityOffset
	}
	if overlayOffset+nsisFirstHeaderSize > contentEnd {
		return E.New("missing NSIS first header")
	}
	firstHeader := make([]byte, nsisFirstHeaderSize)
	_, err = executable.ReadAt(firstHeader, int64(overlayOffset))
	if err != nil {
		return err
	}
	if binary.LittleEndian.Uint32(firstHeader[4:8]) != nsisSignature || string(firstHeader[8:20]) != nsisIdentity {
		return E.New("invalid NSIS first header signature")
	}
	headerSize := uint64(binary.LittleEndian.Uint32(firstHeader[20:24]))
	archiveSize := uint64(binary.LittleEndian.Uint32(firstHeader[24:28]))
	if headerSize < nsisFirstHeaderSize || archiveSize < headerSize || overlayOffset+archiveSize > contentEnd {
		return E.New("invalid NSIS archive size")
	}
	if contentEnd-(overlayOffset+archiveSize) > nsisMaximumAlignmentTail {
		return E.New("unexpected data after NSIS archive")
	}
	return nil
}
