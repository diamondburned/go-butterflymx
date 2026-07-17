//go:build goexperiment.jsonv2

package main

import (
	"bytes"
	"context"
	"encoding/json/v2"
	"iter"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/alecthomas/assert/v2"
	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humachi"
	"github.com/go-chi/chi/v5"
	butterflymx "libdb.so/go-butterflymx"
)

type mockButterflyMXClient struct {
	tenantsFunc              func(ctx context.Context) iter.Seq2[butterflymx.Tenant, error]
	tenantAccessPointsFunc   func(ctx context.Context, tenantID butterflymx.TaggedID) iter.Seq2[butterflymx.AccessPoint, error]
	unlockDoorFunc           func(ctx context.Context, tenantID butterflymx.ID, accessPointID butterflymx.ID) error
	keychainsFunc            func(ctx context.Context, tenantID butterflymx.ID, status butterflymx.AccessCodeStatus) (*butterflymx.ResultsWithReferences[butterflymx.Keychain], error)
	keychainFunc             func(ctx context.Context, keychainID butterflymx.ID) (*butterflymx.ResultWithReferences[butterflymx.Keychain], error)
	createCustomKeychainFunc func(ctx context.Context, tenantID butterflymx.ID, accessPointIDs []butterflymx.ID, args butterflymx.CustomKeychainArgs) (*butterflymx.ResultWithReferences[butterflymx.Keychain], error)
	createVirtualKeysFunc    func(ctx context.Context, keychainID butterflymx.ID, virtualKeyArgs butterflymx.VirtualKeyArgs) (*butterflymx.ResultsWithReferences[butterflymx.VirtualKey], error)
	revokeVirtualKeyFunc     func(ctx context.Context, keychainID, virtualKeyID butterflymx.ID) error
}

func (m *mockButterflyMXClient) Tenants(ctx context.Context) iter.Seq2[butterflymx.Tenant, error] {
	if m.tenantsFunc != nil {
		return m.tenantsFunc(ctx)
	}
	return func(yield func(butterflymx.Tenant, error) bool) {}
}

func (m *mockButterflyMXClient) TenantAccessPoints(ctx context.Context, tenantID butterflymx.TaggedID) iter.Seq2[butterflymx.AccessPoint, error] {
	if m.tenantAccessPointsFunc != nil {
		return m.tenantAccessPointsFunc(ctx, tenantID)
	}
	return func(yield func(butterflymx.AccessPoint, error) bool) {}
}

func (m *mockButterflyMXClient) UnlockDoor(ctx context.Context, tenantID butterflymx.ID, accessPointID butterflymx.ID) error {
	if m.unlockDoorFunc != nil {
		return m.unlockDoorFunc(ctx, tenantID, accessPointID)
	}
	return nil
}

func (m *mockButterflyMXClient) Keychains(ctx context.Context, tenantID butterflymx.ID, status butterflymx.AccessCodeStatus) (*butterflymx.ResultsWithReferences[butterflymx.Keychain], error) {
	if m.keychainsFunc != nil {
		return m.keychainsFunc(ctx, tenantID, status)
	}
	return &butterflymx.ResultsWithReferences[butterflymx.Keychain]{}, nil
}

func (m *mockButterflyMXClient) Keychain(ctx context.Context, keychainID butterflymx.ID) (*butterflymx.ResultWithReferences[butterflymx.Keychain], error) {
	if m.keychainFunc != nil {
		return m.keychainFunc(ctx, keychainID)
	}
	return &butterflymx.ResultWithReferences[butterflymx.Keychain]{}, nil
}

func (m *mockButterflyMXClient) CreateCustomKeychain(ctx context.Context, tenantID butterflymx.ID, accessPointIDs []butterflymx.ID, args butterflymx.CustomKeychainArgs) (*butterflymx.ResultWithReferences[butterflymx.Keychain], error) {
	if m.createCustomKeychainFunc != nil {
		return m.createCustomKeychainFunc(ctx, tenantID, accessPointIDs, args)
	}
	return &butterflymx.ResultWithReferences[butterflymx.Keychain]{}, nil
}

