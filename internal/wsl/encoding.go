package wsl

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"strings"
	"unicode/utf16"
	"unicode/utf8"
)

func decodeOutput(data []byte) (string, error) {
	var order binary.ByteOrder = binary.LittleEndian
	isUTF16 := bytes.HasPrefix(data, []byte{0xff, 0xfe}) || bytes.HasPrefix(data, []byte{0xfe, 0xff})
	if bytes.HasPrefix(data, []byte{0xfe, 0xff}) {
		order = binary.BigEndian
	}
	if isUTF16 {
		data = data[2:]
	} else {
		isUTF16 = bytes.IndexByte(data, 0) >= 0
	}
	if isUTF16 {
		if len(data)%2 != 0 {
			return "", fmt.Errorf("invalid UTF-16 WSL output")
		}
		units := make([]uint16, len(data)/2)
		for i := range units {
			units[i] = order.Uint16(data[i*2:])
		}
		return strings.TrimSpace(string(utf16.Decode(units))), nil
	}
	data = bytes.TrimPrefix(data, []byte{0xef, 0xbb, 0xbf})
	if !utf8.Valid(data) {
		return "", fmt.Errorf("WSL output is neither UTF-8 nor UTF-16")
	}
	return strings.TrimSpace(string(data)), nil
}
