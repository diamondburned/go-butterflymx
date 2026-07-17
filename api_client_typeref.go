//go:build goexperiment.jsonv2

package butterflymx

import (
	"encoding/json/jsontext"
	"encoding/json/v2"
	"errors"
	"fmt"

	"github.com/danielgtaylor/huma/v2"
)

// ObjectType represents the type of an object in the API as a string.
type ObjectType string

const (
	TypeDoorRelease ObjectType = "door_releases"
	TypeKeychain    ObjectType = "keychains"
	TypePanel       ObjectType = "panels"
	TypeVirtualKey  ObjectType = "virtual_keys"
	TypeBuilding    ObjectType = "buildings"
)

// ResultsWithReferences holds a list of results of type T along with
// a map of references to all related objects.
type ResultsWithReferences[T any] struct {
	Data []T                 `json:"data"`
	Refs map[ID]RawReference `json:"refs"`
}

// ResultWithReferences holds a single result of type T along with
// a map of references to all related objects.
type ResultWithReferences[T any] struct {
	Data T                   `json:"data"`
	Refs map[ID]RawReference `json:"refs"`
}

// TypedReference extends from a RawReference to provide type-safe
// resolution of the referenced resource.
type TypedReference[T any] RawReference

// Resolve resolves the relationship reference to the actual resource of type T.
// Each call to Resolve can be quite slow, as it involves lazily unmarshaling
// the referenced object.
func (ref *TypedReference[T]) Resolve(refs map[ID]RawReference) (*T, error) {
	if ref == nil {
		return nil, nil
	}

	refDest, ok := refs[ref.ID]
	if !ok {
		return nil, fmt.Errorf("reference ID %v not found", ref.ID)
	}

	refData, err := unmarshalReference[T](refDest)
	if err != nil {
		return nil, fmt.Errorf("reference ID %v: failed to unmarshal data: %w", ref.ID, err)
	}

	return refData, nil
}

// Schema returns the Huma custom schema for TypedReference.
func (r TypedReference[T]) Schema(registry huma.Registry) *huma.Schema {
	return RawReference(r).Schema(registry)
}

// RawReference holds the internal representation of a relationship
// reference.
type RawReference struct {
	ID   ID             `json:"id,string"`
	Type ObjectType     `json:"type"`
	Data jsontext.Value `json:",inline"`
}

// Schema returns the Huma custom schema for RawReference, resolving it as an
// object containing 'id', 'type', and arbitrary additional properties from the
// inline data field.
func (RawReference) Schema(r huma.Registry) *huma.Schema {
	return &huma.Schema{
		Type: huma.TypeObject,
		Properties: map[string]*huma.Schema{
			"id": {
				Type:        huma.TypeString,
				Description: "The unique identifier of the referenced resource.",
				Examples:    []any{"10001"},
			},
			"type": {
				Type:        huma.TypeString,
				Description: "The type of the referenced resource.",
				Examples:    []any{"keychains"},
			},
		},
		Required:             []string{"id", "type"},
		AdditionalProperties: true,
	}
}

// unmarshalResultsWithReferences unmarshals a list of RawReference objects
// into a ResultsWithReferences structure, resolving the data field into
// the specified DataT type.
func unmarshalResultsWithReferences[DataT any](data, included []RawReference) (*ResultsWithReferences[DataT], error) {
	results := ResultsWithReferences[DataT]{
		Data: make([]DataT, 0, len(data)),
		Refs: make(map[ID]RawReference, len(data)+len(included)),
	}

	for _, raw := range data {
		if raw.Data == nil {
			return nil, fmt.Errorf("object %q: missing data field", raw.ID)
		}

		data, err := unmarshalReference[DataT](raw)
		if err != nil {
			return nil, fmt.Errorf("object %q: %w", raw.ID, err)
		}

		results.Data = append(results.Data, *data)
	}

	for _, raw := range data {
		results.Refs[raw.ID] = raw
	}

	for _, raw := range included {
		if raw.Data == nil {
			return nil, fmt.Errorf("included object %q: missing data field", raw.ID)
		}
		results.Refs[raw.ID] = raw
	}

	return &results, nil
}

func unmarshalResultWithReferences[DataT any](data RawReference, included []RawReference) (*ResultWithReferences[DataT], error) {
	results, err := unmarshalResultsWithReferences[DataT]([]RawReference{data}, included)
	if err != nil {
		return nil, err
	}
	if len(results.Data) != 1 {
		panic("BUG: expected exactly one data object")
	}
	return &ResultWithReferences[DataT]{
		Data: results.Data[0],
		Refs: results.Refs,
	}, nil
}

func unmarshalReference[T any](raw RawReference) (*T, error) {
	// hack to ensure that data still includes the ID and Type fields.
	refOnly := raw
	refOnly.Data = nil

	refOnlyJSON, err := json.Marshal(refOnly)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal reference for data unmarshal: %w", err)
	}

	var data T
	if err := errors.Join(
		json.Unmarshal(refOnlyJSON, &data),
		json.Unmarshal(raw.Data, &data),
	); err != nil {
		return nil, fmt.Errorf("failed to unmarshal reference data: %w", err)
	}

	return &data, nil
}

