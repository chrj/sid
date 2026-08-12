// Package sid generates short, URL-safe identifiers backed by cryptographic
// randomness. An ID holds 40, 80, 120, or 160 bits and encodes as a lowercase
// base32 string of 8, 16, 24, or 32 characters. IDs are safe to put in URLs,
// file names, and database keys without escaping.
//
// Example IDs at each size:
//
//	ab3nqykg                          (40-bit)
//	mf7vt2hjcpx6qw4d                  (80-bit)
//	drn7oow74uhzhjjwl5xasnp6          (120-bit)
//	6hhehlttai7vkdg25p6buouh5mju2zbz  (160-bit)
//
// Basic usage:
//
//	id := sid.New80()
//	fmt.Println(id) // e.g. "mf7vt2hjcpx6qw4d"
//
//	parsed, err := sid.Parse("mf7vt2hjcpx6qw4d")
//	if err != nil {
//		return err
//	}
//	fmt.Println(id == parsed) // true
//
// [New40], [New80], [New120], and [New160] cannot fail. Use [New] when the
// size comes from configuration, and [ParseSize] to read the size from text.
//
// # Sizes
//
// Every size is a multiple of 5 bytes, so it maps to a whole number of base32
// characters. This makes the encoding a bijection: each ID has exactly one
// string form, and each string of a supported length decodes to exactly one
// ID. Sizes that are not multiples of 5 bytes leave spare bits in the last
// character, which lets more than one string decode to the same ID.
//
// # Collisions
//
// IDs are random, so two of them can collide. The chance stays near zero until
// the count approaches the square root of the value space. For 40-bit IDs that
// is about 1.2 million, and for 80-bit IDs about 1.3e12. Use [Size40] only
// where a collision is cheap to detect, such as a short code behind a unique
// index. Use [Size80] or larger for primary keys.
package sid

import (
	"bytes"
	"cmp"
	"crypto/rand"
	"database/sql/driver"
	"encoding/base32"
	"fmt"
	"strconv"
	"strings"
)

// maxBytes is the largest supported payload. ID holds its bytes in an array of
// this length so that ID stays comparable with == and usable as a map key.
const maxBytes = 20

// base32Enc is RFC 4648 base32 with a lowercase alphabet and no padding. All
// supported sizes are multiples of 5 bytes, which is the base32 block size, so
// no size ever needs padding characters.
var base32Enc = base32.NewEncoding("abcdefghijklmnopqrstuvwxyz234567").WithPadding(base32.NoPadding)

// maxErrorInput is the longest input echoed back in a [ParseError]. Inputs can
// come from a remote caller, so the message keeps a bound on its own length.
const maxErrorInput = 64

// Size is the amount of randomness in an ID, counted in bytes. The constants
// are named for the bit count, which is what callers usually reason about.
type Size uint8

// Supported sizes. Each one is a multiple of 5 bytes.
const (
	Size40  Size = 5  // 40 bits, 8 characters
	Size80  Size = 10 // 80 bits, 16 characters
	Size120 Size = 15 // 120 bits, 24 characters
	Size160 Size = 20 // 160 bits, 32 characters
)

// Valid reports whether s is a supported size.
func (s Size) Valid() bool {
	switch s {
	case Size40, Size80, Size120, Size160:
		return true
	default:
		return false
	}
}

// Bits returns the number of random bits in an ID of this size.
func (s Size) Bits() int {
	return int(s) * 8
}

// EncodedLen returns the length in characters of an ID of this size.
// It returns 0 if s is not a supported size.
func (s Size) EncodedLen() int {
	if !s.Valid() {
		return 0
	}

	return base32Enc.EncodedLen(int(s))
}

// String returns a short description of the size, such as "80-bit".
func (s Size) String() string {
	if !s.Valid() {
		return fmt.Sprintf("invalid size (%d bytes)", uint8(s))
	}

	return fmt.Sprintf("%d-bit", s.Bits())
}

