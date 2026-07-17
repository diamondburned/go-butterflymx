//go:build goexperiment.jsonv2

package main

import (
	"context"
	"net/http"
	"time"

	"github.com/danielgtaylor/huma/v2"
	butterflymx "libdb.so/go-butterflymx"
)

type KeychainsInput struct {
	TenantID int    `path:"tenant_id" doc:"The untagged numeric ID of the tenant"`
	Status   string `query:"status" doc:"The status of the keychains, e.g. active" default:"active"`
}

type KeychainsOutput struct {
	Body struct {
		Keychains *butterflymx.ResultsWithReferences[butterflymx.Keychain] `json:"keychains" doc:"The results with references"`
	}
}

type KeychainInput struct {
	KeychainID int `path:"keychain_id" doc:"The untagged numeric ID of the keychain"`
}

type KeychainOutput struct {
	Body struct {
		Keychain *butterflymx.ResultWithReferences[butterflymx.Keychain] `json:"keychain" doc:"The keychain result with references"`
	}
}

type CustomKeychainArgsInput struct {
	Name            string    `json:"name" doc:"Name of the keychain"`
	StartsAt        time.Time `json:"starts_at" doc:"Start time of the keychain"`
	EndsAt          time.Time `json:"ends_at" doc:"End time of the keychain"`
	AllowUnitAccess bool      `json:"allow_unit_access" doc:"Allow unit access"`
}

type CreateCustomKeychainRequestBody struct {
	AccessPointIDs []int                   `json:"access_point_ids" doc:"The list of access point IDs"`
	Args           CustomKeychainArgsInput `json:"args" doc:"The arguments for the custom keychain"`
}

type CreateCustomKeychainInput struct {
	TenantID int `path:"tenant_id" doc:"The untagged numeric ID of the tenant"`
	Body     CreateCustomKeychainRequestBody
}

type CreateCustomKeychainOutput struct {
	Body struct {
		Keychain *butterflymx.ResultWithReferences[butterflymx.Keychain] `json:"keychain" doc:"The created keychain with references"`
	}
}

type CreateVirtualKeysRequestBody struct {
	Args butterflymx.VirtualKeyArgs `json:"args" doc:"The arguments for the virtual key"`
}

type CreateVirtualKeysInput struct {
	KeychainID int `path:"keychain_id" doc:"The untagged numeric ID of the keychain"`
	Body       CreateVirtualKeysRequestBody
}

type CreateVirtualKeysOutput struct {
	Body struct {
		VirtualKeys *butterflymx.ResultsWithReferences[butterflymx.VirtualKey] `json:"virtual_keys" doc:"The created virtual keys with references"`
	}
}

type RevokeVirtualKeyInput struct {
	KeychainID   int `path:"keychain_id" doc:"The untagged numeric ID of the keychain"`
	VirtualKeyID int `path:"virtual_key_id" doc:"The untagged numeric ID of the virtual key"`
}

type RevokeVirtualKeyOutput struct {
	Status int `default:"204"`
}

func registerKeychainRoutes(api huma.API) {
	huma.Register(api, huma.Operation{
		OperationID: "get-keychains",
		Method:      http.MethodGet,
		Path:        "/tenants/{tenant_id}/keychains",
		Summary:     "Get keychains",
		Description: "Retrieves a rich list of keychains, with all related entities resolved into a convenient structure.",
	}, func(ctx context.Context, input *KeychainsInput) (*KeychainsOutput, error) {
		client, err := getClient(ctx)
		if err != nil {
			return nil, err
		}
		keychains, err := client.Keychains(ctx, butterflymx.ID(input.TenantID), butterflymx.AccessCodeStatus(input.Status))
		if err != nil {
			return nil, huma.Error500InternalServerError(err.Error())
		}
		resp := &KeychainsOutput{}
		resp.Body.Keychains = keychains
		return resp, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "get-keychain",
		Method:      http.MethodGet,
		Path:        "/keychains/{keychain_id}",
		Summary:     "Get a single keychain",
		Description: "Retrieves a single keychain by its ID, along with all related entities resolved.",
	}, func(ctx context.Context, input *KeychainInput) (*KeychainOutput, error) {
		client, err := getClient(ctx)
		if err != nil {
			return nil, err
		}
		keychain, err := client.Keychain(ctx, butterflymx.ID(input.KeychainID))
		if err != nil {
			return nil, huma.Error500InternalServerError(err.Error())
		}
		resp := &KeychainOutput{}
		resp.Body.Keychain = keychain
		return resp, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "create-custom-keychain",
		Method:      http.MethodPost,
		Path:        "/tenants/{tenant_id}/keychains/custom",
		Summary:     "Create custom keychain",
		Description: "Creates a new custom keychain with multiple access points.",
	}, func(ctx context.Context, input *CreateCustomKeychainInput) (*CreateCustomKeychainOutput, error) {
		client, err := getClient(ctx)
		if err != nil {
			return nil, err
		}
		apIDs := make([]butterflymx.ID, len(input.Body.AccessPointIDs))
		for i, id := range input.Body.AccessPointIDs {
			apIDs[i] = butterflymx.ID(id)
		}
		args := butterflymx.CustomKeychainArgs{
			Name:            input.Body.Args.Name,
			StartsAt:        input.Body.Args.StartsAt,
			EndsAt:          input.Body.Args.EndsAt,
			AllowUnitAccess: input.Body.Args.AllowUnitAccess,
		}
		keychain, err := client.CreateCustomKeychain(ctx, butterflymx.ID(input.TenantID), apIDs, args)
		if err != nil {
			return nil, huma.Error500InternalServerError(err.Error())
		}
		resp := &CreateCustomKeychainOutput{}
		resp.Body.Keychain = keychain
		return resp, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "create-virtual-keys",
		Method:      http.MethodPost,
		Path:        "/keychains/{keychain_id}/virtual-keys",
		Summary:     "Create virtual keys",
		Description: "Creates a new virtual key for the given keychain. For each recipient, a virtual key is created.",
	}, func(ctx context.Context, input *CreateVirtualKeysInput) (*CreateVirtualKeysOutput, error) {
		client, err := getClient(ctx)
		if err != nil {
			return nil, err
		}
		vks, err := client.CreateVirtualKeys(ctx, butterflymx.ID(input.KeychainID), input.Body.Args)
		if err != nil {
			return nil, huma.Error500InternalServerError(err.Error())
		}
		resp := &CreateVirtualKeysOutput{}
		resp.Body.VirtualKeys = vks
		return resp, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "revoke-virtual-key",
		Method:      http.MethodDelete,
		Path:        "/keychains/{keychain_id}/virtual-keys/{virtual_key_id}",
		Summary:     "Revoke virtual key",
		Description: "Revokes a virtual key associated with a keychain.",
	}, func(ctx context.Context, input *RevokeVirtualKeyInput) (*RevokeVirtualKeyOutput, error) {
		client, err := getClient(ctx)
		if err != nil {
			return nil, err
		}
		err = client.RevokeVirtualKey(ctx, butterflymx.ID(input.KeychainID), butterflymx.ID(input.VirtualKeyID))
		if err != nil {
			return nil, huma.Error500InternalServerError(err.Error())
		}
		return &RevokeVirtualKeyOutput{Status: 204}, nil
	})
}
