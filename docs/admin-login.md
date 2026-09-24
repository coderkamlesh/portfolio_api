# Auth Module — API Reference

This document is the frontend contract for the complete admin authentication feature: password login, mandatory email OTP, token rotation, session inspection, logout, password recovery/change, and email two-factor status.
For frontend state management, token handling, and integration flow guidance, see [Auth Module — Frontend Guide](./auth.md).


## API surface

| Method | Endpoint | Authentication | Success |
|---|---|---|---:|
| `POST` | `/api/auth/login` | None | `200` |
| `POST` | `/api/auth/2fa/verify` | None | `200` |
| `POST` | `/api/auth/2fa/resend` | None | `200` |
| `POST` | `/api/auth/refresh` | None | `200` |
| `POST` | `/api/auth/password/forgot` | None | `202` |
| `POST` | `/api/auth/password/reset` | None | `200` |
| `GET` | `/api/auth/me` | Bearer access token | `200` |
| `POST` | `/api/auth/logout` | Bearer access token | `204` |
| `POST` | `/api/auth/password/change` | Bearer access token | `200` |
| `GET` | `/api/auth/2fa` | Bearer access token | `200` |
| `POST` | `/api/auth/2fa/email/enable` | Bearer access token | `200` |

## Shared conventions

- The examples use `http://localhost:8080` as the API base URL.
- Send request bodies as `application/json` with exactly one JSON object.
- Unknown request fields are rejected. Request bodies are limited to 64 KiB.
- Every auth response includes `Cache-Control: no-store`; `204` responses have no body.
- RFC 3339 timestamps are emitted in UTC. Durations are integer seconds.
- Branch on `error.code`, not on the human-readable `error.message`.
- Protected endpoints require `Authorization: Bearer <access_token>`. Never send the refresh token as a bearer token.
- CORS defaults to `http://localhost:5173` and `http://localhost:3000`. Add the deployed frontend origin to `AUTH_ALLOWED_ORIGINS`.

## Login

The login endpoint accepts an admin username or email address and password. It is public, but successful password authentication always requires an email OTP before an API session is issued.

## Endpoint

```http
POST /api/auth/login
```

**Authentication:** None  
**Request content type:** `application/json`  
**Successful response:** `200 OK`  
**Response content type:** `application/json; charset=utf-8`  
**Cache policy:** `no-store`

Only one JSON object is accepted. Unknown fields are rejected.

## Request

```json
{
  "identifier": "kamlesh",
  "password": "your-admin-password"
}
```

| Field | Type | Required | Description |
|---|---|---:|---|
| `identifier` | string | Yes | Admin username or email address. Leading and trailing whitespace is ignored. |
| `password` | string | Yes | Admin account password. |

Example request:

```bash
curl -i -X POST "http://localhost:8080/api/auth/login" \
  -H "Content-Type: application/json" \
  -d '{
    "identifier": "kamlesh",
    "password": "your-admin-password"
  }'
```

## Login flow

After the account and password are verified, the API always requires mandatory email OTP:

1. The API creates an email OTP challenge and sends the OTP to the admin email.
2. The response contains `two_factor_required: true` and the challenge.
3. No access or refresh token is returned by the password step.
4. Submit the OTP to `POST /api/auth/2fa/verify`.

There is intentionally no server or admin setting that can disable the login second factor.

## Successful responses

### Login response (mandatory 2FA)

```http
HTTP/1.1 200 OK
Content-Type: application/json; charset=utf-8
Cache-Control: no-store
```

```json
{
  "two_factor_required": true,
  "challenge": {
    "id": "01K0CHALLENGEID0000000000000",
    "purpose": "LOGIN_2FA",
    "email": "k*************e@gmail.com",
    "code_length": 6,
    "expires_at": "2026-09-24T10:10:00Z",
    "expires_in": 600,
    "attempts_left": 5,
    "resend_after": 30
  }
}
```

| Field | Type | Description |
|---|---|---|
| `two_factor_required` | boolean | `true` means the password step succeeded but the session is not authenticated yet. |
| `challenge.id` | string | ID required when completing or resending the login OTP. |
| `challenge.purpose` | string | Login challenge purpose. Current value: `LOGIN_2FA`. |
| `challenge.email` | string | Masked destination email. The actual OTP is never returned by the API. |
| `challenge.code_length` | integer | Expected number of OTP digits. |
| `challenge.expires_at` | RFC 3339 string | Absolute challenge expiry time in UTC. |
| `challenge.expires_in` | integer | Remaining challenge lifetime in seconds. |
| `challenge.attempts_left` | integer | Remaining incorrect OTP attempts. |
| `challenge.resend_after` | integer | Resend cooldown in seconds. |

