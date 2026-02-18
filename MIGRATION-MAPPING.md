# Troo Earth: Express → Go Migration Mapping

This document maps every Express route and component to the planned Go implementation. Use it as the single source of truth during migration.

---

## 1. Global middleware chain (order matters)

| Order | Express | Go (Fiber) |
|-------|---------|------------|
| 0 | Stripe webhook (raw body only) | `POST /api/v1/stripe/webhook` — skip JSON/body parser, use `c.Body()` for raw bytes |
| 1 | `express.json()`, `express.urlencoded()` | Fiber: `app.Use(fiberMiddleware.BodyLimit(...))`, default JSON/query parser |
| 2 | CORS (origin suffix + dev-password) | `internal/middleware/cors.go` — same logic: `FRONTEND_URL_ENDS_WITH`, `dev-password` header |
| 3 | `trust proxy` (1) | Fiber: `app.Settings.EnableTrustedProxyCheck = true` or equivalent |
| 4 | Session (connect-redis, name `troo.sid`) | `internal/middleware/session.go` — SCS RedisStore, prefix `session:`, cookie name `troo.sid`, same flags (httpOnly, secure, sameSite, maxAge 24h) |
| 5 | Health request marker | `internal/middleware/healthmarker.go` — mark request in Redis (skip `/`, `/health*`, favicon) |
| 6 | Response formatter | `internal/middleware/response.go` — inject `res.Success()`, `res.Error()` helpers; response shape: `{ status, message, data, metadata }` / `{ status: "error", error: { message, statusCode, details } }` |
| 7 | Tracing (X-Trace-Id) | `internal/middleware/tracing.go` — UUID in header + context |
| 8 | Route logger | `internal/middleware/routelogger.go` — log enter/exit + duration |
| 9 | `res.locals.user` from session | `internal/middleware/sessionuser.go` — set `c.Locals("user", session.User)` |
| 10 | Health router (no auth) | `internal/handlers/health/` |
| 11 | Auth routes (no auth middleware) | `internal/handlers/auth/` |
| 12 | User + public invitation routes | `internal/handlers/user/`, `internal/handlers/invitations/public` |
| 13 | **Auth required** | `internal/middleware/auth.go` — 401 with `{ success: false, message: "Unauthorized" }` if no session user |
| 14 | Protected routes | org, marketplace, invitations (private), listings, holdings, uploads, trading, transactions, retirements, listing-events |
| 15 | Error logger (mark 500 in Redis) | After handler, before global error handler |
| 16 | Global error handler | `internal/middleware/errorhandler.go` — same JSON shape as Express |

---

## 2. Route → Handler mapping

### Health (no auth)

| Method | Express path | Express handler | Go handler (planned) |
|--------|--------------|-----------------|------------------------|
| GET | `/` | healthRouter (HTML) | `handlers/health.Dashboard` |
| GET | `/reset` | healthRouter (query `key`) | `handlers/health.Reset` |
| GET | `/health/json` | healthRouter | `handlers/health.JSON` |
| GET | `/health/errors` | healthRouter | `handlers/health.Errors` |

**Redis keys (must match):** `health:global:req_total`, `health:global:req_errors`, `health:global:res_time_total`, `health:global:res_count`, `health:global:start_time`, `health:global:last_request`, `health:global:error_log`.

---

### Stripe webhook (no auth, raw body)

| Method | Express path | Express handler | Go handler (planned) |
|--------|--------------|-----------------|------------------------|
| POST | `/api/v1/stripe/webhook` | `buildStripeWebhookExpressHandler()` (utils/stripe-webhook.js) | `handlers/payments.StripeWebhook` |

**Critical:** Use raw body for signature verification; do not parse as JSON. Header: `stripe-signature`. Response: `200` body `ok`, or `400` body `Webhook Error: <message>`.

---

### Auth (session cookie; no auth middleware)

| Method | Express path | Express handler | Go handler (planned) |
|--------|--------------|-----------------|------------------------|
| POST | `/api/v1/auth/login` | authController.loginUserController | `handlers/auth.Login` |
| GET | `/api/v1/auth/me` | authController.verifyUserController | `handlers/auth.Me` |
| DELETE | `/api/v1/auth/logout` | authController.logoutUserController | `handlers/auth.Logout` |

**Session data shape (Redis):** `user` = `{ user_id, fullname, email, role, org_id }`. On login/register: `redis.SAdd("user_sessions:"+user_id, sessionID)`. On logout: `redis.SRem(...)`, clear cookie `troo.sid` (same options as Express).

---

### Users (auth required)

