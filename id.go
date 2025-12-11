package butterflymx

import (
	"encoding"
	"encoding/json/v2"
	"errors"
	"fmt"
	"strconv"
	"strings"
)

// ErrInvalidTaggedID is returned when a TaggedID is invalid.
var ErrInvalidTaggedID = errors.New("invalid TaggedID")

// ID is an untagged numeric ID.
type ID int

var (
	_ json.Marshaler   = ID(0)
	_ json.Unmarshaler = (*ID)(nil)
)

// MarshalJSON implements [json.Marshaler].
func (id ID) MarshalJSON() ([]byte, error) {
	return json.Marshal(strconv.Itoa(int(id)))
}

// UnmarshalJSON implements [json.Unmarshaler].
func (id *ID) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}
	n, err := strconv.Atoi(s)
	if err != nil {
		return fmt.Errorf("invalid ID: %w", err)
	}
	*id = ID(n)
	return nil
}

// TaggedID is a string of type `prod-{type}-{id}`.
type TaggedID struct {
	Prefix string // prod
	Type   string // e.g., tenant, unit, building
	Number ID     // numeric ID
}

var (
	_ fmt.Stringer             = (*TaggedID)(nil)
	_ encoding.TextMarshaler   = (*TaggedID)(nil)
	_ encoding.TextUnmarshaler = (*TaggedID)(nil)
)

// NewTaggedID creates a new TaggedID with the "prod" prefix.
func NewTaggedID(typ string, id ID) TaggedID {
	return TaggedID{"prod", typ, id}
}

// String returns the string representation of the TaggedID.
func (t TaggedID) String() string {
	return fmt.Sprintf("%s-%s-%d", t.Prefix, t.Type, t.Number)
}

// MarshalText implements [encoding.TextMarshaler].
func (t TaggedID) MarshalText() ([]byte, error) {
	return []byte(t.String()), nil
}

// UnmarshalText implements [encoding.TextUnmarshaler].
func (t *TaggedID) UnmarshalText(text []byte) error {
	parts := strings.SplitN(string(text), "-", 3)
	if len(parts) < 3 || parts[0] != "prod" || parts[1] == "" {
		return ErrInvalidTaggedID
	}
	id, err := strconv.Atoi(parts[2])
	if err != nil {
		return ErrInvalidTaggedID
	}
	*t = TaggedID{
		Prefix: parts[0],
		Type:   parts[1],
		Number: ID(id),
	}
	return nil
}

// TaggedIDsToNumbers converts a slice of TaggedID to a slice of ID.
func TaggedIDsToNumbers(taggedIDs []TaggedID) []ID {
	ids := make([]ID, len(taggedIDs))
	for i, tID := range taggedIDs {
		ids[i] = tID.Number
	}
	return ids
}