// ParseSize reads a size from text such as "80" or "80-bit". Use it for a size
// that comes from configuration or from a command-line flag.
func ParseSize(s string) (Size, error) {
	bits, err := strconv.Atoi(strings.TrimSuffix(s, "-bit"))

	if err == nil && bits%8 == 0 {
		if size := Size(bits / 8); size.Valid() {
			return size, nil
		}
	}

	return 0, fmt.Errorf("sid: cannot parse size %q: use 40, 80, 120, or 160", s)
}

// MarshalText implements [encoding.TextMarshaler], so that a Size can be a
// field in a JSON or YAML document.
func (s Size) MarshalText() ([]byte, error) {
	if !s.Valid() {
		return nil, &SizeError{Bytes: int(s)}
	}

	return []byte(s.String()), nil
}

// UnmarshalText implements [encoding.TextUnmarshaler].
func (s *Size) UnmarshalText(text []byte) error {
	size, err := ParseSize(string(text))

	if err != nil {
		return err
	}

	*s = size

	return nil
}

// sizeForEncodedLen returns the size that encodes to n characters.
func sizeForEncodedLen(n int) (Size, bool) {
	switch n {
	case Size40.EncodedLen():
		return Size40, true
	case Size80.EncodedLen():
		return Size80, true
	case Size120.EncodedLen():
		return Size120, true
	case Size160.EncodedLen():
		return Size160, true
	default:
		return 0, false
	}
}

// SizeError reports a size that this package does not support. Bytes is the
// rejected length, which can come from a [Size] or from a slice of bytes.
type SizeError struct {
	Bytes int
}

func (e *SizeError) Error() string {
	return fmt.Sprintf("sid: unsupported size (%d bytes): use Size40, Size80, Size120, or Size160", e.Bytes)
}

// MixedSizeError reports IDs of more than one size passed to [EncodeMultiple].
type MixedSizeError struct {
	First Size
	Found Size
}

func (e *MixedSizeError) Error() string {
	return fmt.Sprintf("sid: mixed sizes: first ID is %s but found %s: encode each size separately", e.First, e.Found)
}

// ParseError reports a string that is not a valid ID. Reason says which rule
// the input broke.
type ParseError struct {
	Input  string
	Reason string
}

func (e *ParseError) Error() string {
	input := e.Input

	if len(input) > maxErrorInput {
		input = input[:maxErrorInput] + "..."
	}

	return fmt.Sprintf("sid: cannot parse %q: %s", input, e.Reason)
}

// ID is a short, randomly generated identifier. It is comparable with == and
// can be used as a map key.
//
// The zero value is the empty ID. It carries no randomness, encodes as the
// empty string, and marshals as an empty JSON string. Use [ID.IsZero] to test
// for it.
type ID struct {
	// b holds n bytes of randomness. Bytes past n are always zero, so == and
	// map lookups compare only meaningful state.
	b [maxBytes]byte
	// n is 0 for the empty ID, otherwise a supported Size. Both fields are
	// unexported, so no caller outside this package can break that invariant.
	n Size
}

// New40 generates a random 40-bit ID. It cannot fail.
func New40() ID { return newID(Size40) }

// New80 generates a random 80-bit ID. It cannot fail.
func New80() ID { return newID(Size80) }

// New120 generates a random 120-bit ID. It cannot fail.
func New120() ID { return newID(Size120) }

// New160 generates a random 160-bit ID. It cannot fail.
func New160() ID { return newID(Size160) }

// New generates a random ID of the given size. Use it when the size comes from
// configuration. For a size that is fixed in code, use [New40], [New80],
// [New120], or [New160], which return no error.
//
// It returns a [SizeError] if size is not supported.
func New(size Size) (ID, error) {
	if !size.Valid() {
		return ID{}, &SizeError{Bytes: int(size)}
	}

	return newID(size), nil
}

// MustNew is like [New] but panics if size is not supported.
func MustNew(size Size) ID {
	id, err := New(size)

	if err != nil {
		panic(err)
	}

	return id
}

// newID fills an ID with randomness. The size must already be valid.
func newID(size Size) ID {
	var id ID

	// crypto/rand.Read never returns an error. It crashes the program if the
	// system source of randomness fails, so there is nothing to report here.
	_, _ = rand.Read(id.b[:size])

	id.n = size

	return id
}