The next request is:

```http
POST /api/auth/2fa/verify
Content-Type: application/json

{
  "challenge_id": "01K0CHALLENGEID0000000000000",
  "otp": "123456"
}
```

Only the newest login OTP is valid. A new OTP invalidates the previous pending OTP.

### Verify login OTP

```http
POST /api/auth/2fa/verify
```

**Authentication:** None  
**Request:**

```json
{
  "challenge_id": "01K0CHALLENGEID0000000000000",
  "otp": "123456"
}
```

| Field | Type | Required | Description |
|---|---|---:|---|
| `challenge_id` | string | Yes | `challenge.id` returned by login. |
| `otp` | string | Yes | Code from the login email. The configured OTP length is returned as `challenge.code_length`. |

A successful `200 OK` response returns the authenticated session:

```json
{
  "two_factor_required": false,
  "tokens": {
    "token_type": "Bearer",
    "access_token": "<jwt-access-token>",
    "expires_in": 900,
    "expires_at": "2026-09-24T10:15:00Z",
    "refresh_token": "<opaque-refresh-token>",
    "refresh_expires_in": 2592000,
    "refresh_expires_at": "2026-10-24T10:00:00Z"
  },
  "admin": {
    "id": "01K0ADMINID0000000000000000",
    "username": "kamlesh",
    "email": "kamlesh@example.com",
    "is_active": true,
    "last_login_at": "2026-09-23T09:45:00Z"
  }
}
```

Important frontend behavior:

1. Submit the OTP once and disable the submit action while the request is pending.
2. If `error.code` is `invalid_otp`, keep the same challenge and show `attempts_left` from the message only if needed; the server remains authoritative.
3. If the challenge is expired, consumed, locked, or invalid, return the admin to the password screen and start a new login.
4. Never prefill or log the OTP.

Relevant verification errors are `invalid_challenge` (`400`), `invalid_otp` (`401`), `challenge_expired` (`410`), `challenge_already_used` (`410`), `otp_attempts_exceeded` (`429`), and `account_disabled` (`403`).

### Resend login OTP

```http
POST /api/auth/2fa/resend
```

**Authentication:** None  
**Request:**

```json
{
  "challenge_id": "01K0CHALLENGEID0000000000000"
}
```

A successful `200 OK` returns a new `ChallengeView` directly:

```json
{
  "id": "01K0NEWCHALLENGEID000000000000",
  "purpose": "LOGIN_2FA",
  "email": "k*************e@gmail.com",
  "code_length": 6,
  "expires_at": "2026-09-24T10:20:00Z",
  "expires_in": 600,
  "attempts_left": 5,
  "resend_after": 30
}
```

The previous pending login OTP is invalidated. A request before the cooldown returns `429 otp_resend_cooldown` with `Retry-After`; a request after the per-admin OTP window is exhausted returns `429 otp_too_many_requests`. Replace the stored challenge ID and expiry countdown whenever resend succeeds.

### Session issued directly

```http
HTTP/1.1 200 OK
Content-Type: application/json; charset=utf-8
Cache-Control: no-store
```

```json
{
  "two_factor_required": false,
  "tokens": {
    "token_type": "Bearer",
    "access_token": "<jwt-access-token>",
    "expires_in": 900,
    "expires_at": "2026-09-24T10:00:00Z",
    "refresh_token": "<opaque-refresh-token>",
    "refresh_expires_in": 2592000,
    "refresh_expires_at": "2026-10-24T09:45:00Z"
  },
  "admin": {
    "id": "01K0ADMINID0000000000000000",
    "username": "kamlesh",
    "email": "kamlesh@example.com",
    "is_active": true,
    "last_login_at": "2026-09-23T09:45:00Z"
  }
}
```

