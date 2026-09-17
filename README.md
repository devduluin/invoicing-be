# invoice-service

Backend for **Duluin Invoice** (PRD Phase 1). Go + Fiber + GORM + PostgreSQL + Redis —
same conventions as `acc-master-service` / `accounting-engine-service`.

## Auth

Authentication follows **Duluin Launchpad / SSO**. The frontend has no login form —
it redirects to the Launchpad hub, which sets the shared `app_token` cookie on
`.duluin.com` / `.duluin.id`. This service validates that bearer token on every
request:

- `middlewares.ValidateToken()` → `GET {SSO_URL}/users/signin-cookies` with
  `X-Account-Type: duluin_invoice` (config `SSO_ACCOUNT_TYPE`).
- Result (user id, `secondary_id` → `company_id`, roles, permissions, activation)
  is cached in Redis for 5 minutes, then exposed via `c.Locals` and the
  `middlewares.Get*` helpers.
- Fixed roles (PRD §13): `Invoice Owner`, `Invoice Admin`, `Invoice Viewer` —
  seeded in the SSO repo (`DuluinInvoiceSeeder`). Guard routes with
  `middlewares.RequireRole(...)` / `middlewares.RequirePermission("invoice-...")`.

## Run

```bash
cp .env.example .env      # local defaults match acc-master-service
go run .                  # :8090
```

Local stack (same infra as `acc-master-service`):

- PostgreSQL `localhost:5432` (`postgres` / `12345678`), DB `invoice_service_db`
- Redis `localhost:6379` (`redis123`) — optional; without it the service calls
  SSO on every request instead of using the 5-minute cache
- SSO at `http://sso.test/api` (laragon vhost) — direct, not via the gateway

The frontend reaches this service **through `gateway_v3`** (`http://localhost:9996`):
add `SERVICE_INVOICE=http://localhost:8090` to `gateway_v3/.env` and it exposes
`…/api/proxy/v1/invoice/<path>` → `:8090/api/v1/<path>`.

## Endpoints

| Method | Path              | Auth        | Notes                                  |
|--------|-------------------|-------------|----------------------------------------|
| GET    | `/health`         | none        | liveness                               |
| GET    | `/api/v1/ping`    | none        |                                        |
| GET    | `/api/v1/me`      | SSO token   | resolved identity (roles, permissions) |
| CRUD   | `/api/v1/partners`| SSO token   | Mitra master data (PRD §8)             |

## Conventions

- **Money columns must be `numeric`, never `float`** (PRD §3). `utils.BoolInt`
  stores booleans as `smallint`.
- Multi-tenant: every table carries `company_id` (indexed); repositories filter by
  `middlewares.GetCompanyID(c)`.
- Audit columns `created_by` / `updated_by` on the main tables (PRD §18).
- Layering: `routes → controller → service → repository → model`, interfaces in
  `app/domain/<entity>`.

## Next modules (out of scope for the skeleton)

Sales/purchase invoices, sales orders, purchase orders, goods receipt (GRN/BAST),
bills + three-way matching, taxes & discounts, COA/journal posting, reports. Each
is a new vertical under `app/{domain,model,repository,service,controller}` +
`routes/v1`.
