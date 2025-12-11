# go-butterflymx

A Go client library for interacting with the ButterflyMX API.

> [!NOTE]
> This is an unofficial library and uses reverse-engineered API endpoints. Use
> at your own risk.

## Coverage

- [ ] Authentication -- **will not be implemented** due to 2FA complexity
- [x] Authorization
  - [x] Fetching Rails API Access Token
  - [x] Renewing Rails API Access Token
- [x] Fetching Tenants list
- [x] Fetching Access Points for a Tenant
- [x] Unlocking Door
- [x] Keychains support
  - [x] List
  - [x] Get (by ID)
  - [x] Create
  - [ ] Update
  - [ ] Delete
- [x] Virtual Keys support
  - [x] List (via Keychains)
  - [x] Create (via adding to Keychain)
  - [ ] Update
  - [ ] Delete
