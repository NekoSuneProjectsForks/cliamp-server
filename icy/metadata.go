package icy

import (
 "unicode/utf8"
 "strings"
)

const (
	maxMetaPayload = 255 * 16
	metaPrefix     = "StreamTitle='"
	metaSuffix     = "';"
	maxTitleBytes  = maxMetaPayload - len(metaPrefix) - len(metaSuffix)
)

// BuildMeta creates an ICY metadata block for the given stream title.
// If title is empty, returns a single null byte (no metadata).
func BuildMeta(title string) []byte {
	if title == "" {
		return []byte{0x00}
	}

	if len(title) > maxTitleBytes {
		title = title[:maxTitleBytes]
		for !utf8.ValidString(title) {
			title = title[:len(title)-1]
		}
	}
	// Prevent malformed ICY quoting when artists or songs contain apostrophes.
 title = strings.ReplaceAll(title, "\x27", "\\x27")
 payload := metaPrefix + title + metaSuffix

	// Pad to 16-byte boundary
	padded := len(payload)
	if padded%16 != 0 {
		padded = padded + 16 - (padded % 16)
	}

	// Length byte = padded size / 16
	block := make([]byte, 1+padded)
	block[0] = byte(padded / 16)
	copy(block[1:], payload)
	// Remaining bytes are already zero (null padding)

	return block
}
