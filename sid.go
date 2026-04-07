// Package sid generates short, URL-safe identifiers backed by 64 bits of
// cryptographic randomness. Each ID is encoded as a 13-character lowercase
// base32 string, making it compact and safe to embed in URLs, file names, and
// database keys without escaping.
//
// Example IDs:
//
//	ab3nqykgzx4rw
//	mf7vt2hjcpx6q
//	yz9rk4bdwn2xs
//
// Basic usage:
//
//	id := sid.New()
//	fmt.Println(id)          // e.g. "ab3nqykgzx4rw"
//	parsed := sid.Parse(id.String())
package sid

import (
	"bytes"
	"crypto/rand"
	"encoding/base32"
	"encoding/json"
	"fmt"
	"strings"
)

// Length is the number of random bytes in each ID (64 bits).
const Length = 8

// ID is a short, randomly generated identifier.
// Its zero value (nil slice) is valid and encodes as an empty JSON string.
type ID []byte

// New generates a new random ID using crypto/rand.
// It panics if the system's random source is unavailable.
func New() ID {

	buf := make([]byte, Length)

	if _, err := rand.Read(buf); err != nil {
		panic(err)
	}

	return ID(buf)

}

// Parse decodes an ID from its 13-character string representation.
// It panics if s is not a valid 13-character base32-encoded ID.
func Parse(s string) ID {

	if len(s) != 13 {
		panic(fmt.Sprintf("unknown SID length encounted: %d", len(s)))
	}

	b, err := base32.StdEncoding.DecodeString(strings.ToUpper(s) + "===")

	if err != nil {
		panic(fmt.Sprintf("couldn't parse SID: %v", err))
	}

	if len(b) != Length {
		panic(fmt.Sprintf("unknown SID length encounted: %d", len(b)))
	}

	return ID(b)

}

// String returns the ID as a 13-character lowercase base32 string.
// It panics if the ID does not contain exactly [Length] bytes.
func (id ID) String() string {

	if len(id) != Length {
		panic(fmt.Sprintf("unknown SID length encounted: %d", len(id)))
	}

	return strings.ToLower(base32.StdEncoding.EncodeToString(id)[:13])

}

// Equals reports whether id and other represent the same identifier.
func (id ID) Equals(other ID) bool {
	return bytes.Equal([]byte(id), []byte(other))
}

// MarshalJSON implements [encoding/json.Marshaler].
// A nil ID marshals as an empty JSON string ("").
func (id ID) MarshalJSON() ([]byte, error) {
	if id == nil {
		return []byte(`""`), nil
	}

	return json.Marshal(id.String())
}

// UnmarshalJSON implements [encoding/json.Unmarshaler].
// An empty JSON string ("") leaves the ID unchanged.
func (id *ID) UnmarshalJSON(data []byte) error {

	if string(data) == `""` {
		return nil
	}

	var s string

	if err := json.Unmarshal(data, &s); err != nil {
		return fmt.Errorf("invalid SID string: %v", err)
	}

	*id = Parse(s)

	return nil
}

// EncodeMultiple concatenates the string representations of all IDs in us
// into a single string. Use [DecodeMultiple] to reverse the operation.
func EncodeMultiple(us []ID) string {
	var b strings.Builder
	for _, u := range us {
		b.WriteString(u.String())
	}
	return b.String()
}

// DecodeMultiple splits a concatenated string produced by [EncodeMultiple]
// back into individual IDs. It panics if any 13-character segment is invalid.
func DecodeMultiple(s string) []ID {

	us := []ID{}

	for i := 0; i < len(s)/13; i++ {
		us = append(us, Parse(s[i*13:(i+1)*13]))
	}

	return us

}
