package sid

import (
	"database/sql"
	"database/sql/driver"
	"encoding"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
	"testing"
)

// allSizes is every supported size, for tests that must cover each one.
var allSizes = []Size{Size40, Size80, Size120, Size160}

func TestSize(t *testing.T) {
	tests := []struct {
		name       string
		size       Size
		wantValid  bool
		wantBits   int
		wantLen    int
		wantString string
	}{
		{"40 bit", Size40, true, 40, 8, "40-bit"},
		{"80 bit", Size80, true, 80, 16, "80-bit"},
		{"120 bit", Size120, true, 120, 24, "120-bit"},
		{"160 bit", Size160, true, 160, 32, "160-bit"},
		{"zero", Size(0), false, 0, 0, "invalid size (0 bytes)"},
		{"unsupported", Size(8), false, 64, 0, "invalid size (8 bytes)"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.size.Valid(); got != tt.wantValid {
				t.Errorf("Valid() = %v, want %v", got, tt.wantValid)
			}

			if got := tt.size.Bits(); got != tt.wantBits {
				t.Errorf("Bits() = %d, want %d", got, tt.wantBits)
			}

			if got := tt.size.EncodedLen(); got != tt.wantLen {
				t.Errorf("EncodedLen() = %d, want %d", got, tt.wantLen)
			}

			if got := tt.size.String(); got != tt.wantString {
				t.Errorf("String() = %q, want %q", got, tt.wantString)
			}
		})
	}
}

// TestEncoding pins the wire format. If these literals ever change, existing
// stored IDs stop decoding.
func TestEncoding(t *testing.T) {
	tests := []struct {
		name  string
		bytes []byte
		want  string
	}{
		{"40 bit zero", []byte{0, 0, 0, 0, 0}, "aaaaaaaa"},
		{"40 bit ones", []byte{0xff, 0xff, 0xff, 0xff, 0xff}, "77777777"},
		{"40 bit sequence", []byte{0x00, 0x11, 0x22, 0x33, 0x44}, "aaisem2e"},
		{"80 bit zero", make([]byte, 10), "aaaaaaaaaaaaaaaa"},
		{"80 bit ones", []byte{0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff}, "7777777777777777"},
		{
			"80 bit sequence",
			[]byte{0x00, 0x11, 0x22, 0x33, 0x44, 0x55, 0x66, 0x77, 0x88, 0x99},
			"aaisem2ekvthpcez",
		},
		{
			"120 bit sequence",
			[]byte{0x00, 0x11, 0x22, 0x33, 0x44, 0x55, 0x66, 0x77, 0x88, 0x99, 0xaa, 0xbb, 0xcc, 0xdd, 0xee},
			"aaisem2ekvthpcezvk54zxpo",
		},
		{
			"160 bit sequence",
			[]byte{
				0x00, 0x11, 0x22, 0x33, 0x44, 0x55, 0x66, 0x77, 0x88, 0x99,
				0xaa, 0xbb, 0xcc, 0xdd, 0xee, 0xff, 0x01, 0x02, 0x03, 0x04,
			},
			"aaisem2ekvthpcezvk54zxpo74aqeaye",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			id, err := Parse(tt.want)
			if err != nil {
				t.Fatalf("Parse(%q) returned error: %v", tt.want, err)
			}

			if got := id.String(); got != tt.want {
				t.Errorf("String() = %q, want %q", got, tt.want)
			}

			if got := id.Bytes(); string(got) != string(tt.bytes) {
				t.Errorf("Bytes() = % x, want % x", got, tt.bytes)
			}

			if got, want := id.Size(), Size(len(tt.bytes)); got != want {
				t.Errorf("Size() = %v, want %v", got, want)
			}
		})
	}
}