| Field | Type | Description |
|---|---|---|
| `two_factor_required` | boolean | `false` for an immediately authenticated session. |
| `tokens.token_type` | string | Always `Bearer` for the issued access token. |
| `tokens.access_token` | string | Short-lived JWT access token. Send it as `Authorization: Bearer <access_token>`. |
| `tokens.expires_in` | integer | Access-token lifetime in seconds. Default configuration: 900 seconds. |
| `tokens.expires_at` | RFC 3339 string | Access-token expiry time in UTC. |
| `tokens.refresh_token` | string | Opaque token used only with `POST /api/auth/refresh`. Do not send it as a bearer token. |
| `tokens.refresh_expires_in` | integer | Refresh-token lifetime in seconds. Default configuration: 2,592,000 seconds. |
| `tokens.refresh_expires_at` | RFC 3339 string | Refresh-token expiry time in UTC. |
| `admin` | object | Safe admin projection. It never includes the password hash. |
| `admin.last_login_at` | RFC 3339 string or omitted | Previous login time; omitted when no login has been recorded. |

Treat both tokens as sensitive. Access tokens are short-lived, while refresh tokens are rotating credentials. Do not expose either token in browser URLs or application logs.

## Session refresh

```http
POST /api/auth/refresh
```

**Authentication:** None  
**Request:**

```json
{
  "refresh_token": "<opaque-refresh-token>"
}
```

A successful `200 OK` rotates the refresh token and returns a new session payload:

```json
{
  "two_factor_required": false,
  "tokens": {
    "token_type": "Bearer",
    "access_token": "<new-jwt-access-token>",
    "expires_in": 900,
    "expires_at": "2026-09-24T10:30:00Z",
    "refresh_token": "<new-opaque-refresh-token>",
    "refresh_expires_in": 2592000,
    "refresh_expires_at": "2026-10-24T10:15:00Z"
  },
  "admin": {
    "id": "01K0ADMINID0000000000000000",
    "username": "kamlesh",
    "email": "kamlesh@example.com",
    "is_active": true,
    "last_login_at": "2026-09-24T10:15:00Z"
  }
}
```

The old refresh token becomes invalid immediately. Atomically replace **both** access and refresh tokens in frontend state before retrying the original protected request.

Frontend refresh rules:

1. Allow only one refresh request at a time. Queue concurrent `401` responses behind that single request.
2. Refresh proactively shortly before `tokens.expires_at`, or react to `401 invalid_token` once and retry the original request once.
3. Do not retry non-`401` failures and do not include the refresh token in `Authorization`.
4. On `401 invalid_refresh_token` or `401 refresh_token_reused`, clear the local session and return to login. Reuse detection revokes every remaining session for that admin.
5. On `403 account_disabled`, clear the local session and show the server message.

## Current admin

```http
GET /api/auth/me
Authorization: Bearer <access_token>
```

A successful `200 OK` returns current account, effective 2FA status, and active refresh-session count:

```json
{
  "admin": {
    "id": "01K0ADMINID0000000000000000",
    "username": "kamlesh",
    "email": "kamlesh@example.com",
    "is_active": true,
    "last_login_at": "2026-09-24T10:15:00Z"
  },
  "two_factor": {
    "required": true,
    "method": "EMAIL_OTP",
    "enabled": true,
    "email": "k*************e@gmail.com",
    "confirmed_at": "2026-09-20T08:00:00Z",
    "updated_at": "2026-09-20T08:00:00Z"
  },
  "active_sessions": 2
}
```

Use this endpoint on admin-panel startup when a token exists, and after refresh or password change. The safe admin projection never contains the password hash. `two_factor.required` is the server policy, while `two_factor.enabled` is the effective state after combining policy with the admin setting.

## Logout

```http
POST /api/auth/logout
Authorization: Bearer <access_token>
```

The body is optional. To revoke only the refresh session represented by the supplied token:

```json
{
  "refresh_token": "<opaque-refresh-token>",
  "all_devices": false
}
```

To revoke every refresh session for the admin:

```json
{
  "all_devices": true
}
```

Omitting the body, sending `{}`, or omitting `refresh_token` also revokes every device. A successful response is `204 No Content` with no JSON body. Clear local tokens in a `finally` path after the server responds; if the network fails, still clear local state because a subsequent login will establish a new session.

Logout revokes refresh sessions, but access JWTs are stateless and can remain cryptographically valid until their short expiry. Do not keep using an access token after local logout.

## Password management

### Request password reset

