//go:build goexperiment.jsonv2

package butterflymx

import (
	"encoding/json/v2"
	"fmt"
	"iter"
	"time"
)

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

// String returns the string representation of the PINCode.
func (p PINCode) String() string {
	return string(p)
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

// Resolve resolves all references in the list to their actual resources of type
// T.
func (l ReferenceList[T]) Resolve(refs map[ID]RawReference) iter.Seq2[*T, error] {
	return func(yield func(*T, error) bool) {
		for _, ref := range l {
			resolved, err := ref.Resolve(refs)
			if !yield(resolved, err) {
				return
			}
		}
	}
}

// MarshalJSON implements [json.Marshaler].
func (l ReferenceList[T]) MarshalJSON() ([]byte, error) {
	type Alias []*TypedReference[T]
	return json.Marshal(map[string]any{
		"data": Alias(l),
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
	ID        TaggedID `json:"id" example:"prod-tenant-12345"`
	FirstName string   `json:"firstName" example:"Jane"`
	LastName  string   `json:"lastName" example:"Doe"`
	Name      string   `json:"name" example:"Jane Doe"`
	PINCode   PINCode  `json:"pinCode" example:"012345"`
	Unit      Unit     `json:"unit"`
	Building  Building `json:"building"`
}

// Unit represents a specific unit within a building.
type Unit struct {
	ID          TaggedID `json:"id" example:"prod-unit-40001"`
	Label       string   `json:"label" example:"Apt 4B"`
	FloorNumber int      `json:"floorNumber,string" example:"4"`
}

// Building represents a building with its properties.
type Building struct {
	ID   TaggedID `json:"id" example:"prod-building-40003"`
	GUID string   `json:"guid" example:"b80ca6a6-e0e6-4b8a-8be7-5c56dfca48ff"`
	Name string   `json:"name" example:"Hunter Capital"`
}

// AccessPoint represents a door or entry point that can be unlocked.
type AccessPoint struct {
	ID           TaggedID `json:"id" example:"prod-access_point-50001"`
	Name         string   `json:"name" example:"Front Door"`
	OpenDuration int      `json:"openDuration" example:"5"`
	Online       bool     `json:"online" example:"true"`
}

// Keychain represents a virtual keychain, containing virtual keys and their associated entities.
type Keychain struct {
	ID         ID `json:"id" example:"10001"`
	Attributes struct {
		Name string       `json:"name" example:"Amazon Delivery"`
		Kind KeychainKind `json:"kind" example:"recurring"`
		// StartsAt is when the keychain becomes active.
		StartsAt time.Time `json:"starts_at" example:"2023-01-01T00:00:00Z"`
		// EndsAt is when the keychain expires.
		EndsAt time.Time `json:"ends_at" example:"2023-01-02T00:00:00Z"`
		// TimeFrom is the daily start time for access in the building timezone.
		// For a custom keychain, this is wrong, since the keychain is always
		// active.
		TimeFrom Timestamp `json:"time_from" example:"08:00"`
		// TimeTo is the daily end time for access in the building timezone.
		// For a custom keychain, this is wrong, since the keychain is always
		// active.
		TimeTo Timestamp `json:"time_to" example:"20:00"`
		// StartDate is the date when access begins in the building timezone.
		StartDate Datestamp `json:"start_date,format:'2006-01-02'" example:"2023-01-01"`
		// EndDate is the date when access ends in the building timezone.
		EndDate Datestamp `json:"end_date,format:'2006-01-02'" example:"2023-01-02"`
		// Weekdays is the list of weekdays when access is allowed.
		Weekdays []Weekday `json:"weekdays" example:"[\"mon\", \"tue\"]"`
		// AllowUnitAccess indicates if unit access is permitted.
		AllowUnitAccess bool `json:"allow_unit_access" example:"false"`
	} `json:"attributes"`
	Relationships struct {
		VirtualKeys ReferenceList[VirtualKey] `json:"virtual_keys"`
		Devices     ReferenceList[Panel]      `json:"devices"`
	} `json:"relationships"`
}

// VirtualKey represents an allocated door PIN code for a contact (not
// necessarily a user but usually is).
type VirtualKey struct {
	ID         ID `json:"id" example:"10002"`
	Attributes struct {
		Name            string    `json:"name" example:"john.doe@example.com"`
		Email           string    `json:"email" example:"john.doe@example.com"`
		PINCode         PINCode   `json:"pin" example:"012345"`
		QRCodeImageURL  string    `json:"qr_code_image_url" example:"https://api.butterflymx.com/v3/qr_codes/some-uuid.png"`
		InstructionsURL string    `json:"instructions_url" example:"https://butterflymx.com/instructions/some-uuid"`
		SentAt          time.Time `json:"sent_at" example:"2023-01-01T00:00:00Z"`
	} `json:"attributes"`
	Relationships struct {
		DoorReleases ReferenceList[DoorRelease] `json:"door_releases"`
	} `json:"relationships"`
}

// DoorRelease represents an event of a door being released.
type DoorRelease struct {
	ID         ID `json:"id" example:"30001"`
	Attributes struct {
		ReleaseMethod   string    `json:"release_method" example:"virtual_key_pin"`
		DoorReleaseType string    `json:"door_release_type" example:"visitor"`
		PanelUserType   string    `json:"panel_user_type" example:"default"`
		Name            string    `json:"name" example:"Jane Doe"` // account name
		CreatedAt       time.Time `json:"created_at" example:"2023-01-01T00:00:00Z"`
		LoggedAt        time.Time `json:"logged_at" example:"2023-01-01T00:00:00Z"`
		ThumbURL        string    `json:"thumb_url" example:"https://api.butterflymx.com/v3/door_releases/30001/thumb.jpg"`
		MediumURL       string    `json:"medium_url" example:"https://api.butterflymx.com/v3/door_releases/30001/medium.jpg"`
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

// Panel represents a physical ButterflyMX panel. Not sure what the relation of
// this is to [AccessPoint], since they both contain a relationship to a
// [Building], but during creation of a keychain, supplying the access point IDs
// will yield the keychain with relations to panels instead.
type Panel struct {
	ID         ID `json:"id" example:"10003"`
	Attributes struct {
		Name string `json:"name" example:"Hunter Capital Front Door"`
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
	HasNextPage bool   `json:"hasNextPage" example:"true"`
	EndCursor   string `json:"endCursor" example:"eyJpZCI6IjEwMDAxIn0"`
}