// TestDocumentedExamples keeps the example IDs in the package doc and the
// README parsable. A reader can copy one of them into code.
func TestDocumentedExamples(t *testing.T) {
	tests := []struct {
		input string
		want  Size
	}{
		{"ab3nqykg", Size40},
		{"mf7vt2hjcpx6qw4d", Size80},
		{"drn7oow74uhzhjjwl5xasnp6", Size120},
		{"6hhehlttai7vkdg25p6buouh5mju2zbz", Size160},
		{"mf7vt2hjcpx6qw4d2uatqdrm27w4abvu", Size160},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			id, err := Parse(tt.input)
			if err != nil {
				t.Fatalf("Parse(%q) returned error: %v", tt.input, err)
			}

			if got := id.Size(); got != tt.want {
				t.Errorf("Size() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestNew(t *testing.T) {
	for _, size := range allSizes {
		t.Run(size.String(), func(t *testing.T) {
			id, err := New(size)
			if err != nil {
				t.Fatalf("New(%v) returned error: %v", size, err)
			}

			if id.IsZero() {
				t.Error("IsZero() = true, want false")
			}

			if got := id.Size(); got != size {
				t.Errorf("Size() = %v, want %v", got, size)
			}

			if got, want := len(id.String()), size.EncodedLen(); got != want {
				t.Errorf("len(String()) = %d, want %d", got, want)
			}

			if got, want := len(id.Bytes()), int(size); got != want {
				t.Errorf("len(Bytes()) = %d, want %d", got, want)
			}
		})
	}
}

func TestNewUnsupportedSize(t *testing.T) {
	_, err := New(Size(7))

	if err == nil {
		t.Fatal("New(7) returned no error, want SizeError")
	}

	var sizeErr *SizeError

	if !errors.As(err, &sizeErr) {
		t.Fatalf("New(7) error is %T, want *SizeError", err)
	}

	if got, want := sizeErr.Bytes, 7; got != want {
		t.Errorf("SizeError.Bytes = %d, want %d", got, want)
	}

	const want = "sid: unsupported size (7 bytes): use Size40, Size80, Size120, or Size160"

	if got := err.Error(); got != want {
		t.Errorf("Error() = %q, want %q", got, want)
	}
}

func TestNewIsRandom(t *testing.T) {
	const count = 100

	for _, size := range allSizes {
		t.Run(size.String(), func(t *testing.T) {
			seen := make(map[ID]bool, count)

			for i := range count {
				id, err := New(size)
				if err != nil {
					t.Fatalf("New(%v) returned error: %v", size, err)
				}

				if seen[id] {
					t.Fatalf("New(%v) repeated %v after %d draws", size, id, i)
				}

				seen[id] = true
			}
		})
	}
}

func TestRoundTrip(t *testing.T) {
	for _, size := range allSizes {
		t.Run(size.String(), func(t *testing.T) {
			id, err := New(size)
			if err != nil {
				t.Fatalf("New(%v) returned error: %v", size, err)
			}

			parsed, err := Parse(id.String())
			if err != nil {
				t.Fatalf("Parse(%q) returned error: %v", id.String(), err)
			}

			if parsed != id {
				t.Errorf("Parse(String()) = %v, want %v", parsed, id)
			}
		})
	}
}

// TestParseErrors covers the inputs that made the previous version panic.
func TestParseErrors(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			"too short",
			"hello",
			`sid: cannot parse "hello": length 5: want 8, 16, 24, or 32 characters`,
		},
		{
			"old 13 character format",
			"ab3nqykgzx4rw",
			`sid: cannot parse "ab3nqykgzx4rw": length 13: want 8, 16, 24, or 32 characters`,
		},
		{
			"uppercase",
			"AAAAAAAA",
			`sid: cannot parse "AAAAAAAA": uppercase 'A' at offset 0: IDs are lowercase`,
		},
		{
			"digit outside alphabet",
			"aaaaaaa1",
			`sid: cannot parse "aaaaaaa1": character '1' at offset 7 is outside the alphabet a-z and 2-7`,
		},
		{
			"punctuation",
			"aaaaaaa!",
			`sid: cannot parse "aaaaaaa!": character '!' at offset 7 is outside the alphabet a-z and 2-7`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			id, err := Parse(tt.input)

			if err == nil {
				t.Fatalf("Parse(%q) returned no error, want ParseError", tt.input)
			}

			var parseErr *ParseError

			if !errors.As(err, &parseErr) {
				t.Fatalf("Parse(%q) error is %T, want *ParseError", tt.input, err)
			}

			if parseErr.Input != tt.input {
				t.Errorf("ParseError.Input = %q, want %q", parseErr.Input, tt.input)
			}

			if got := err.Error(); got != tt.want {
				t.Errorf("Error() = %q, want %q", got, tt.want)
			}

			if !id.IsZero() {
				t.Errorf("Parse(%q) = %v, want the zero ID on failure", tt.input, id)
			}
		})
	}
}