```http
POST /api/auth/password/forgot
```

**Authentication:** None  
**Request:**

```json
{
  "email": "kamlesh@example.com"
}
```

A successful request returns `202 Accepted`. The response shape is the same for known, unknown, inactive, and delivery-failed email addresses:

```json
{
  "message": "If that email belongs to an admin account, a reset code is on its way.",
  "challenge": {
    "id": "01K0RESETCHALLENGEID00000000000",
    "purpose": "PASSWORD_RESET",
    "email": "k*************e@gmail.com",
    "code_length": 6,
    "expires_at": "2026-09-24T11:00:00Z",
    "expires_in": 600,
    "attempts_left": 5,
    "resend_after": 30
  }
}
```

Keep `challenge.id` in the password-reset screen state and show the generic message; do not claim that an account exists. A synthetic challenge returned for an unknown/inactive account cannot be completed. There is no dedicated password-reset resend endpoint: to request another code, call this endpoint again. It creates a new challenge and invalidates the previous pending reset code, subject to the per-admin OTP window.

Expected errors are request validation errors and unexpected server errors. Mail-delivery failure and OTP-window exhaustion are intentionally reported as the normal `202` response so response status/body cannot reveal whether the email exists. Because the generic response cannot reveal that a request was rate-limited, the frontend should tell the user to wait for the configured OTP window before requesting another code. A timing side-channel is not eliminated: a real delivery attempt can take longer than the synthetic unknown-account path.

### Reset password and sign in

```http
POST /api/auth/password/reset
```

**Authentication:** None  
**Request:**

```json
{
  "challenge_id": "01K0RESETCHALLENGEID00000000000",
  "otp": "123456",
  "new_password": "a-new-strong-passphrase-42"
}
```

A successful `200 OK` changes the password, revokes all existing sessions, and returns a fresh authenticated session using the same `LoginResult` shape as login/2FA verification: `two_factor_required: false`, `tokens`, and `admin`. Replace any existing frontend token pair atomically and route to the admin panel.

Password policy: at least `AUTH_MIN_PASSWORD_LENGTH` characters (default 12), at most 256 characters, and a mix of letters with numbers, symbols, or spaces. Relevant errors are `weak_password` (`400`), normal OTP/challenge errors, `invalid_challenge` for a synthetic or unknown challenge, and `internal_error` (`500`).

### Change password while signed in

```http
POST /api/auth/password/change
Authorization: Bearer <access_token>
Content-Type: application/json
```

**Request:**

```json
{
  "current_password": "current-strong-passphrase",
  "new_password": "replacement-strong-passphrase-42"
}
```

A successful `200 OK` returns a new token pair. All existing sessions are revoked before a fresh session is issued for the current device, so immediately replace both returned tokens:

```json
{
  "tokens": {
    "token_type": "Bearer",
    "access_token": "<new-jwt-access-token>",
    "expires_in": 900,
    "expires_at": "2026-09-24T11:30:00Z",
    "refresh_token": "<new-opaque-refresh-token>",
    "refresh_expires_in": 2592000,
    "refresh_expires_at": "2026-10-24T11:00:00Z"
  }
}
```

Use `invalid_password` (`401`) for a wrong current password, `password_unchanged` (`400`) when the new value equals the current value, and `weak_password` (`400`) for a policy violation.

## Email two-factor status

Email OTP is mandatory and cannot be disabled. These endpoints require `Authorization: Bearer <access_token>`.

### Get 2FA status

```http
GET /api/auth/2fa
```

A successful `200 OK` response:

```json
{
  "required": true,
  "method": "EMAIL_OTP",
  "enabled": true,
  "email": "k*************e@gmail.com",
  "confirmed_at": "2026-09-20T08:00:00Z",
  "updated_at": "2026-09-20T08:00:00Z"
}
```

- `required` is always `true`; email OTP is mandatory for login.
- `enabled` is always `true`. A missing row is auto-provisioned during the next login, and a stale disabled row is re-enabled.
- `method` currently has the value `EMAIL_OTP`.
- `email` is always masked.
- Timestamp fields can be omitted when no stored configuration exists.

### Confirm/repair email 2FA

```http
POST /api/auth/2fa/email/enable
Content-Type: application/json
```

**Request:**

```json
{
  "password": "current-admin-password"
}
```

