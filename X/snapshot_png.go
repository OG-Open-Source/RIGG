//go:build snapshot

// PNG 編碼輔助（snapshot 專用；標準庫 png.Encoder 直寫 bytes.Buffer）。
package main

import (
	"bytes"
	"image"
	"image/png"
)

func pngWrite(buf *[]byte, img image.Image) {
	var b bytes.Buffer
	if err := png.Encode(&b, img); err != nil {
		return
	}
	*buf = b.Bytes()
}