// TestParseErrorTruncatesInput keeps a hostile input from filling the log.
func TestParseErrorTruncatesInput(t *testing.T) {
	long := strings.Repeat("a", 500)

	_, err := Parse(long)
	if err == nil {
		t.Fatal("Parse(500 characters) returned no error, want ParseError")
	}

	if got, want := len(err.Error()), 200; got > want {
		t.Errorf("len(Error()) = %d, want at most %d", got, want)
	}
}

func TestZeroValue(t *testing.T) {
	var id ID

	if !id.IsZero() {
		t.Error("IsZero() = false, want true")
	}

	if got := id.String(); got != "" {
		t.Errorf("String() = %q, want %q", got, "")
	}

	if got := id.Bytes(); got != nil {
		t.Errorf("Bytes() = % x, want nil", got)
	}

	if got := id.Size(); got != 0 {
		t.Errorf("Size() = %v, want 0", got)
	}

	parsed, err := Parse("")
	if err != nil {
		t.Fatalf(`Parse("") returned error: %v`, err)
	}

	if parsed != id {
		t.Errorf(`Parse("") = %v, want the zero ID`, parsed)
	}
}

func TestJSON(t *testing.T) {
	type record struct {
		ID   ID     `json:"id"`
		Name string `json:"name"`
	}

	t.Run("round trip", func(t *testing.T) {
		for _, size := range allSizes {
			t.Run(size.String(), func(t *testing.T) {
				in := record{ID: MustNew(size), Name: "alice"}

				b, err := json.Marshal(in)
				if err != nil {
					t.Fatalf("Marshal returned error: %v", err)
				}

				var out record

				if err := json.Unmarshal(b, &out); err != nil {
					t.Fatalf("Unmarshal returned error: %v", err)
				}

				if out.ID != in.ID {
					t.Errorf("ID = %v, want %v", out.ID, in.ID)
				}
			})
		}
	})

	t.Run("marshals as a string", func(t *testing.T) {
		in := record{ID: MustParse("aaisem2ekvthpcez"), Name: "alice"}

		b, err := json.Marshal(in)
		if err != nil {
			t.Fatalf("Marshal returned error: %v", err)
		}

		const want = `{"id":"aaisem2ekvthpcez","name":"alice"}`

		if got := string(b); got != want {
			t.Errorf("Marshal = %s, want %s", got, want)
		}
	})

	t.Run("zero ID marshals as an empty string", func(t *testing.T) {
		b, err := json.Marshal(record{Name: "alice"})
		if err != nil {
			t.Fatalf("Marshal returned error: %v", err)
		}

		const want = `{"id":"","name":"alice"}`

		if got := string(b); got != want {
			t.Errorf("Marshal = %s, want %s", got, want)
		}
	})

	// Each of these panicked in the previous version.
	t.Run("invalid input returns an error", func(t *testing.T) {
		tests := []struct {
			name    string
			input   string
			wantErr bool
			wantID  ID
		}{
			{"null", `{"id":null}`, false, ID{}},
			{"empty string", `{"id":""}`, false, ID{}},
			{"missing field", `{"name":"alice"}`, false, ID{}},
			{"bad characters", `{"id":"nope!!!!"}`, true, ID{}},
			{"wrong length", `{"id":"nope"}`, true, ID{}},
			{"old format", `{"id":"ab3nqykgzx4rw"}`, true, ID{}},
			{"number", `{"id":42}`, true, ID{}},
			{"object", `{"id":{"a":1}}`, true, ID{}},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				var out record

				err := json.Unmarshal([]byte(tt.input), &out)

				if tt.wantErr && err == nil {
					t.Fatalf("Unmarshal(%s) returned no error, want one", tt.input)
				}

				if !tt.wantErr && err != nil {
					t.Fatalf("Unmarshal(%s) returned error: %v", tt.input, err)
				}

				if out.ID != tt.wantID {
					t.Errorf("ID = %v, want %v", out.ID, tt.wantID)
				}
			})
		}
	})

	t.Run("works as a map key", func(t *testing.T) {
		id := MustParse("aaisem2ekvthpcez")

		b, err := json.Marshal(map[ID]int{id: 1})
		if err != nil {
			t.Fatalf("Marshal returned error: %v", err)
		}

		const want = `{"aaisem2ekvthpcez":1}`

		if got := string(b); got != want {
			t.Errorf("Marshal = %s, want %s", got, want)
		}
	})
}

