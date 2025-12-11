//go:build goexperiment.jsonv2

package butterflymx

import (
	"encoding"
	"encoding/json/v2"
	"errors"
	"fmt"
	"iter"
	"strconv"
	"strings"
	"time"
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

// PINCode represents a door PIN code.
// Every character is guaranteed to be a digit.
type PINCode string

// Digits returns an iterator over the digits in the PINCode.
// If validation fails, the iterator yields nothing.
func (p PINCode) Digits() iter.Seq[int] {
	return func(yield func(int) bool) {
		if err := p.Validate(); err != nil {
			return
		}

		for _, r := range p {
			digit := int(r - '0')
			if !yield(digit) {
				return
			}
		}
	}
}

// Validate checks if the PINCode contains only digits.
func (p PINCode) Validate() error {
	for _, r := range p {
		if r < '0' || r > '9' {
			return fmt.Errorf("invalid PIN code: contains non-digit character %q", r)
		}
	}
	return nil
}

// UnmarshalText implements [encoding.TextUnmarshaler].
func (p *PINCode) UnmarshalText(text []byte) error {
	newPIN := PINCode(text)
	if err := newPIN.Validate(); err != nil {
		return err
	}
	*p = newPIN
	return nil
}

// ReferenceList is a helper type for lists of typed references.
type ReferenceList[T any] []*TypedReference[T]

// MarshalJSON implements [json.Marshaler].
func (l ReferenceList[T]) MarshalJSON() ([]byte, error) {
	return json.Marshal(map[string]any{
		"data": l,
	})
}

// UnmarshalJSON implements [json.Unmarshaler].
func (l *ReferenceList[T]) UnmarshalJSON(data []byte) error {
	var aux struct {
		Data []*TypedReference[T] `json:"data"`
	}
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}
	*l = ReferenceList[T](aux.Data)
	return nil
}

// --- Public API Types ---

// Tenant represents a user's residence information within a building.
type Tenant struct {
	ID        TaggedID `json:"id"`
	FirstName string   `json:"firstName"`
	LastName  string   `json:"lastName"`
	Name      string   `json:"name"`
	PINCode   PINCode  `json:"pinCode"`
	Unit      Unit     `json:"unit"`
	Building  Building `json:"building"`
}

// Unit represents a specific unit within a building.
type Unit struct {
	ID          TaggedID `json:"id"`
	Label       string   `json:"label"`
	FloorNumber int      `json:"floorNumber"`
}

// Building represents a building with its properties.
type Building struct {
	ID   TaggedID `json:"id"`
	GUID string   `json:"guid"`
	Name string   `json:"name"`
}

// AccessPoint represents a door or entry point that can be unlocked.
type AccessPoint struct {
	ID           TaggedID `json:"id"`
	Name         string   `json:"name"`
	OpenDuration int      `json:"openDuration"`
	Online       bool     `json:"online"`
}

// Keychain represents a virtual keychain, containing virtual keys and their associated entities.
type Keychain struct {
	ID         ID `json:"id"`
	Attributes struct {
		Name     string       `json:"name"`
		Kind     KeychainKind `json:"kind"`
		StartsAt time.Time    `json:"starts_at"`
		EndsAt   time.Time    `json:"ends_at"`
		TimeFrom WatchTime    `json:"time_from"`
		TimeTo   WatchTime    `json:"time_to"`
		Weekdays []Weekday    `json:"weekdays"`
	} `json:"attributes"`
	Relationships struct {
		VirtualKeys ReferenceList[VirtualKey] `json:"virtual_keys"`
		Devices     ReferenceList[Panel]      `json:"devices"`
	} `json:"relationships"`
}

// VirtualKey represents an allocated door PIN code for a contact (not
// necessarily a user but usually is).
type VirtualKey struct {
	ID         ID `json:"id"`
	Attributes struct {
		Name            string    `json:"name"`
		Email           string    `json:"email"`
		PINCode         PINCode   `json:"pin"`
		QRCodeImageURL  string    `json:"qr_code_image_url"`
		InstructionsURL string    `json:"instructions_url"`
		SentAt          time.Time `json:"sent_at"`
	} `json:"attributes"`
	Relationships struct {
		DoorReleases ReferenceList[DoorRelease] `json:"door_releases"`
	} `json:"relationships"`
}

// DoorRelease represents an event of a door being released.
type DoorRelease struct {
	ID         ID `json:"id"`
	Attributes struct {
		ReleaseMethod   string    `json:"release_method"`
		DoorReleaseType string    `json:"door_release_type"`
		PanelUserType   string    `json:"panel_user_type"`
		Name            string    `json:"name"` // account name
		CreatedAt       time.Time `json:"created_at"`
		LoggedAt        time.Time `json:"logged_at"`
		ThumbURL        string    `json:"thumb_url"`
		MediumURL       string    `json:"medium_url"`
	} `json:"attributes"`
	Relationships struct {
		Unit struct {
			Data *RawReference `json:"data"`
		} `json:"unit"`
		User struct {
			Data *RawReference `json:"data"`
		} `json:"user"`
		Panel struct {
			Data *TypedReference[Panel] `json:"data"`
		} `json:"panel"`
		Device struct {
			Data *TypedReference[Panel] `json:"data"` // type=panels for some reason?
		} `json:"device"`
	} `json:"relationships"`
}