| Method | Express path | Express handler | Permission | Go handler (planned) |
|--------|--------------|-----------------|------------|------------------------|
| POST | `/api/v1/users/create-user` | userController.createUserController | — | `handlers/user.CreateUser` |
| PUT | `/api/v1/users/update-user/:id` | userController.updateUserController | — | `handlers/user.UpdateUser` |
| GET | `/api/v1/users/view-user/:id` | userController.viewUserController | — | `handlers/user.ViewUser` |
| PATCH | `/api/v1/users/update-role` | userController.updateUserRoleController | ASSIGN_ROLE | `handlers/user.UpdateRole` + middleware |
| DELETE | `/api/v1/users/remove-user` | userController.removeUserFromOrgController | REMOVE_USER | `handlers/user.RemoveUser` + middleware |

---

### Invitations – public (no auth)

| Method | Express path | Express handler | Go handler (planned) |
|--------|--------------|-----------------|------------------------|
| POST | `/api/v1/invitations/public/check-token` | invitationController.checkInvitationTokenController | `handlers/invitations.CheckToken` (public) |

---

### Orgs (auth required)

| Method | Express path | Express handler | Go handler (planned) |
|--------|--------------|-----------------|------------------------|
| POST | `/api/v1/orgs/create-org` | orgController.createOrgController | `handlers/org.CreateOrg` |
| GET | `/api/v1/orgs/view-org` | orgController.getOrgByIdController | `handlers/org.ViewOrg` |
| PATCH | `/api/v1/orgs/update-org/:id` | orgController.updateOrgController | `handlers/org.UpdateOrg` |

---

### Uploads (auth required)

| Method | Express path | Express handler | Go handler (planned) |
|--------|--------------|-----------------|------------------------|
| POST | `/api/v1/uploads/org-logo` | uploadController.uploadOrgLogoController | `handlers/uploads.OrgLogo` |
| POST | `/api/v1/uploads/org-doc` | uploadController.uploadOrgDocController | `handlers/uploads.OrgDoc` |

**Backend:** Supabase Storage signed upload URL; buckets `org-logos`, `org-docs`. Response: `{ uploadUrl, publicUrl, path }`.

---

### Marketplace (auth required)

| Method | Express path | Express handler | Go handler (planned) |
|--------|--------------|-----------------|------------------------|
| GET | `/api/v1/marketplace/projects` | marketplaceController.getAllProjects | `handlers/marketplace.GetAllProjects` |
| GET | `/api/v1/marketplace/projects/:id` | marketplaceController.getProjectById | `handlers/marketplace.GetProjectById` |
| POST | `/api/v1/marketplace/admin-sync` | marketplaceController.syncIcrProjects | `handlers/marketplace.AdminSync` |

**Note:** Marketplace controller returns `{ success: true, data }` (not `status`/`message`/`data`). Preserve exactly for parity.

---

### Invitations – private (auth + permission)

| Method | Express path | Express handler | Permission | Go handler (planned) |
|--------|--------------|-----------------|------------|------------------------|
| POST | `/api/v1/invitations/create-invite` | invitationController.sendInviteController | INVITE_USER | `handlers/invitations.SendInvite` |
| POST | `/api/v1/invitations/accept-invite` | invitationController.acceptInvitationController | — | `handlers/invitations.AcceptInvite` |
| PATCH | `/api/v1/invitations/revoke-invite` | invitationController.revokeInvitationController | INVITE_USER | `handlers/invitations.RevokeInvite` |
| GET | `/api/v1/invitations/view-invites` | invitationController.listOrgInvitationsController | VIEW_DATA | `handlers/invitations.ViewInvites` |
| POST | `/api/v1/invitations/resend-invite` | invitationController.resendInvitationController | INVITE_USER | `handlers/invitations.ResendInvite` |

---

### Listings (auth required)

| Method | Express path | Express handler | Go handler (planned) |
|--------|--------------|-----------------|------------------------|
| POST | `/api/v1/listings/create-listing` | listingController.createListingController | `handlers/listing.CreateListing` |
| GET | `/api/v1/listings/get-all-listings` | listingController.getAllListingsController | `handlers/listing.GetAllListings` |
| GET | `/api/v1/listings/get-org-listings` | listingController.getOrgListingsController | `handlers/listing.GetOrgListings` |
| GET | `/api/v1/listings/get-listing/:listing_id` | listingController.getListingByIdController | `handlers/listing.GetListingById` |
| GET | `/api/v1/listings/get-all-active-listings` | listingController.getAllActiveListingsController | `handlers/listing.GetAllActiveListings` |
| GET | `/api/v1/listings/get-all-closed-listings` | listingController.getAllClosedListingsController | `handlers/listing.GetAllClosedListings` |
| GET | `/api/v1/listings/get-org-active-listings` | listingController.getOrgActiveListingsController | `handlers/listing.GetOrgActiveListings` |
| GET | `/api/v1/listings/get-org-closed-listings` | listingController.getOrgClosedListingsController | `handlers/listing.GetOrgClosedListings` |
| PUT | `/api/v1/listings/edit-listing` | listingController.editListingController | `handlers/listing.EditListing` |
| POST | `/api/v1/listings/cancel-listing` | listingController.cancelListingController | `handlers/listing.CancelListing` |