func TestEncodeMultiple(t *testing.T) {
	t.Run("round trip", func(t *testing.T) {
		for _, size := range allSizes {
			t.Run(size.String(), func(t *testing.T) {
				ids := []ID{MustNew(size), MustNew(size), MustNew(size)}

				s, err := EncodeMultiple(ids)
				if err != nil {
					t.Fatalf("EncodeMultiple returned error: %v", err)
				}

				if got, want := len(s), 3*size.EncodedLen(); got != want {
					t.Errorf("len = %d, want %d", got, want)
				}

				decoded, err := DecodeMultiple(s, size)
				if err != nil {
					t.Fatalf("DecodeMultiple returned error: %v", err)
				}

				if len(decoded) != len(ids) {
					t.Fatalf("decoded %d IDs, want %d", len(decoded), len(ids))
				}

				for i := range ids {
					if decoded[i] != ids[i] {
						t.Errorf("ID %d = %v, want %v", i, decoded[i], ids[i])
					}
				}
			})
		}
	})

	t.Run("empty", func(t *testing.T) {
		s, err := EncodeMultiple(nil)
		if err != nil {
			t.Fatalf("EncodeMultiple returned error: %v", err)
		}

		if s != "" {
			t.Errorf("EncodeMultiple(nil) = %q, want %q", s, "")
		}

		ids, err := DecodeMultiple("", Size80)
		if err != nil {
			t.Fatalf("DecodeMultiple returned error: %v", err)
		}

		if len(ids) != 0 {
			t.Errorf("DecodeMultiple = %v, want an empty slice", ids)
		}
	})

	t.Run("mixed sizes", func(t *testing.T) {
		_, err := EncodeMultiple([]ID{MustNew(Size80), MustNew(Size40)})

		if err == nil {
			t.Fatal("EncodeMultiple returned no error, want MixedSizeError")
		}

		var mixed *MixedSizeError

		if !errors.As(err, &mixed) {
			t.Fatalf("error is %T, want *MixedSizeError", err)
		}

		const want = "sid: mixed sizes: first ID is 80-bit but found 40-bit: encode each size separately"

		if got := err.Error(); got != want {
			t.Errorf("Error() = %q, want %q", got, want)
		}
	})

	t.Run("zero ID", func(t *testing.T) {
		_, err := EncodeMultiple([]ID{{}})

		var sizeErr *SizeError

		if !errors.As(err, &sizeErr) {
			t.Fatalf("error is %T, want *SizeError", err)
		}
	})
}

func TestDecodeMultipleErrors(t *testing.T) {
	t.Run("trailing characters", func(t *testing.T) {
		id := MustNew(Size80)

		_, err := DecodeMultiple(id.String()+"ab", Size80)

		if err == nil {
			t.Fatal("DecodeMultiple returned no error, want ParseError")
		}

		var parseErr *ParseError

		if !errors.As(err, &parseErr) {
			t.Fatalf("error is %T, want *ParseError", err)
		}

		if got, want := parseErr.Reason, "length 18 is not a multiple of 16 characters"; got != want {
			t.Errorf("Reason = %q, want %q", got, want)
		}
	})

	t.Run("unsupported size", func(t *testing.T) {
		_, err := DecodeMultiple("aaaaaaaa", Size(7))

		var sizeErr *SizeError

		if !errors.As(err, &sizeErr) {
			t.Fatalf("error is %T, want *SizeError", err)
		}
	})

	t.Run("invalid segment", func(t *testing.T) {
		_, err := DecodeMultiple("aaaaaaaaaaaaaaa!", Size80)

		var parseErr *ParseError

		if !errors.As(err, &parseErr) {
			t.Fatalf("error is %T, want *ParseError", err)
		}
	})
}

