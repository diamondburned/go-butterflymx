//go:build goexperiment.jsonv2

package main

import (
	"context"
	"iter"

	"github.com/danielgtaylor/huma/v2"
	butterflymx "libdb.so/go-butterflymx"
)

type contextKey string

const (
	tokenContextKey  contextKey = "bfmx-token"
	clientContextKey contextKey = "bfmx-client"
)

// ButterflyMXClient defines the interface for interacting with the ButterflyMX library.
type ButterflyMXClient interface {
	Tenants(ctx context.Context) iter.Seq2[butterflymx.Tenant, error]
	TenantAccessPoints(ctx context.Context, tenantID butterflymx.TaggedID) iter.Seq2[butterflymx.AccessPoint, error]
	UnlockDoor(ctx context.Context, tenantID butterflymx.ID, accessPointID butterflymx.ID) error
	Keychains(ctx context.Context, tenantID butterflymx.ID, status butterflymx.AccessCodeStatus) (*butterflymx.ResultsWithReferences[butterflymx.Keychain], error)
	Keychain(ctx context.Context, keychainID butterflymx.ID) (*butterflymx.ResultWithReferences[butterflymx.Keychain], error)
	CreateCustomKeychain(ctx context.Context, tenantID butterflymx.ID, accessPointIDs []butterflymx.ID, args butterflymx.CustomKeychainArgs) (*butterflymx.ResultWithReferences[butterflymx.Keychain], error)
	CreateVirtualKeys(ctx context.Context, keychainID butterflymx.ID, virtualKeyArgs butterflymx.VirtualKeyArgs) (*butterflymx.ResultsWithReferences[butterflymx.VirtualKey], error)
	RevokeVirtualKey(ctx context.Context, keychainID, virtualKeyID butterflymx.ID) error
}

func getClient(ctx context.Context) (ButterflyMXClient, error) {
	if mock, ok := ctx.Value(clientContextKey).(ButterflyMXClient); ok {
		return mock, nil
	}
	token, _ := ctx.Value(tokenContextKey).(string)
	if token == "" {
		return nil, huma.Error400BadRequest("missing ButterflyMX API token (pass X-ButterflyMX-API-Token header or configure BUTTERFLYMX_API_TOKEN)")
	}
	return butterflymx.NewAPIClient(butterflymx.APIStaticToken(token), nil), nil
}
