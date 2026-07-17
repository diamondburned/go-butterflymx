//go:build goexperiment.jsonv2

package main

import (
	"context"
	"net/http"

	"github.com/danielgtaylor/huma/v2"
	butterflymx "libdb.so/go-butterflymx"
)

type TenantsInput struct{}

type TenantsOutput struct {
	Body struct {
		Tenants []butterflymx.Tenant `json:"tenants" doc:"The list of tenants"`
	}
}

type TenantAccessPointsInput struct {
	TenantID butterflymx.TaggedID `path:"tenant_id" doc:"The tagged ID of the tenant, e.g. prod-tenant-123"`
}

type TenantAccessPointsOutput struct {
	Body struct {
		AccessPoints []butterflymx.AccessPoint `json:"access_points" doc:"The list of access points for the tenant"`
	}
}

type UnlockDoorInput struct {
	TenantID      int `path:"tenant_id" doc:"The untagged numeric ID of the tenant"`
	AccessPointID int `path:"access_point_id" doc:"The untagged numeric ID of the access point"`
}

type UnlockDoorOutput struct {
	Status int `default:"204"`
}

func registerTenantRoutes(api huma.API) {
	huma.Register(api, huma.Operation{
		OperationID: "get-tenants",
		Method:      http.MethodGet,
		Path:        "/tenants",
		Summary:     "Get all tenants",
		Description: "Retrieves a list of all tenants associated with the current user.",
	}, func(ctx context.Context, input *TenantsInput) (*TenantsOutput, error) {
		client, err := getClient(ctx)
		if err != nil {
			return nil, err
		}
		tenants, err := butterflymx.CollectResults(client.Tenants(ctx))
		if err != nil {
			return nil, huma.Error500InternalServerError(err.Error())
		}
		resp := &TenantsOutput{}
		resp.Body.Tenants = tenants
		return resp, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "get-tenant-access-points",
		Method:      http.MethodGet,
		Path:        "/tenants/{tenant_id}/access-points",
		Summary:     "Get tenant access points",
		Description: "Retrieves a list of access points (doors) for a given tenant.",
	}, func(ctx context.Context, input *TenantAccessPointsInput) (*TenantAccessPointsOutput, error) {
		client, err := getClient(ctx)
		if err != nil {
			return nil, err
		}
		aps, err := butterflymx.CollectResults(client.TenantAccessPoints(ctx, input.TenantID))
		if err != nil {
			return nil, huma.Error500InternalServerError(err.Error())
		}
		resp := &TenantAccessPointsOutput{}
		resp.Body.AccessPoints = aps
		return resp, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "unlock-door",
		Method:      http.MethodPost,
		Path:        "/tenants/{tenant_id}/access-points/{access_point_id}/unlock",
		Summary:     "Unlock a door",
		Description: "Sends a request to unlock a door (access point) for a given tenant.",
	}, func(ctx context.Context, input *UnlockDoorInput) (*UnlockDoorOutput, error) {
		client, err := getClient(ctx)
		if err != nil {
			return nil, err
		}
		err = client.UnlockDoor(ctx, butterflymx.ID(input.TenantID), butterflymx.ID(input.AccessPointID))
		if err != nil {
			return nil, huma.Error500InternalServerError(err.Error())
		}
		return &UnlockDoorOutput{Status: 204}, nil
	})
}
