// Package challenge encodes and reproduces shareable offline puzzles.
//
// A code is an honor-system transport: compact, checksummed, and deterministic,
// but neither secret nor authenticated. It has no persistence or UI knowledge.
package challenge

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"fmt"
	"io"
	"strings"

	"github.com/nxck2005/surmise/internal/game"
	"github.com/nxck2005/surmise/internal/words"
)

const (
	formatVersion = 1
	flagsV1       = 0
	encodedLen    = 16
	payloadBits   = 70
	checksumBits  = 10
	idTag         = "challenge-id-v1\x00"
)

const alphabet = "0123456789ABCDEFGHJKMNPQRSTVWXYZ"

// Code is one parsed challenge. Its fields stay private so every value comes
// through Generate or Parse and therefore has a valid checksum and version.
type Code struct {
	listVersion int
	length      int
	flags       uint8
	seed        uint64 // low 56 bits
}

// Generate makes a fresh challenge for a supported word length.
func Generate(length int) (Code, error) {
	return generate(length, rand.Reader)
}

func generate(length int, r io.Reader) (Code, error) {
	if !words.SupportedLength(length) {
		return Code{}, fmt.Errorf("challenge: unsupported length %d", length)
	}
	var b [7]byte
	if _, err := io.ReadFull(r, b[:]); err != nil {
		return Code{}, fmt.Errorf("challenge: generate seed: %w", err)
	}
	var seed uint64
	for _, v := range b {
		seed = seed<<8 | uint64(v)
	}
	return Code{
		listVersion: words.CurrentAnswerVersion,
		length:      length,
		flags:       flagsV1,
		seed:        seed,
	}, nil
}

// Parse validates and canonicalizes a challenge code.
func Parse(text string) (Code, error) {
	normalized, err := normalize(text)
	if err != nil {
		return Code{}, err
	}

	var packed [10]byte
	for i, r := range normalized {
		value := strings.IndexRune(alphabet, r)
		if value < 0 {
			return Code{}, fmt.Errorf("challenge: invalid character %q", r)
		}
		putBits(packed[:], i*5, 5, uint64(value))
	}

	want := checksum(packed)
	got := uint16(getBits(packed[:], payloadBits, checksumBits))
	if subtle.ConstantTimeEq(int32(got), int32(want)) != 1 {
		return Code{}, fmt.Errorf("challenge: checksum mismatch")
	}

	version := int(getBits(packed[:], 0, 3))
	if version != formatVersion {
		return Code{}, fmt.Errorf("challenge: unsupported format version %d", version)
	}
	listVersion := int(getBits(packed[:], 3, 5))
	selector := int(getBits(packed[:], 8, 2))
	length, ok := lengthFor(selector)
	if !ok {
		return Code{}, fmt.Errorf("challenge: unsupported length selector %d", selector)
	}
	flags := uint8(getBits(packed[:], 10, 4))
	if flags != flagsV1 {
		return Code{}, fmt.Errorf("challenge: unsupported mode flags %d", flags)
	}
	if _, err := words.AnswerCountAt(listVersion, length); err != nil {
		return Code{}, fmt.Errorf("challenge: unsupported answer version %d", listVersion)
	}

	return Code{
		listVersion: listVersion,
		length:      length,
		flags:       flags,
		seed:        getBits(packed[:], 14, 56),
	}, nil
}

func normalize(text string) (string, error) {
	if len(text) > 64 {
		return "", fmt.Errorf("challenge: code is too long")
	}
	var b strings.Builder
	b.Grow(encodedLen)
	for _, r := range text {
		switch {
		case r == '-' || r == ' ':
			continue
		case r >= 'a' && r <= 'z':
			r -= 'a' - 'A'
		case r < '0' || r > 'Z':
			return "", fmt.Errorf("challenge: invalid character %q", r)
		}
		if !strings.ContainsRune(alphabet, r) {
			return "", fmt.Errorf("challenge: invalid character %q", r)
		}
		b.WriteRune(r)
	}
	if b.Len() != encodedLen {
		return "", fmt.Errorf("challenge: needs %d characters", encodedLen)
	}
	return b.String(), nil
}

