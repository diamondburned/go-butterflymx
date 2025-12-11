//go:build goexperiment.jsonv2

package butterflymx

import (
	"encoding/json/jsontext"
	"encoding/json/v2"
	"errors"
	"fmt"
	"log/slog"
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
	Data []T
	Refs map[ID]RawReference
}

// ResultWithReferences holds a single result of type T along with
// a map of references to all related objects.
type ResultWithReferences[T any] struct {
	Data T
	Refs map[ID]RawReference
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
		return nil, fmt.Errorf("reference ID %q not found", ref.ID)
	}

	refData, err := unmarshalReference[T](refDest)
	if err != nil {
		return nil, fmt.Errorf("reference ID %q: failed to unmarshal data: %w", ref.ID, err)
	}

	return refData, nil
}

// RawReference holds the internal representation of a relationship
// reference.
type RawReference struct {
	ID   ID             `json:"id,string"`
	Type ObjectType     `json:"type"`
	Data jsontext.Value `json:",inline"`
}

// unmarshalResultsWithReferences unmarshals a list of RawReference objects
// into a ResultsWithReferences structure, resolving the data field into
// the specified DataT type.
func unmarshalResultsWithReferences[DataT any](data, included []RawReference, slog *slog.Logger) (*ResultsWithReferences[DataT], error) {
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

	slog.Debug(
		"unmarshaled results with references",
		"data_count", len(results.Data),
		"refs_count", len(results.Refs),
		"included_count", len(included))

	return &results, nil
}

func unmarshalResultWithReferences[DataT any](data RawReference, included []RawReference, slog *slog.Logger) (*ResultWithReferences[DataT], error) {
	results, err := unmarshalResultsWithReferences[DataT]([]RawReference{data}, included, slog)
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