func TestComparable(t *testing.T) {
	a := MustParse("aaisem2ekvthpcez")
	b := MustParse("aaisem2ekvthpcez")
	c := MustNew(Size80)

	if a != b {
		t.Error("two IDs parsed from the same string are not equal")
	}

	if a == c {
		t.Error("a parsed ID equals an unrelated random ID")
	}

	// An ID of one size must never equal an ID of another, even when the
	// shorter one is a prefix of the longer.
	short := MustParse("aaisem2e")
	long := MustParse("aaisem2ekvthpcez")

	if short == long {
		t.Error("a 40-bit ID equals an 80-bit ID with the same prefix")
	}

	counts := map[ID]int{}
	counts[a]++
	counts[b]++

	if counts[a] != 2 {
		t.Errorf("map lookup counted %d, want 2", counts[a])
	}
}

// TestParseNeverPanics locks in the fix for the panics in the previous
// version. Parse takes input from callers, so it must always return an error.
func TestParseNeverPanics(t *testing.T) {
	inputs := []string{
		"",
		"a",
		"hello",
		"ab3nqykgzx4rw",
		"AAAAAAAA",
		"aaaaaaa!",
		"........",
		"\x00\x00\x00\x00\x00\x00\x00\x00",
		"ütf8ütf8",
		"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
	}

	for _, input := range inputs {
		t.Run(input, func(t *testing.T) {
			defer func() {
				if r := recover(); r != nil {
					t.Fatalf("Parse(%q) panicked: %v", input, r)
				}
			}()

			if _, err := Parse(input); err != nil && input == "" {
				t.Errorf(`Parse("") returned error: %v`, err)
			}
		})
	}
}

// FuzzParse checks that Parse never panics and that every accepted string is
// the one and only encoding of the ID it decodes to.
func FuzzParse(f *testing.F) {
	seeds := []string{
		"",
		"aaaaaaaa",
		"77777777",
		"aaisem2ekvthpcez",
		"aaisem2ekvthpcezvk54zxpo",
		"aaisem2ekvthpcezvk54zxpo74aqeaye",
		"AAAAAAAA",
		"ab3nqykgzx4rw",
		"aaaaaaa!",
	}

	for _, seed := range seeds {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, s string) {
		id, err := Parse(s)

		if err != nil {
			return
		}

		if got := id.String(); got != s {
			t.Errorf("Parse(%q).String() = %q: the same ID has two encodings", s, got)
		}

		again, err := Parse(id.String())
		if err != nil {
			t.Fatalf("Parse(%q) accepted but Parse of its own output failed: %v", s, err)
		}

		if again != id {
			t.Errorf("re-parsing %q gave a different ID", s)
		}
	})
}

// FuzzDecodeMultiple covers the fixed-width split that DecodeMultiple does.
// FuzzParse reaches one segment at a time, so it cannot reach that split.
func FuzzDecodeMultiple(f *testing.F) {
	seeds := []struct {
		s    string
		size uint8
	}{
		{"", 5},
		{"aaisem2e", 5},
		{"aaisem2e77777777", 5},
		{"aaisem2ekvthpcez", 10},
		{"aaisem2ekvthpcezvk54zxpo", 15},
		{"aaisem2ekvthpcezvk54zxpo74aqeaye", 20},
		{"aaisem2e", 10},
		{"aaisem2eAAAAAAAA", 5},
		{"aaisem2e!", 5},
		{"aaaaaaaa", 7},
		{"aaaaaaaa", 0},
	}

	for _, seed := range seeds {
		f.Add(seed.s, seed.size)
	}

	f.Fuzz(func(t *testing.T, s string, sizeByte uint8) {
		size := Size(sizeByte)

		ids, err := DecodeMultiple(s, size)

		if err != nil {
			return
		}

		if !size.Valid() {
			t.Fatalf("DecodeMultiple(%q, %d) accepted an unsupported size", s, sizeByte)
		}

		if got, want := len(ids), len(s)/size.EncodedLen(); got != want {
			t.Errorf("DecodeMultiple(%q, %v) gave %d IDs, want %d", s, size, got, want)
		}

		for i, id := range ids {
			if id.Size() != size {
				t.Errorf("ID %d of DecodeMultiple(%q, %v) has size %v", i, s, size, id.Size())
			}
		}

		back, err := EncodeMultiple(ids)
		if err != nil {
			t.Fatalf("EncodeMultiple rejected the IDs from DecodeMultiple(%q, %v): %v", s, size, err)
		}

		if back != s {
			t.Errorf("EncodeMultiple(DecodeMultiple(%q, %v)) = %q: the same IDs have two encodings", s, size, back)
		}
	})
}