func (m *mockButterflyMXClient) CreateVirtualKeys(ctx context.Context, keychainID butterflymx.ID, virtualKeyArgs butterflymx.VirtualKeyArgs) (*butterflymx.ResultsWithReferences[butterflymx.VirtualKey], error) {
	if m.createVirtualKeysFunc != nil {
		return m.createVirtualKeysFunc(ctx, keychainID, virtualKeyArgs)
	}
	return &butterflymx.ResultsWithReferences[butterflymx.VirtualKey]{}, nil
}

func (m *mockButterflyMXClient) RevokeVirtualKey(ctx context.Context, keychainID, virtualKeyID butterflymx.ID) error {
	if m.revokeVirtualKeyFunc != nil {
		return m.revokeVirtualKeyFunc(ctx, keychainID, virtualKeyID)
	}
	return nil
}

func setupTestAPI(mockClient ButterflyMXClient) http.Handler {
	r := chi.NewRouter()

	// Context Middleware to inject the mock client
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := context.WithValue(r.Context(), clientContextKey, mockClient)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	})

	config := huma.DefaultConfig("Test API", "1.0.0")
	api := humachi.New(r, config)

	registerRoutes(api)

	return r
}

func TestGetTenants(t *testing.T) {
	mockClient := &mockButterflyMXClient{
		tenantsFunc: func(ctx context.Context) iter.Seq2[butterflymx.Tenant, error] {
			return func(yield func(butterflymx.Tenant, error) bool) {
				yield(butterflymx.Tenant{
					ID:        butterflymx.NewTaggedID("tenant", 12345),
					FirstName: "Jane",
					LastName:  "Doe",
					Name:      "Jane Doe",
					Unit: butterflymx.Unit{
						ID: butterflymx.NewTaggedID("unit", 67890),
					},
					Building: butterflymx.Building{
						ID: butterflymx.NewTaggedID("building", 11111),
					},
				}, nil)
			}
		},
	}

	handler := setupTestAPI(mockClient)
	req := httptest.NewRequest(http.MethodGet, "/tenants", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp struct {
		Tenants []butterflymx.Tenant `json:"tenants"`
	}
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.Equal(t, 1, len(resp.Tenants))
	assert.Equal(t, "Jane Doe", resp.Tenants[0].Name)
}

func TestGetTenantAccessPoints(t *testing.T) {
	var capturedID butterflymx.TaggedID
	mockClient := &mockButterflyMXClient{
		tenantAccessPointsFunc: func(ctx context.Context, tenantID butterflymx.TaggedID) iter.Seq2[butterflymx.AccessPoint, error] {
			capturedID = tenantID
			return func(yield func(butterflymx.AccessPoint, error) bool) {
				yield(butterflymx.AccessPoint{
					ID:     butterflymx.NewTaggedID("access_point", 54321),
					Name:   "Front Gate",
					Online: true,
				}, nil)
			}
		},
	}

	handler := setupTestAPI(mockClient)
	req := httptest.NewRequest(http.MethodGet, "/tenants/prod-tenant-12345/access-points", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, butterflymx.NewTaggedID("tenant", 12345), capturedID)

	var resp struct {
		AccessPoints []butterflymx.AccessPoint `json:"access_points"`
	}
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.Equal(t, 1, len(resp.AccessPoints))
	assert.Equal(t, "Front Gate", resp.AccessPoints[0].Name)
}

func TestUnlockDoor(t *testing.T) {
	var capturedTenantID, capturedAPID butterflymx.ID
	mockClient := &mockButterflyMXClient{
		unlockDoorFunc: func(ctx context.Context, tenantID butterflymx.ID, accessPointID butterflymx.ID) error {
			capturedTenantID = tenantID
			capturedAPID = accessPointID
			return nil
		},
	}

	handler := setupTestAPI(mockClient)
	req := httptest.NewRequest(http.MethodPost, "/tenants/123/access-points/456/unlock", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNoContent, w.Code)
	assert.Equal(t, butterflymx.ID(123), capturedTenantID)
	assert.Equal(t, butterflymx.ID(456), capturedAPID)
}

func TestGetKeychains(t *testing.T) {
	var capturedTenantID butterflymx.ID
	var capturedStatus butterflymx.AccessCodeStatus
	mockClient := &mockButterflyMXClient{
		keychainsFunc: func(ctx context.Context, tenantID butterflymx.ID, status butterflymx.AccessCodeStatus) (*butterflymx.ResultsWithReferences[butterflymx.Keychain], error) {
			capturedTenantID = tenantID
			capturedStatus = status

			kc := butterflymx.Keychain{ID: 789}
			kc.Attributes.Name = "Guest Keys"
			kc.Attributes.Kind = butterflymx.CustomKeychain
			kc.Attributes.StartDate = butterflymx.Datestamp{Year: 2026, Month: time.January, Day: 1}
			kc.Attributes.EndDate = butterflymx.Datestamp{Year: 2026, Month: time.January, Day: 2}

			return &butterflymx.ResultsWithReferences[butterflymx.Keychain]{
				Data: []butterflymx.Keychain{kc},
			}, nil
		},
	}

	handler := setupTestAPI(mockClient)
	req := httptest.NewRequest(http.MethodGet, "/tenants/123/keychains?status=active", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, butterflymx.ID(123), capturedTenantID)
	assert.Equal(t, butterflymx.ActiveAccessCode, capturedStatus)

	var resp struct {
		Keychains butterflymx.ResultsWithReferences[butterflymx.Keychain] `json:"keychains"`
	}
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.Equal(t, 1, len(resp.Keychains.Data))
	assert.Equal(t, "Guest Keys", resp.Keychains.Data[0].Attributes.Name)
}

func TestGetKeychain(t *testing.T) {
	var capturedKeychainID butterflymx.ID
	mockClient := &mockButterflyMXClient{
		keychainFunc: func(ctx context.Context, keychainID butterflymx.ID) (*butterflymx.ResultWithReferences[butterflymx.Keychain], error) {
			capturedKeychainID = keychainID
			kc := butterflymx.Keychain{ID: keychainID}
			kc.Attributes.Name = "Jane's Keychain"
			kc.Attributes.StartDate = butterflymx.Datestamp{Year: 2026, Month: time.January, Day: 1}
			kc.Attributes.EndDate = butterflymx.Datestamp{Year: 2026, Month: time.January, Day: 2}
			return &butterflymx.ResultWithReferences[butterflymx.Keychain]{
				Data: kc,
			}, nil
		},
	}

	handler := setupTestAPI(mockClient)
	req := httptest.NewRequest(http.MethodGet, "/keychains/789", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, butterflymx.ID(789), capturedKeychainID)

	var resp struct {
		Keychain butterflymx.ResultWithReferences[butterflymx.Keychain] `json:"keychain"`
	}
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.Equal(t, butterflymx.ID(789), resp.Keychain.Data.ID)
	assert.Equal(t, "Jane's Keychain", resp.Keychain.Data.Attributes.Name)
}

func TestCreateCustomKeychain(t *testing.T) {
	var capturedTenantID butterflymx.ID
	var capturedAccessPointIDs []butterflymx.ID
	var capturedArgs butterflymx.CustomKeychainArgs

	mockClient := &mockButterflyMXClient{
		createCustomKeychainFunc: func(ctx context.Context, tenantID butterflymx.ID, accessPointIDs []butterflymx.ID, args butterflymx.CustomKeychainArgs) (*butterflymx.ResultWithReferences[butterflymx.Keychain], error) {
			capturedTenantID = tenantID
			capturedAccessPointIDs = accessPointIDs
			capturedArgs = args

			kc := butterflymx.Keychain{ID: 999}
			kc.Attributes.Name = args.Name
			kc.Attributes.Kind = butterflymx.CustomKeychain
			kc.Attributes.StartDate = butterflymx.Datestamp{Year: 2026, Month: time.January, Day: 1}
			kc.Attributes.EndDate = butterflymx.Datestamp{Year: 2026, Month: time.January, Day: 2}
			return &butterflymx.ResultWithReferences[butterflymx.Keychain]{
				Data: kc,
			}, nil
		},
	}

	handler := setupTestAPI(mockClient)

	starts := time.Now().Truncate(time.Second)
	ends := starts.Add(24 * time.Hour)

	reqBody := CreateCustomKeychainRequestBody{
		AccessPointIDs: []int{555, 666},
	}
	reqBody.Args.Name = "Special Guest"
	reqBody.Args.StartsAt = starts
	reqBody.Args.EndsAt = ends
	reqBody.Args.AllowUnitAccess = true

	b, err := json.Marshal(reqBody)
	assert.NoError(t, err)

	req := httptest.NewRequest(http.MethodPost, "/tenants/123/keychains/custom", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, butterflymx.ID(123), capturedTenantID)
	assert.Equal(t, []butterflymx.ID{555, 666}, capturedAccessPointIDs)
	assert.Equal(t, "Special Guest", capturedArgs.Name)
	assert.Equal(t, starts.Format(time.RFC3339), capturedArgs.StartsAt.Format(time.RFC3339))
	assert.True(t, capturedArgs.AllowUnitAccess)

	var resp struct {
		Keychain butterflymx.ResultWithReferences[butterflymx.Keychain] `json:"keychain"`
	}
	err = json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.Equal(t, butterflymx.ID(999), resp.Keychain.Data.ID)
}

func TestCreateVirtualKeys(t *testing.T) {
	var capturedKeychainID butterflymx.ID
	var capturedArgs butterflymx.VirtualKeyArgs

	mockClient := &mockButterflyMXClient{
		createVirtualKeysFunc: func(ctx context.Context, keychainID butterflymx.ID, virtualKeyArgs butterflymx.VirtualKeyArgs) (*butterflymx.ResultsWithReferences[butterflymx.VirtualKey], error) {
			capturedKeychainID = keychainID
			capturedArgs = virtualKeyArgs

			vk := butterflymx.VirtualKey{ID: 123}
			vk.Attributes.Name = "Virtual Key Name"
			return &butterflymx.ResultsWithReferences[butterflymx.VirtualKey]{
				Data: []butterflymx.VirtualKey{vk},
			}, nil
		},
	}

	handler := setupTestAPI(mockClient)

	reqBody := CreateVirtualKeysRequestBody{
		Args: butterflymx.VirtualKeyArgs{
			Recipients: []butterflymx.VirtualKeyRecipient{
				{
					Name:      "test",
					DeliverTo: "test@example.com",
				},
			},
		},
	}

	b, err := json.Marshal(reqBody)
	assert.NoError(t, err)

	req := httptest.NewRequest(http.MethodPost, "/keychains/789/virtual-keys", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, butterflymx.ID(789), capturedKeychainID)
	assert.Equal(t, 1, len(capturedArgs.Recipients))
	assert.Equal(t, "test@example.com", capturedArgs.Recipients[0].DeliverTo)
}

func TestRevokeVirtualKey(t *testing.T) {
	var capturedKeychainID, capturedVirtualKeyID butterflymx.ID
	mockClient := &mockButterflyMXClient{
		revokeVirtualKeyFunc: func(ctx context.Context, keychainID, virtualKeyID butterflymx.ID) error {
			capturedKeychainID = keychainID
			capturedVirtualKeyID = virtualKeyID
			return nil
		},
	}

	handler := setupTestAPI(mockClient)
	req := httptest.NewRequest(http.MethodDelete, "/keychains/123/virtual-keys/456", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNoContent, w.Code)
	assert.Equal(t, butterflymx.ID(123), capturedKeychainID)
	assert.Equal(t, butterflymx.ID(456), capturedVirtualKeyID)
}