**Response shape note:** Create/GetAll use `{ success, message, data }` with status 201/200; others use `res.success()` → `{ status, message, data }`. Replicate exactly.

---

### Holdings (auth required)

| Method | Express path | Express handler | Go handler (planned) |
|--------|--------------|-----------------|------------------------|
| GET | `/api/v1/holdings/view-holdings` | holdingsController.viewHoldingController | `handlers/holdings.ViewHoldings` |
| POST | `/api/v1/holdings/view-project` | holdingsController.getProjectByHoldingIdController | `handlers/holdings.ViewProject` |

---

### Trading (auth + permission)

| Method | Express path | Express handler | Permission | Go handler (planned) |
|--------|--------------|-----------------|------------|------------------------|
| POST | `/api/v1/trading/buy-credits` | tradingController.buyCreditsController | BUY_CREDITS | `handlers/trading.BuyCredits` |
| POST | `/api/v1/trading/sell-credits` | tradingController.sellCreditsController | SELL_CREDITS | `handlers/trading.SellCredits` |
| POST | `/api/v1/trading/retire-credits` | tradingController.retireCreditsController | RETIRE_CREDITS | `handlers/trading.RetireCredits` |
| POST | `/api/v1/trading/transfer-credits` | tradingController.transferCreditsController | TRANSFER_CREDITS | `handlers/trading.TransferCredits` |

**Buy credits:** Creates Stripe PaymentIntent; actual transfer happens in Stripe webhook (`payment_intent.succeeded`) via `buyCreditsService` in transaction with Payments record + listing/holdings update.

---

### Transactions (auth required)

| Method | Express path | Express handler | Go handler (planned) |
|--------|--------------|-----------------|------------------------|
| GET | `/api/v1/transactions/get-transactions` | transactionsController.viewTransactionsController | `handlers/transactions.GetTransactions` |

---

### Retirements (auth required)

| Method | Express path | Express handler | Go handler (planned) |
|--------|--------------|-----------------|------------------------|
| GET | `/api/v1/retirements/view-org` | retirementController.viewOrgRetirementController | `handlers/retirements.ViewOrg` |
| POST | `/api/v1/retirements/view-one` | retirementController.viewOneRetirementController | `handlers/retirements.ViewOne` |

---

### Listing events (auth required)

| Method | Express path | Express handler | Go handler (planned) |
|--------|--------------|-----------------|------------------------|
| GET | `/api/v1/listing-events/get-org-listing-events` | listingEventsController.getOrgListingEventsController | `handlers/listingevents.GetOrgListingEvents` |

---

## 3. Sequelize model → GORM model mapping

| Express model (file) | Table name | Go model (planned) |
|----------------------|------------|---------------------|
| User (userModel.js) | Users | `internal/models/user.go` → `User` |
| Org (orgModel.js) | Orgs | `internal/models/org.go` → `Org` |
| Listing (listingModel.js) | Listings | `internal/models/listing.go` → `Listing` |
| Holdings (holdingsModel.js) | Holdings | `internal/models/holding.go` → `Holding` |
| Transactions (transactionsModel.js) | Transactions | `internal/models/transaction.go` → `Transaction` |
| Payments (paymentsModel.js) | Payments | `internal/models/payment.go` → `Payment` |
| RetirementCertificate (retirementCertificateModel.js) | RetirementCertificates | `internal/models/retirement_certificate.go` → `RetirementCertificate` |
| ListingEvents (listingEventsModel.js) | ListingEvents | `internal/models/listing_event.go` → `ListingEvent` |
| Invitation (invitationModel.js) | Invitations | `internal/models/invitation.go` → `Invitation` |
| IcrProject (icrProjects.js) | icrProjects | `internal/models/icr_project.go` → `IcrProject` |

**Associations to replicate:** ListingEvents → Listings (listing_id), Listings → seller (org), Holdings → Orgs, Transactions → from_org/to_org/related_listing, RetirementCertificate → org/project/transaction, Invitation → org, User → org_id. Use GORM associations and tags; no separate “models” folder needed if using `internal/domain` or `internal/models` per Go convention.

