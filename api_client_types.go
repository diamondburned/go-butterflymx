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
		Name string       `json:"name"`
		Kind KeychainKind `json:"kind"`
		// StartsAt is when the keychain becomes active.
		StartsAt time.Time `json:"starts_at"`
		// EndsAt is when the keychain expires.
		EndsAt time.Time `json:"ends_at"`
		// TimeFrom is the daily start time for access in the building timezone.
		// For a custom keychain, this is wrong, since the keychain is always
		// active.
		TimeFrom Timestamp `json:"time_from"`
		// TimeTo is the daily end time for access in the building timezone.
		// For a custom keychain, this is wrong, since the keychain is always
		// active.
		TimeTo Timestamp `json:"time_to"`
		// StartDate is the date when access begins in the building timezone.
		StartDate Datestamp `json:"start_date,format:'2006-01-02'"`
		// EndDate is the date when access ends in the building timezone.
		EndDate Datestamp `json:"end_date,format:'2006-01-02'"`
		// Weekdays is the list of weekdays when access is allowed.
		Weekdays []Weekday `json:"weekdays"`
		// AllowUnitAccess indicates if unit access is permitted.
		AllowUnitAccess bool `json:"allow_unit_access"`
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

// Panel represents a physical ButterflyMX panel. Not sure what the relation of
// this is to [AccessPoint], since they both contain a relationship to a
// [Building], but during creation of a keychain, supplying the access point IDs
// will yield the keychain with relations to panels instead.
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
