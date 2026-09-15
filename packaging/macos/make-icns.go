//go:build ignore

// Command make-icns packages the PNG sizes created by sips into a modern ICNS file.
package main

import (
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"
)

type iconEntry struct {
	kind string
	name string
}

func main() {
	if len(os.Args) != 3 {
		fmt.Fprintln(os.Stderr, "usage: go run make-icns.go ICONSET OUTPUT")
		os.Exit(2)
	}
	entries := []iconEntry{
		{"icp4", "icon_16x16.png"},
		{"icp5", "icon_32x32.png"},
		{"icp6", "icon_32x32@2x.png"},
		{"ic07", "icon_128x128.png"},
		{"ic08", "icon_256x256.png"},
		{"ic09", "icon_512x512.png"},
		{"ic10", "icon_512x512@2x.png"},
	}

	chunks := make([][]byte, 0, len(entries))
	totalSize := uint32(8)
	for _, entry := range entries {
		png, err := os.ReadFile(filepath.Join(os.Args[1], entry.name))
		if err != nil {
			fail(err)
		}
		chunk := make([]byte, 8+len(png))
		copy(chunk[:4], entry.kind)
		binary.BigEndian.PutUint32(chunk[4:8], uint32(len(chunk)))
		copy(chunk[8:], png)
		chunks = append(chunks, chunk)
		totalSize += uint32(len(chunk))
	}

	output := make([]byte, 8, totalSize)
	copy(output[:4], "icns")
	binary.BigEndian.PutUint32(output[4:8], totalSize)
	for _, chunk := range chunks {
		output = append(output, chunk...)
	}
	if err := os.WriteFile(os.Args[2], output, 0o644); err != nil {
		fail(err)
	}
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
