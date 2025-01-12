package sid

import (
	"bytes"
	"crypto/rand"
	"encoding/base32"
	"encoding/json"
	"fmt"
	"strings"
)

// Bytes in an ID
const Length = 8

type ID []byte

// Generates new, random ID
func New() ID {

	buf := make([]byte, Length)

	if _, err := rand.Read(buf); err != nil {
		panic(err)
	}

	return ID(buf)

}

// Parses an ID from a string
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

// Encode an ID as a 13 character string (base32 alphabet)
func (id ID) String() string {

	if len(id) != Length {
		panic(fmt.Sprintf("unknown SID length encounted: %d", len(id)))
	}

	return strings.ToLower(base32.StdEncoding.EncodeToString(id)[:13])

}

// Equality check
func (id ID) Equals(other ID) bool {
	return bytes.Equal([]byte(id), []byte(other))
}

func (id ID) MarshalJSON() ([]byte, error) {
	if id == nil {
		return []byte(`""`), nil
	}

	return json.Marshal(id.String())
}

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

// Encode multiple IDs in a single string
func EncodeMultiple(us []ID) string {
	s := ""
	for _, u := range us {
		s += u.String()
	}
	return s
}

// Decode multiple IDs from a single string
func DecodeMultiple(s string) []ID {

	us := []ID{}

	for i := 0; i < len(s)/13; i++ {
		us = append(us, Parse(s[i*13:(i+1)*13]))
	}

	return us

}