A successful `200 OK` returns the same `TwoFAStatus` shape as `GET /api/auth/2fa`, with fresh timestamps. The operation is idempotent and can repair a stale disabled database row. A wrong password returns `401 invalid_password`.

## Error response

Every failed request uses this envelope:

```json
{
  "error": {
    "code": "invalid_credentials",
    "message": "Invalid username or password."
  }
}
```

The `message` is for display. The frontend must branch on `error.code`.

### Error code reference

| HTTP | Code | Meaning / frontend action |
|---:|---|---|
| `400` | `empty_body` | Required JSON body is missing. |
| `400` | `invalid_json` | Malformed JSON, unknown field, or more than one JSON value. Fix the request. |
| `400` | `body_too_large` | Body exceeds 64 KiB. |
| `400` | `validation_failed` | A required request field is empty. Highlight the named field in `message`. |
| `400` | `invalid_challenge` | Challenge is unknown, synthetic, or has the wrong purpose. Restart the flow. |
| `400` | `weak_password` | New password fails the configured policy. |
| `400` | `password_unchanged` | New password equals the current password. |
| `401` | `invalid_credentials` | Username/email or password is incorrect. |
| `401` | `invalid_otp` | OTP is incorrect; keep a still-valid challenge and let the user retry. |
| `401` | `invalid_refresh_token` | Refresh token is missing, unknown, or expired. Clear session and log in. |
| `401` | `refresh_token_reused` | Rotated token was replayed. All sessions were revoked; clear session and log in. |
| `401` | `invalid_password` | Current-password re-confirmation failed. |
| `401` | `missing_token` | Protected request has no valid bearer header. Refresh or log in. |
| `401` | `invalid_token` | Access token is invalid or expired. Refresh once, then retry once. |
| `403` | `account_disabled` | Admin account is inactive. Clear session and do not retry automatically. |
| `410` | `challenge_expired` | OTP challenge expired. Request/resend a code and restart verification. |
| `410` | `challenge_already_used` | Challenge was consumed or invalidated. Restart the flow. |
| `429` | `too_many_attempts` | Login limiter is active. Honor `Retry-After`. |
| `429` | `otp_too_many_requests` | Per-admin OTP window is exhausted. Honor `Retry-After`. Forgot-password masks this as its generic `202` response. |
| `429` | `otp_attempts_exceeded` | Challenge is locked after too many incorrect OTPs. Restart the flow; no `Retry-After` is emitted. |
| `429` | `otp_resend_cooldown` | Login OTP resend is cooling down. Honor `Retry-After`. |
| `500` | `internal_error` | Unexpected server failure. Do not expose internal details. |
| `502` | `email_delivery_failed` | Login/resend OTP email could not be delivered. Forgot-password intentionally masks this as its generic `202` response. |

Protected-route `401 missing_token` and `401 invalid_token` responses also include:

```http
WWW-Authenticate: Bearer realm="portfolio-api"
```

Rate-limit responses that provide a delay include an integer `Retry-After` header:

```http
HTTP/1.1 429 Too Many Requests
Content-Type: application/json; charset=utf-8
Cache-Control: no-store
Retry-After: 42
```

```json
{
  "error": {
    "code": "too_many_attempts",
    "message": "Too many failed attempts. Try again in 42 seconds."
  }
}
```

## Configuration-dependent behavior

The examples use the defaults in `.env.example`. The frontend should consume returned values such as `code_length`, `expires_in`, `expires_at`, `attempts_left`, and `resend_after` instead of duplicating server policy.

| Variable | Default | Effect |
|---|---:|---|
| `AUTH_ALLOWED_ORIGINS` | Localhost 5173/3000 | Comma-separated frontend origins allowed by CORS. |
| `AUTH_OTP_LENGTH` | `6` | OTP digits returned as `challenge.code_length`; valid range 4–10. |
| `AUTH_OTP_TTL_MINUTES` | `10` | OTP `expires_in` and `expires_at`. |
| `AUTH_OTP_MAX_ATTEMPTS` | `5` | Incorrect OTP attempts allowed per challenge. |
| `AUTH_OTP_MAX_PER_WINDOW` | `5` | OTP challenges allowed per admin in the OTP window. |
| `AUTH_OTP_WINDOW_MINUTES` | `60` | Per-admin OTP request window. |
| `AUTH_OTP_RESEND_COOLDOWN_SECONDS` | `30` | Login OTP resend cooldown exposed as `resend_after`. |
| `AUTH_MIN_PASSWORD_LENGTH` | `12` | Minimum new/reset password length. |
| `AUTH_LOGIN_MAX_ATTEMPTS` | `10` | Failed logins allowed per identifier and client IP. |
| `AUTH_LOGIN_WINDOW_MINUTES` | `15` | Login limiter window. |
| `JWT_ACCESS_TTL_MINUTES` | `15` | Access-token lifetime. |
| `JWT_REFRESH_TTL_DAYS` | `30` | Rotating refresh-token lifetime. |
| `EMAIL_PROVIDER` | `ses` | `ses` uses AWS SES; `log` writes OTP emails to server logs for local development. |