---

## 4. Service / business logic mapping

| Express service | Go (planned) |
|-----------------|---------------|
| authService | `internal/services/auth.go` |
| userService | `internal/services/user.go` |
| orgService | `internal/services/org.go` |
| uploadService (Supabase signed URL) | `internal/services/upload.go` or `internal/supabase/storage.go` |
| marketplaceService | `internal/services/marketplace.go` (+ ICR integration) |
| invitationService | `internal/services/invitation.go` |
| listingService | `internal/services/listing.go` |
| holdingsService | `internal/services/holdings.go` |
| transactionsService | `internal/services/transactions.go` |
| tradingService | `internal/services/trading.go` (buy/sell/transfer/retire; Stripe PI in handler) |
| retirementService | `internal/services/retirement.go` |
| listingEventsService | `internal/services/listing_events.go` |
| Stripe webhook (buyCreditsService in tx) | Same `trading.BuyCreditsService` in `handlers/payments` webhook |

**Repositories (optional but recommended):** One per aggregate (User, Org, Listing, Holding, Transaction, Payment, RetirementCertificate, ListingEvent, Invitation, IcrProject) in `internal/repositories/` or colocated in `internal/<module>/store.go`.

---

## 5. Constants (roles & permissions)

- **roles.js** → `internal/constants/roles.go`: `Superadmin`, `Admin`, `Manager`, `Viewer`.
- **permissions.js** → `internal/constants/permissions.go`: same permission keys.
- **permissionRoles.js** → `internal/constants/permission_roles.go`: map permission → allowed roles; use in `authorizePermission` middleware.

---

## 6. Config / env

- **config.js** → Viper: `DATABASE_URL_DEV/TEST/PROD`, `NODE_ENV` → `APP_ENV`.
- **session.js** → Same env: `SESSION_SECRET`, `REDIS_URL`, `ALLOW_CROSS_SITE_DEV`, `NODE_ENV` (for secure/sameSite).
- **redis.js** → Redis client for session store + health + user_sessions.
- **database.js** → GORM + pgx; same pooler URL, SSL.
- **supabase.js** → Supabase client for storage (signed URLs).
- **.env** → `.env.example` in `go/` with all keys (no secrets); Stripe, ICR, Sendinblue, etc.

---

## 7. Response shape parity (critical)

- **Success (responseFormatter):**  
  `{ "status": "success", "message": "<string>", "data": <any>, "metadata": {} }`  
  Status code: usually `200` (create-user uses formatter so currently 200; create-listing uses 201 and `{ success, message, data }`).
- **Error (responseFormatter):**  
  `{ "status": "error", "error": { "message": "<string>", "statusCode": <number>, "details": {} } }`
- **Auth 401:**  
  `{ "success": false, "message": "Unauthorized" }`
- **Marketplace:**  
  `{ "success": true, "data": <projects|project> }`
- **Listing create/get-all:**  
  `{ "success": true, "message": "...", "data": ... }` with 201 or 200.

Every handler must return one of these shapes so frontend/mobile see no change.

---

## 8. Module migration order (safe sequence)

1. **health** — no DB, Redis only.
2. **auth** — session + User model.
3. **user** — User CRUD + role/remove (permission middleware).
4. **org** — Org + User update.
5. **uploads** — Supabase signed URL.
6. **marketplace** — IcrProject + ICR integration.
7. **listing** — Listing CRUD + listing events (audit).
8. **holdings** — Holdings read + project by holding.
9. **transactions** — read-only.
10. **trading** — buy (Stripe PI), sell, transfer, retire.
11. **retirements** — RetirementCertificate read.
12. **payments** — Stripe webhook (raw body, signature, buyCredits in tx).
13. **emails** — Sendinblue (invitation emails); can be after invitations.
14. **invitations** — Invitation model + public check-token + private routes.
15. **integrations** — ICR client (used by marketplace); already covered in marketplace.

After each module: unit tests, integration tests, manual Postman verification, then proceed to next.

---

## 9. Session compatibility checklist

- [ ] Cookie name: `troo.sid`
- [ ] Redis store prefix: `session:`
- [ ] Session data key: `user` with `{ user_id, fullname, email, role, org_id }`
- [ ] Cookie: httpOnly, secure (prod when ALLOW_CROSS_SITE_DEV), sameSite (lax or none), maxAge 24h
- [ ] On login/register: SAdd `user_sessions:{user_id}` → sessionID
- [ ] On logout: SRem same; destroy session; clear cookie with same options (domain in prod `.troo.earth` if used in Express)

This ensures existing logged-in users remain valid after cutover.
