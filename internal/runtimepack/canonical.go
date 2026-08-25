package runtimepack

import "bytes"

func canonicalInventory(data []byte) []byte {
	return bytes.ReplaceAll(data, []byte("\r\n"), []byte("\n"))
}