// FromBytes builds an ID from raw bytes, which is the reverse of [ID.Bytes].
// Use it to read an ID back from storage that holds the bytes and not the
// string. The length of b must be 5, 10, 15, or 20.
//
// It returns a [SizeError] if b has any other length.
func FromBytes(b []byte) (ID, error) {
	size := Size(len(b))

	if len(b) > maxBytes || !size.Valid() {
		return ID{}, &SizeError{Bytes: len(b)}
	}

	var id ID

	copy(id.b[:], b)
	id.n = size

	return id, nil
}

// Parse decodes an ID from its string form. The size comes from the length of
// s, which must be 8, 16, 24, or 32 characters. The empty string decodes to
// the zero ID.
//
// Only lowercase input is accepted, so that each ID has exactly one string
// form. It returns a [ParseError] if s is not a valid ID.
func Parse(s string) (ID, error) {
	if s == "" {
		return ID{}, nil
	}

	size, ok := sizeForEncodedLen(len(s))

	if !ok {
		return ID{}, &ParseError{
			Input:  s,
			Reason: fmt.Sprintf("length %d: want 8, 16, 24, or 32 characters", len(s)),
		}
	}

	if err := checkAlphabet(s); err != nil {
		return ID{}, err
	}

	b, err := base32Enc.DecodeString(s)

	if err != nil {
		return ID{}, &ParseError{Input: s, Reason: err.Error()}
	}

	if len(b) != int(size) {
		return ID{}, &ParseError{
			Input:  s,
			Reason: fmt.Sprintf("decoded to %d bytes, want %d", len(b), int(size)),
		}
	}

	var id ID

	copy(id.b[:], b)
	id.n = size

	return id, nil
}

// checkAlphabet rejects any character outside the encoding alphabet.
//
// Parse cannot leave this to the decoder. WithPadding(NoPadding) sets the pad
// character to -1, which the decoder compares against input as the byte 0xff.
// A literal 0xff byte therefore reads as padding, and the decoder returns
// fewer bytes than the length of the input calls for, with no error.
func checkAlphabet(s string) *ParseError {
	for i := 0; i < len(s); i++ {
		c := s[i]

		if (c >= 'a' && c <= 'z') || (c >= '2' && c <= '7') {
			continue
		}

		if c >= 'A' && c <= 'Z' {
			return &ParseError{
				Input:  s,
				Reason: fmt.Sprintf("uppercase %q at offset %d: IDs are lowercase", c, i),
			}
		}

		return &ParseError{
			Input:  s,
			Reason: fmt.Sprintf("character %q at offset %d is outside the alphabet a-z and 2-7", c, i),
		}
	}

	return nil
}

// MustParse is like [Parse] but panics if s is not a valid ID. Use it for
// constants in code, not for input from a caller.
func MustParse(s string) ID {
	id, err := Parse(s)

	if err != nil {
		panic(err)
	}

	return id
}

// Size returns the size of the ID, or 0 for the zero ID.
func (id ID) Size() Size {
	return id.n
}

// IsZero reports whether id is the zero ID.
func (id ID) IsZero() bool {
	return id.n == 0
}

// Bytes returns a copy of the random bytes in the ID.
// It returns nil for the zero ID.
func (id ID) Bytes() []byte {
	if id.IsZero() {
		return nil
	}

	b := make([]byte, id.n)
	copy(b, id.b[:id.n])

	return b
}

// String returns the ID as a lowercase base32 string.
// It returns the empty string for the zero ID.
func (id ID) String() string {
	if id.IsZero() {
		return ""
	}

	return base32Enc.EncodeToString(id.b[:id.n])
}

// Compare orders IDs by their raw bytes, and then by size. It returns 0 only
// when the two IDs are equal, so it agrees with ==. Use it with
// [slices.SortFunc]:
//
//	slices.SortFunc(ids, sid.ID.Compare)
//
// The order follows the bytes, not the characters. The base32 alphabet puts
// a-z before 2-7, so it does not match the order of the string forms.
func (id ID) Compare(other ID) int {
	if c := bytes.Compare(id.b[:], other.b[:]); c != 0 {
		return c
	}

	return cmp.Compare(id.n, other.n)
}