func TestFixedSizeConstructors(t *testing.T) {
	tests := []struct {
		name string
		new  func() ID
		want Size
	}{
		{"New40", New40, Size40},
		{"New80", New80, Size80},
		{"New120", New120, Size120},
		{"New160", New160, Size160},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			id := tt.new()

			if got := id.Size(); got != tt.want {
				t.Errorf("Size() = %v, want %v", got, tt.want)
			}

			if id.IsZero() {
				t.Error("IsZero() = true, want false")
			}

			if got, want := len(id.String()), tt.want.EncodedLen(); got != want {
				t.Errorf("len(String()) = %d, want %d", got, want)
			}
		})
	}
}

func TestFromBytes(t *testing.T) {
	t.Run("reverses Bytes", func(t *testing.T) {
		for _, size := range allSizes {
			t.Run(size.String(), func(t *testing.T) {
				id := MustNew(size)

				back, err := FromBytes(id.Bytes())
				if err != nil {
					t.Fatalf("FromBytes returned error: %v", err)
				}

				if back != id {
					t.Errorf("FromBytes(Bytes()) = %v, want %v", back, id)
				}
			})
		}
	})

	t.Run("known bytes", func(t *testing.T) {
		id, err := FromBytes([]byte{0x00, 0x11, 0x22, 0x33, 0x44})
		if err != nil {
			t.Fatalf("FromBytes returned error: %v", err)
		}

		if got, want := id.String(), "aaisem2e"; got != want {
			t.Errorf("String() = %q, want %q", got, want)
		}
	})

	t.Run("does not alias the caller slice", func(t *testing.T) {
		b := []byte{0x00, 0x11, 0x22, 0x33, 0x44}

		id, err := FromBytes(b)
		if err != nil {
			t.Fatalf("FromBytes returned error: %v", err)
		}

		b[0] = 0xff

		if got, want := id.String(), "aaisem2e"; got != want {
			t.Errorf("String() = %q after the caller changed the slice, want %q", got, want)
		}
	})

	t.Run("wrong length", func(t *testing.T) {
		tests := []struct {
			name  string
			bytes []byte
			want  int
		}{
			{"empty", []byte{}, 0},
			{"seven", make([]byte, 7), 7},
			{"old eight byte ID", make([]byte, 8), 8},
			{"too long", make([]byte, 40), 40},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				_, err := FromBytes(tt.bytes)

				var sizeErr *SizeError

				if !errors.As(err, &sizeErr) {
					t.Fatalf("error is %T, want *SizeError", err)
				}

				if sizeErr.Bytes != tt.want {
					t.Errorf("SizeError.Bytes = %d, want %d", sizeErr.Bytes, tt.want)
				}
			})
		}
	})
}

