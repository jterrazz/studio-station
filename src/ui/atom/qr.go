package atom

import (
	"strings"

	"rsc.io/qr"

	"github.com/jterrazz/studio-station/src/ui/token"
)

func QR(url string) []string {
	code, err := qr.Encode(url, qr.L)
	if err != nil {
		return nil
	}
	n := code.Size
	dark := func(x, y int) bool { return x >= 0 && y >= 0 && x < n && y < n && code.Black(x, y) }
	var lines []string
	for y := -1; y <= n; y += 2 {
		var b strings.Builder
		for x := -1; x <= n; x++ {
			top, bottom := dark(x, y), dark(x, y+1)
			switch {
			case top && bottom:
				b.WriteString(token.QRFull)
			case top:
				b.WriteString(token.QRUpper)
			case bottom:
				b.WriteString(token.QRLower)
			default:
				b.WriteByte(' ')
			}
		}
		lines = append(lines, b.String())
	}
	return lines
}