// MarshalText implements [encoding.TextMarshaler]. This also gives the ID its
// JSON form, and lets it be a key in a JSON object or a YAML document.
func (id ID) MarshalText() ([]byte, error) {
	return id.AppendText(nil)
}

// AppendText implements [encoding.TextAppender].
func (id ID) AppendText(b []byte) ([]byte, error) {
	if id.IsZero() {
		return b, nil
	}

	return base32Enc.AppendEncode(b, id.b[:id.n]), nil
}

// UnmarshalText implements [encoding.TextUnmarshaler].
// Empty text decodes to the zero ID.
func (id *ID) UnmarshalText(text []byte) error {
	parsed, err := Parse(string(text))

	if err != nil {
		return err
	}

	*id = parsed

	return nil
}

// MarshalBinary implements [encoding.BinaryMarshaler]. The binary form is the
// raw bytes, which is shorter than the text form.
func (id ID) MarshalBinary() ([]byte, error) {
	return id.Bytes(), nil
}

// AppendBinary implements [encoding.BinaryAppender].
func (id ID) AppendBinary(b []byte) ([]byte, error) {
	return append(b, id.b[:id.n]...), nil
}

// UnmarshalBinary implements [encoding.BinaryUnmarshaler].
// Empty data decodes to the zero ID.
func (id *ID) UnmarshalBinary(data []byte) error {
	if len(data) == 0 {
		*id = ID{}

		return nil
	}

	parsed, err := FromBytes(data)

	if err != nil {
		return err
	}

	*id = parsed

	return nil
}

// Value implements [driver.Valuer] and stores the string form.
// The zero ID becomes SQL NULL.
func (id ID) Value() (driver.Value, error) {
	if id.IsZero() {
		return nil, nil
	}

	return id.String(), nil
}

// Scan implements [sql.Scanner]. It accepts the string form as a string or as
// bytes. SQL NULL becomes the zero ID.
//
// A column that holds the raw bytes needs [FromBytes] instead, because Scan
// cannot tell the two forms apart.
func (id *ID) Scan(src any) error {
	switch v := src.(type) {
	case nil:
		*id = ID{}

		return nil
	case string:
		return id.UnmarshalText([]byte(v))
	case []byte:
		return id.UnmarshalText(v)
	default:
		return fmt.Errorf("sid: cannot scan %T into an ID: want a string, bytes, or NULL", src)
	}
}

// EncodeMultiple joins the string forms of ids into one string. All ids must
// have the same size, because [DecodeMultiple] splits on a fixed width.
//
// It returns a [SizeError] if any id is the zero ID, and a [MixedSizeError] if
// the sizes differ.
func EncodeMultiple(ids []ID) (string, error) {
	if len(ids) == 0 {
		return "", nil
	}

	size := ids[0].Size()

	if !size.Valid() {
		return "", &SizeError{Bytes: int(size)}
	}

	var b strings.Builder

	b.Grow(len(ids) * size.EncodedLen())

	for _, id := range ids {
		if id.Size() != size {
			return "", &MixedSizeError{First: size, Found: id.Size()}
		}

		b.WriteString(id.String())
	}

	return b.String(), nil
}

// DecodeMultiple splits a string built by [EncodeMultiple] back into IDs of
// the given size. The empty string decodes to an empty slice.
//
// It returns a [SizeError] if size is not supported, and a [ParseError] if the
// length of s is not a whole number of IDs or a segment is not valid.
func DecodeMultiple(s string, size Size) ([]ID, error) {
	if !size.Valid() {
		return nil, &SizeError{Bytes: int(size)}
	}

	width := size.EncodedLen()

	if len(s)%width != 0 {
		return nil, &ParseError{
			Input:  s,
			Reason: fmt.Sprintf("length %d is not a multiple of %d characters", len(s), width),
		}
	}

	ids := make([]ID, 0, len(s)/width)

	for i := 0; i < len(s); i += width {
		id, err := Parse(s[i : i+width])

		if err != nil {
			return nil, err
		}

		ids = append(ids, id)
	}

	return ids, nil
}