// String returns the canonical grouped representation.
func (c Code) String() string {
	packed := c.pack()
	var raw [encodedLen]byte
	for i := range raw {
		raw[i] = alphabet[getBits(packed[:], i*5, 5)]
	}
	return string(raw[:4]) + "-" + string(raw[4:8]) + "-" +
		string(raw[8:12]) + "-" + string(raw[12:])
}

// Length reports the board size carried by the code.
func (c Code) Length() int { return c.length }

// Answer returns the answer selected by the code's versioned seed.
func (c Code) Answer() (string, error) {
	count, err := words.AnswerCountAt(c.listVersion, c.length)
	if err != nil {
		return "", err
	}
	index := int(c.seed % uint64(count))
	return words.AnswerAtVersion(c.listVersion, c.length, index)
}

// ID returns the stable UUIDv8 used as the puzzle's persistence key.
func (c Code) ID() string {
	payload := c.payload()
	h := sha256.New()
	_, _ = h.Write([]byte(idTag))
	_, _ = h.Write(payload[:])
	sum := h.Sum(nil)
	var id [16]byte
	copy(id[:], sum)
	return game.FormatID(id, 8)
}

// NewGame builds the ordinary game represented by the code.
func (c Code) NewGame() (*game.Game, error) {
	answer, err := c.Answer()
	if err != nil {
		return nil, err
	}
	g, err := game.NewFrom(c.ID(), answer, c.length)
	if err != nil {
		return nil, err
	}
	g.Challenge = &game.ChallengeInfo{Code: c.String()}
	return g, nil
}

// ValidateGame checks that persisted challenge metadata still describes the
// game carrying it. Stores call this at their shared codec boundary so a
// hand-edited or imported record cannot resume one board and later share a code
// for another. Tombstones intentionally have no code and are validated by the
// ordinary game rules instead.
func ValidateGame(g *game.Game) error {
	if g.Challenge == nil || g.Deleted {
		return nil
	}
	c, err := Parse(g.Challenge.Code)
	if err != nil {
		return fmt.Errorf("challenge: invalid saved code: %w", err)
	}
	answer, err := c.Answer()
	if err != nil {
		return err
	}
	switch {
	case g.Challenge.Code != c.String():
		return fmt.Errorf("challenge: saved code is not canonical")
	case g.ID != c.ID():
		return fmt.Errorf("challenge: saved code does not match puzzle id")
	case g.Length != c.Length():
		return fmt.Errorf("challenge: saved code does not match puzzle length")
	case g.Answer != answer:
		return fmt.Errorf("challenge: saved code does not match puzzle answer")
	}
	return nil
}

func (c Code) pack() [10]byte {
	packed := c.payload()
	putBits(packed[:], payloadBits, checksumBits, uint64(checksum(packed)))
	return packed
}

func (c Code) payload() [10]byte {
	var packed [10]byte
	putBits(packed[:], 0, 3, formatVersion)
	putBits(packed[:], 3, 5, uint64(c.listVersion))
	selector, _ := selectorFor(c.length)
	putBits(packed[:], 8, 2, uint64(selector))
	putBits(packed[:], 10, 4, uint64(c.flags))
	putBits(packed[:], 14, 56, c.seed)
	return packed
}

func checksum(packed [10]byte) uint16 {
	putBits(packed[:], payloadBits, checksumBits, 0)
	sum := sha256.Sum256(packed[:9])
	return uint16(sum[0])<<2 | uint16(sum[1])>>6
}

func selectorFor(length int) (int, bool) {
	switch length {
	case 4:
		return 0, true
	case 5:
		return 1, true
	case 6:
		return 2, true
	default:
		return 0, false
	}
}

func lengthFor(selector int) (int, bool) {
	switch selector {
	case 0:
		return 4, true
	case 1:
		return 5, true
	case 2:
		return 6, true
	default:
		return 0, false
	}
}

func putBits(dst []byte, offset, width int, value uint64) {
	for i := range width {
		bit := byte(value >> (width - i - 1) & 1)
		at := offset + i
		mask := byte(1 << (7 - at%8))
		if bit == 1 {
			dst[at/8] |= mask
		} else {
			dst[at/8] &^= mask
		}
	}
}

func getBits(src []byte, offset, width int) uint64 {
	var value uint64
	for i := range width {
		at := offset + i
		value = value<<1 | uint64(src[at/8]>>(7-at%8)&1)
	}
	return value
}
