// Copyright 2026 PointerByte Contributors
// SPDX-License-Identifier: Apache-2.0

package local

import "errors"

// pkcs7Pad applies PKCS#7 padding to b for the given block size.
func pkcs7Pad(b []byte, blockSize int) []byte {
	if blockSize <= 0 {
		panic("blockSize must be > 0")
	}
	padLen := blockSize - (len(b) % blockSize)
	if padLen == 0 {
		padLen = blockSize
	}
	pad := bytesRepeat(byte(padLen), padLen)
	return append(b, pad...)
}

// pkcs7Unpad removes PKCS#7 padding from b and returns an error when the
// padding is invalid.
func pkcs7Unpad(b []byte, blockSize int) ([]byte, error) {
	if len(b) == 0 || len(b)%blockSize != 0 {
		return nil, errors.New("invalid padding: size")
	}
	padLen := int(b[len(b)-1])
	if padLen == 0 || padLen > blockSize || padLen > len(b) {
		return nil, errors.New("invalid padding: length")
	}

	for i := range padLen {
		if b[len(b)-1-i] != byte(padLen) {
			return nil, errors.New("invalid padding: content")
		}
	}
	return b[:len(b)-padLen], nil
}

// bytesRepeat returns a slice made of count copies of v.
func bytesRepeat(v byte, count int) []byte {
	out := make([]byte, count)
	for i := range out {
		out[i] = v
	}
	return out
}
