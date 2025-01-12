package sid

import (
	"encoding/json"
	"testing"
)

func TestSID(t *testing.T) {

	id := New()
	parsed := Parse(id.String())

	if !id.Equals(parsed) {
		t.Errorf("unexpected failure: %v != %v",
			id,
			parsed,
		)
	}

}

func TestSIDMulti(t *testing.T) {

	ids := []ID{
		New(),
		New(),
		New(),
	}

	parsed := DecodeMultiple(EncodeMultiple(ids))

	for i, id := range ids {

		if !id.Equals(parsed[i]) {
			t.Errorf("unexpected failure: %v != %v",
				id,
				parsed,
			)
		}

	}

}

type s struct {
	Id ID `json:"id"`
}

func TestJSONSupport(t *testing.T) {

	v1 := s{Id: New()}
	v2 := s{}

	b, err := json.Marshal(v1)
	if err != nil {
		t.Fatalf("unexpected error during json marshalling: %v", err)
	}

	if err := json.Unmarshal(b, &v2); err != nil {
		t.Fatalf("unexpected error during json unmarshalling: %v", err)
	}

	if !v1.Id.Equals(v2.Id) {
		t.Errorf("unexpected failure: %v != %v",
			v1.Id,
			v2.Id,
		)
	}

}