func TestParseSize(t *testing.T) {
	tests := []struct {
		input   string
		want    Size
		wantErr bool
	}{
		{"40", Size40, false},
		{"80", Size80, false},
		{"120", Size120, false},
		{"160", Size160, false},
		{"40-bit", Size40, false},
		{"80-bit", Size80, false},
		{"120-bit", Size120, false},
		{"160-bit", Size160, false},
		{"64", 0, true},
		{"0", 0, true},
		{"200", 0, true},
		{"", 0, true},
		{"eighty", 0, true},
		{"80 bits", 0, true},
		{"-80", 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := ParseSize(tt.input)

			if tt.wantErr {
				if err == nil {
					t.Fatalf("ParseSize(%q) = %v, want an error", tt.input, got)
				}

				return
			}

			if err != nil {
				t.Fatalf("ParseSize(%q) returned error: %v", tt.input, err)
			}

			if got != tt.want {
				t.Errorf("ParseSize(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestParseSizeErrorMessage(t *testing.T) {
	_, err := ParseSize("64")

	if err == nil {
		t.Fatal("ParseSize(\"64\") returned no error, want one")
	}

	const want = `sid: cannot parse size "64": use 40, 80, 120, or 160`

	if got := err.Error(); got != want {
		t.Errorf("Error() = %q, want %q", got, want)
	}
}

// TestSizeInConfig covers a size that arrives in a configuration file.
func TestSizeInConfig(t *testing.T) {
	type config struct {
		IDSize Size `json:"id_size"`
	}

	t.Run("decodes", func(t *testing.T) {
		var c config

		if err := json.Unmarshal([]byte(`{"id_size":"120-bit"}`), &c); err != nil {
			t.Fatalf("Unmarshal returned error: %v", err)
		}

		if c.IDSize != Size120 {
			t.Errorf("IDSize = %v, want %v", c.IDSize, Size120)
		}
	})

	t.Run("round trips", func(t *testing.T) {
		b, err := json.Marshal(config{IDSize: Size80})
		if err != nil {
			t.Fatalf("Marshal returned error: %v", err)
		}

		const want = `{"id_size":"80-bit"}`

		if got := string(b); got != want {
			t.Errorf("Marshal = %s, want %s", got, want)
		}
	})

	t.Run("rejects an unsupported size", func(t *testing.T) {
		var c config

		if err := json.Unmarshal([]byte(`{"id_size":"64"}`), &c); err == nil {
			t.Error("Unmarshal returned no error, want one")
		}
	})

	t.Run("rejects marshalling an invalid size", func(t *testing.T) {
		if _, err := json.Marshal(config{IDSize: Size(7)}); err == nil {
			t.Error("Marshal returned no error, want one")
		}
	})
}

func TestCompare(t *testing.T) {
	t.Run("agrees with equality", func(t *testing.T) {
		a := MustParse("aaisem2ekvthpcez")
		b := MustParse("aaisem2ekvthpcez")

		if got := a.Compare(b); got != 0 {
			t.Errorf("Compare of equal IDs = %d, want 0", got)
		}

		c := New80()

		if (a.Compare(c) == 0) != (a == c) {
			t.Error("Compare and == disagree")
		}
	})

	t.Run("orders by bytes", func(t *testing.T) {
		low := MustParse("aaaaaaaa")
		high := MustParse("77777777")

		if got := low.Compare(high); got >= 0 {
			t.Errorf("Compare(low, high) = %d, want a negative number", got)
		}

		if got := high.Compare(low); got <= 0 {
			t.Errorf("Compare(high, low) = %d, want a positive number", got)
		}
	})

	t.Run("orders a shared prefix by size", func(t *testing.T) {
		short := MustParse("aaisem2e")
		long := MustParse("aaisem2eaaaaaaaa")

		if got := short.Compare(long); got >= 0 {
			t.Errorf("Compare(40-bit, 80-bit) = %d, want a negative number", got)
		}
	})

	t.Run("sorts", func(t *testing.T) {
		ids := []ID{
			MustParse("77777777"),
			MustParse("aaaaaaaa"),
			MustParse("mmmmmmmm"),
		}

		slices.SortFunc(ids, ID.Compare)

		want := []string{"aaaaaaaa", "mmmmmmmm", "77777777"}

		for i, id := range ids {
			if id.String() != want[i] {
				t.Errorf("position %d = %q, want %q", i, id, want[i])
			}
		}
	})
}

func TestAppendText(t *testing.T) {
	id := MustParse("aaisem2ekvthpcez")

	got, err := id.AppendText([]byte("id="))
	if err != nil {
		t.Fatalf("AppendText returned error: %v", err)
	}

	const want = "id=aaisem2ekvthpcez"

	if string(got) != want {
		t.Errorf("AppendText = %q, want %q", got, want)
	}

	t.Run("zero ID appends nothing", func(t *testing.T) {
		got, err := ID{}.AppendText([]byte("id="))
		if err != nil {
			t.Fatalf("AppendText returned error: %v", err)
		}

		if string(got) != "id=" {
			t.Errorf("AppendText = %q, want %q", got, "id=")
		}
	})
}

func TestBinary(t *testing.T) {
	t.Run("round trip", func(t *testing.T) {
		for _, size := range allSizes {
			t.Run(size.String(), func(t *testing.T) {
				id := MustNew(size)

				b, err := id.MarshalBinary()
				if err != nil {
					t.Fatalf("MarshalBinary returned error: %v", err)
				}

				if len(b) != int(size) {
					t.Errorf("len = %d, want %d", len(b), int(size))
				}

				var back ID

				if err := back.UnmarshalBinary(b); err != nil {
					t.Fatalf("UnmarshalBinary returned error: %v", err)
				}

				if back != id {
					t.Errorf("UnmarshalBinary = %v, want %v", back, id)
				}
			})
		}
	})

	t.Run("zero ID", func(t *testing.T) {
		b, err := ID{}.MarshalBinary()
		if err != nil {
			t.Fatalf("MarshalBinary returned error: %v", err)
		}

		var back ID

		if err := back.UnmarshalBinary(b); err != nil {
			t.Fatalf("UnmarshalBinary returned error: %v", err)
		}

		if !back.IsZero() {
			t.Errorf("UnmarshalBinary = %v, want the zero ID", back)
		}
	})

	t.Run("append", func(t *testing.T) {
		id := MustParse("aaisem2e")

		got, err := id.AppendBinary([]byte{0xaa})
		if err != nil {
			t.Fatalf("AppendBinary returned error: %v", err)
		}

		want := []byte{0xaa, 0x00, 0x11, 0x22, 0x33, 0x44}

		if string(got) != string(want) {
			t.Errorf("AppendBinary = % x, want % x", got, want)
		}
	})

	t.Run("wrong length", func(t *testing.T) {
		var id ID

		if err := id.UnmarshalBinary(make([]byte, 8)); err == nil {
			t.Error("UnmarshalBinary returned no error, want one")
		}
	})
}

func TestSQL(t *testing.T) {
	t.Run("round trip", func(t *testing.T) {
		id := MustNew(Size80)

		v, err := id.Value()
		if err != nil {
			t.Fatalf("Value returned error: %v", err)
		}

		if got, want := v, driver.Value(id.String()); got != want {
			t.Errorf("Value = %v, want %v", got, want)
		}

		var back ID

		if err := back.Scan(v); err != nil {
			t.Fatalf("Scan returned error: %v", err)
		}

		if back != id {
			t.Errorf("Scan = %v, want %v", back, id)
		}
	})

	t.Run("zero ID becomes NULL", func(t *testing.T) {
		v, err := ID{}.Value()
		if err != nil {
			t.Fatalf("Value returned error: %v", err)
		}

		if v != nil {
			t.Errorf("Value = %v, want nil", v)
		}
	})

	t.Run("scan", func(t *testing.T) {
		tests := []struct {
			name    string
			src     any
			want    ID
			wantErr bool
		}{
			{"null", nil, ID{}, false},
			{"string", "aaisem2ekvthpcez", MustParse("aaisem2ekvthpcez"), false},
			{"bytes", []byte("aaisem2ekvthpcez"), MustParse("aaisem2ekvthpcez"), false},
			{"empty string", "", ID{}, false},
			{"invalid string", "nope", ID{}, true},
			{"int", 42, ID{}, true},
			{"time", struct{}{}, ID{}, true},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				var id ID

				err := id.Scan(tt.src)

				if tt.wantErr {
					if err == nil {
						t.Fatalf("Scan(%v) returned no error, want one", tt.src)
					}

					return
				}

				if err != nil {
					t.Fatalf("Scan(%v) returned error: %v", tt.src, err)
				}

				if id != tt.want {
					t.Errorf("Scan(%v) = %v, want %v", tt.src, id, tt.want)
				}
			})
		}
	})

	t.Run("scan error names the type", func(t *testing.T) {
		var id ID

		err := id.Scan(42)

		const want = "sid: cannot scan int into an ID: want a string, bytes, or NULL"

		if got := err.Error(); got != want {
			t.Errorf("Error() = %q, want %q", got, want)
		}
	})
}

// TestInterfaces fails to compile if an interface is not satisfied.
func TestInterfaces(t *testing.T) {
	var (
		_ encoding.TextMarshaler     = ID{}
		_ encoding.TextUnmarshaler   = &ID{}
		_ encoding.TextAppender      = ID{}
		_ encoding.BinaryMarshaler   = ID{}
		_ encoding.BinaryUnmarshaler = &ID{}
		_ encoding.BinaryAppender    = ID{}
		_ driver.Valuer              = ID{}
		_ sql.Scanner                = &ID{}
		_ fmt.Stringer               = ID{}
		_ encoding.TextMarshaler     = Size80
		_ encoding.TextUnmarshaler   = new(Size)
		_ fmt.Stringer               = Size80
	)
}
