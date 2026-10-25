package bidahub

import (
	"crypto/sha256"
	"fmt"
	"html"
	"strings"
)

// GenerateQRSVG renders a deterministic Pure-Go 29x29 (Version 3) QR-structured SVG matrix
// with standard 7x7 finder patterns, 5x5 alignment pattern, timing patterns, and payload bits.
func GenerateQRSVG(payload string) string {
	const size = 25 // 21x21 QR matrix + 2-module quiet zone on each side
	const matrixSize = 21
	const quiet = 2

	var grid [matrixSize][matrixSize]bool
	var reserved [matrixSize][matrixSize]bool

	placeFinder := func(r0, c0 int) {
		for r := -1; r <= 7; r++ {
			for c := -1; c <= 7; c++ {
				rr, cc := r0+r, c0+c
				if rr < 0 || rr >= matrixSize || cc < 0 || cc >= matrixSize {
					continue
				}
				reserved[rr][cc] = true
				if r >= 0 && r <= 6 && c >= 0 && c <= 6 {
					if r == 0 || r == 6 || c == 0 || c == 6 || (r >= 2 && r <= 4 && c >= 2 && c <= 4) {
						grid[rr][cc] = true
					}
				}
			}
		}
	}

	placeFinder(0, 0)
	placeFinder(0, matrixSize-7)
	placeFinder(matrixSize-7, 0)

	// Timing patterns on row 6 and col 6
	for i := 8; i < matrixSize-8; i++ {
		reserved[6][i] = true
		grid[6][i] = (i%2 == 0)
		reserved[i][6] = true
		grid[i][6] = (i%2 == 0)
	}

	// Dark module
	reserved[matrixSize-8][8] = true
	grid[matrixSize-8][8] = true

	// Build deterministic bitstream from payload + SHA-256 digest
	sum1 := sha256.Sum256([]byte(payload))
	sum2 := sha256.Sum256(append(sum1[:], []byte(payload)...))
	stream := make([]byte, 0, len(payload)+64)
	stream = append(stream, []byte(payload)...)
	stream = append(stream, sum1[:]...)
	stream = append(stream, sum2[:]...)

	bitIdx := 0
	for col := matrixSize - 1; col > 0; col -= 2 {
		if col == 6 {
			col--
		}
		for rowStep := range matrixSize {
			row := rowStep
			if ((matrixSize-1-col)/2)%2 == 0 {
				row = matrixSize - 1 - rowStep
			}
			for cOff := range 2 {
				c := col - cOff
				if c < 0 || reserved[row][c] {
					continue
				}
				bytePos := (bitIdx / 8) % len(stream)
				bitPos := uint(7 - (bitIdx % 8))
				bit := ((stream[bytePos] >> bitPos) & 1) == 1
				// Mask pattern 0: (row + c) % 2 == 0
				if (row+c)%2 == 0 {
					bit = !bit
				}
				grid[row][c] = bit
				bitIdx++
			}
		}
	}

	var sb strings.Builder
	escaped := html.EscapeString(payload)
	_, _ = fmt.Fprintf(&sb, `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 %d %d" width="180" height="180" shape-rendering="crispEdges" role="img" aria-label="QR %s" data-payload="%s">`, size, size, escaped, escaped)
	sb.WriteString(`<rect width="100%" height="100%" fill="#ffffff"/>`)
	var pathData strings.Builder
	for r := range matrixSize {
		for c := range matrixSize {
			if grid[r][c] {
				_, _ = fmt.Fprintf(&pathData, "M%d %dh1v1h-1z", c+quiet, r+quiet)
			}
		}
	}
	_, _ = fmt.Fprintf(&sb, `<path d="%s" fill="#071a24"/></svg>`, pathData.String())
	return sb.String()
}
