//go:build windows

package main

import "encoding/binary"

// trayIcon is a 16×16 ICO (BGRA, brand graphite + amber) so the helper
// does not ship a binary asset.
func trayIcon() []byte {
	const n = 16
	xor := make([]byte, n*n*4)
	for y := 0; y < n; y++ {
		for x := 0; x < n; x++ {
			i := ((n-1-y)*n + x) * 4
			dx := float64(x) - 7.5
			dy := float64(y) - 7.5
			if dx*dx+dy*dy <= 36 {
				xor[i+0] = 0x23
				xor[i+1] = 0xa6
				xor[i+2] = 0xf5
				xor[i+3] = 0xff
				continue
			}
			xor[i+0] = 0x0e
			xor[i+1] = 0x0c
			xor[i+2] = 0x0b
			xor[i+3] = 0xff
		}
	}
	andMask := make([]byte, n*4)
	imageSize := 40 + len(xor) + len(andMask)
	buf := make([]byte, 22+imageSize)
	binary.LittleEndian.PutUint16(buf[2:], 1)
	binary.LittleEndian.PutUint16(buf[4:], 1)
	buf[6] = n
	buf[7] = n
	binary.LittleEndian.PutUint16(buf[10:], 1)
	binary.LittleEndian.PutUint16(buf[12:], 32)
	binary.LittleEndian.PutUint32(buf[14:], uint32(imageSize))
	binary.LittleEndian.PutUint32(buf[18:], 22)
	off := 22
	binary.LittleEndian.PutUint32(buf[off:], 40)
	binary.LittleEndian.PutUint32(buf[off+4:], uint32(n))
	binary.LittleEndian.PutUint32(buf[off+8:], uint32(n*2))
	binary.LittleEndian.PutUint16(buf[off+12:], 1)
	binary.LittleEndian.PutUint16(buf[off+14:], 32)
	binary.LittleEndian.PutUint32(buf[off+20:], uint32(len(xor)+len(andMask)))
	copy(buf[off+40:], xor)
	copy(buf[off+40+len(xor):], andMask)
	return buf
}