// Panel represents a physical ButterflyMX panel, or an access point.
type Panel struct {
	ID         ID `json:"id"`
	Attributes struct {
		Name string `json:"name"`
	} `json:"attributes"`
	Relationships struct {
		Building struct {
			Data *RawReference `json:"data"`
		} `json:"building"`
	} `json:"relationships"`
}

// --- Enums and Custom Types ---

// AccessCodeStatus represents the status of an access code.
type AccessCodeStatus string

const (
	ActiveAccessCode AccessCodeStatus = "active"
)

// KeychainKind represents the kind of keychain.
type KeychainKind string

const (
	CustomKeychain    KeychainKind = "custom"
	RecurringKeychain KeychainKind = "recurring"
)

// Weekday represents a day of the week.
type Weekday string

const (
	Monday    Weekday = "mon"
	Tuesday   Weekday = "tue"
	Wednesday Weekday = "wed"
	Thursday  Weekday = "thu"
	Friday    Weekday = "fri"
	Saturday  Weekday = "sat"
	Sunday    Weekday = "sun"
)

// ToTimeWeekday converts the Weekday to [time.Weekday].
func (w Weekday) ToTimeWeekday() time.Weekday {
	switch w {
	case Monday:
		return time.Monday
	case Tuesday:
		return time.Tuesday
	case Wednesday:
		return time.Wednesday
	case Thursday:
		return time.Thursday
	case Friday:
		return time.Friday
	case Saturday:
		return time.Saturday
	case Sunday:
		return time.Sunday
	default:
		return -1
	}
}

// WatchTime represents a time of day in the format [WatchTimeLayout].
type WatchTime struct {
	Hour   int
	Minute int
}

const WatchTimeLayout = "15:04"

var (
	_ encoding.TextMarshaler   = (*WatchTime)(nil)
	_ encoding.TextUnmarshaler = (*WatchTime)(nil)
)

// UnmarshalText implements [encoding.TextUnmarshaler].
func (wt *WatchTime) UnmarshalText(text []byte) error {
	t, err := time.Parse(WatchTimeLayout, string(text))
	if err != nil {
		return err
	}
	*wt = WatchTime{Hour: t.Hour(), Minute: t.Minute()}
	return nil
}

// MarshalText implements [encoding.TextMarshaler].
func (wt WatchTime) MarshalText() ([]byte, error) {
	return []byte(wt.String()), nil
}

// String returns the string representation of the WatchTime.
func (wt WatchTime) String() string {
	return fmt.Sprintf("%02d:%02d", wt.Hour, wt.Minute)
}

// ToTime converts the WatchTime to a time.Time on the given date.
func (wt WatchTime) ToTime(date time.Time) time.Time {
	date = date.Truncate(24 * time.Hour)
	date = date.Add(time.Duration(wt.Hour) * time.Hour)
	date = date.Add(time.Duration(wt.Minute) * time.Minute)
	return date
}

// --- GraphQL Specific Types (can be moved if file is split) ---

const tenantsQuery = `
	query Tenants($after: String) { tenants(after: $after) { pageInfo { ...PageInfoFragment } nodes { ...TenantFragment } } }
	fragment PageInfoFragment on PageInfo { hasNextPage endCursor }
	fragment UnitFragment on Unit { id label floorNumber }
	fragment BuildingFragment on Building { id guid name }
	fragment TenantFragment on Tenant { id firstName lastName name pinCode unit { ...UnitFragment } building { ...BuildingFragment } }
`

type tenantsGraphQLResponse struct {
	Data struct {
		Tenants struct {
			Nodes    []Tenant `json:"nodes"`
			PageInfo PageInfo `json:"pageInfo"`
		} `json:"tenants"`
	} `json:"data"`
}

const tenantAccessPointsQuery = `
	query TenantAccessPoints($ids: [ID!]!, $after: String) { nodes(ids: $ids) { __typename id ... on Tenant { accessPoints(after: $after) { pageInfo { ...PageInfoFragment } nodes { ...AccessPointFragment } } } } }
	fragment PageInfoFragment on PageInfo { hasNextPage endCursor }
	fragment AccessPointFragment on AccessPoint { id name openDuration online }
`

type tenantAccessPointsGraphQLResponse struct {
	Data struct {
		Nodes []struct {
			AccessPoints struct {
				Nodes    []AccessPoint `json:"nodes"`
				PageInfo PageInfo      `json:"pageInfo"`
			} `json:"accessPoints"`
		} `json:"nodes"`
	} `json:"data"`
}

type PageInfo struct {
	HasNextPage bool   `json:"hasNextPage"`
	EndCursor   string `json:"endCursor"`
}