For login, an SES delivery failure returns `502 email_delivery_failed` and no unusable challenge/session. Password-reset delivery failure is intentionally masked by the generic `202` response at the response-content level; request timing is not equalized.

## Recommended frontend state model

Use an explicit auth state rather than several unrelated booleans:

```text
signed_out
  -> authenticating
  -> login_otp
  -> authenticated

signed_out
  -> reset_requested
  -> reset_otp
  -> authenticated

authenticated
  -> refreshing
  -> authenticated | signed_out
```

Store an `AdminView` only after OTP verification, refresh, password reset, or password change returns a token pair. Store each OTP challenge in component/session state with its purpose; never reuse a `LOGIN_2FA` challenge ID for `PASSWORD_RESET` or vice versa.

### Token storage

- Access token: keep in memory when the frontend architecture allows it.
- Refresh token: do not place it in a URL, analytics event, error report, or log. Avoid long-lived plaintext `localStorage`; a refresh token in browser storage is vulnerable to XSS.
- If the admin panel must survive a full browser restart, use the product's approved secure persistence strategy. This backend returns tokens in JSON, so it does not itself provide `HttpOnly` cookie storage.
- Never decode the access JWT for authorization decisions. The backend validates it, and protected responses remain the source of truth.

### Startup and protected requests

1. If no token pair exists, render the login screen.
2. If a token pair exists, validate it with `GET /api/auth/me` before rendering protected UI.
3. Attach only `Authorization: Bearer <access_token>` to protected requests.
4. On `401 invalid_token` or `401 missing_token`, run the single-flight refresh flow once and retry the original request once.
5. Never create a refresh loop. If refresh fails, clear tokens/admin/challenge state and route to login.
6. Cancel or ignore stale UI results after login/logout/reset transitions so an older response cannot overwrite newer auth state.

### End-to-end transitions

1. **Login:** `POST /login`; always retain the returned `challenge` and move to `login_otp`.
2. **Login verification:** `POST /2fa/verify`; save returned tokens/admin. On a terminal challenge error, clear the challenge and restart login.
3. **Login resend:** call `/2fa/resend` only after `resend_after`; replace the challenge and countdown.
4. **Session refresh:** call `/refresh` once; atomically replace the token pair, then resolve queued protected requests.
5. **Current admin:** call `/me` after startup, refresh, reset, or password change when admin data must be reloaded.
6. **Logout:** call `/logout` with `{ refresh_token, all_devices: false }` for the current session or `{ all_devices: true }` for all devices; always clear local auth state.
7. **Forgot password:** call `/password/forgot`; retain the returned reset challenge without revealing whether the account exists.
8. **Reset password:** call `/password/reset`; save the returned fresh tokens/admin and clear reset state.
9. **Change password:** call `/password/change`; atomically replace tokens because all prior sessions were revoked.
10. **2FA status:** load `/2fa`; optionally call `/2fa/email/enable` with current-password confirmation to confirm/repair the mandatory setting.

## Implementation checklist

- Parse non-`204` success responses as JSON; do not attempt to parse the logout body.
- Read `Retry-After` when present and keep resend/buttons disabled for that many seconds.
- Run expiry countdowns from server timestamps/seconds and re-check state on the server.
- Clear OTP and password inputs after terminal success and on component unmount.
- Use generic copy on forgot-password so the UI does not enumerate accounts.
- Display `error.message`, but make state transitions only from `error.code`.
- Configure the exact production frontend origin in `AUTH_ALLOWED_ORIGINS`; do not deploy with `*`.
