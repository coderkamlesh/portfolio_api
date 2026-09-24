# Auth Module — Frontend Guide

This guide explains how the frontend should integrate admin authentication. For complete request/response examples, use [Auth Module — API Reference](./admin-login.md).

## Scope

- Password login using username or email.
- Mandatory email OTP after every successful password login.
- JWT access token plus rotating refresh token.
- Session inspection, logout, refresh, password recovery, and password change.
- Email 2FA status for the signed-in admin.

Email OTP cannot be disabled for login. The password step never returns API tokens.

## Endpoint map

| Method | Endpoint | Access token required | Purpose |
|---|---|---:|---|
| `POST` | `/api/auth/login` | No | Verify password and create a login OTP challenge |
| `POST` | `/api/auth/2fa/verify` | No | Verify login OTP and return tokens |
| `POST` | `/api/auth/2fa/resend` | No | Replace a login OTP challenge |
| `POST` | `/api/auth/refresh` | No | Rotate a refresh token and return a new token pair |
| `POST` | `/api/auth/password/forgot` | No | Start password recovery without account enumeration |
| `POST` | `/api/auth/password/reset` | No | Verify reset OTP and set a new password |
| `GET` | `/api/auth/me` | Yes | Load current admin, 2FA status, and session count |
| `POST` | `/api/auth/logout` | Yes | Revoke current or all sessions |
| `POST` | `/api/auth/password/change` | Yes | Change password and rotate to fresh tokens |
| `GET` | `/api/auth/2fa` | Yes | Load mandatory email 2FA status |
| `POST` | `/api/auth/2fa/email/enable` | Yes | Confirm/repair the mandatory email 2FA row |

Protected endpoints use only the access token:

```http
Authorization: Bearer <access_token>
```

Never send the refresh token as a bearer token.

## Frontend state machine

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

Keep the OTP challenge in component or session state together with its `purpose`. Do not reuse a challenge ID across `LOGIN_2FA` and `PASSWORD_RESET`.

## Token handling

- Store the access token in memory when the frontend architecture allows it.
- Do not put the refresh token in a URL, analytics event, error report, or log.
- Avoid long-lived plaintext `localStorage` for refresh tokens because XSS can read browser storage.
- This API returns tokens in JSON; it does not provide `HttpOnly` cookie storage.
- Do not decode the access JWT for authorization decisions. Protected API responses are the source of truth.

## API client rules

1. Attach `Authorization: Bearer <access_token>` only to protected endpoints.
2. For protected `401 missing_token` or `401 invalid_token`, run one single-flight refresh request.
3. Atomically replace the complete token pair returned by refresh.
4. Retry the original protected request once after a successful refresh.
5. If refresh fails, clear tokens, admin data, and challenge state; route to login.
6. Do not run the refresh interceptor for public auth failures such as wrong password or wrong OTP.
7. Parse every non-`204` success response as JSON. Logout returns `204` with no body.
8. Branch on `error.code`; display `error.message` only as human-readable copy.
9. Honor `Retry-After` when a `429` response includes it.

## Flow recipes

### Login

1. Submit `identifier` and `password` to `POST /api/auth/login`.
2. Always expect `two_factor_required: true` with a `challenge` on valid credentials.
3. Save the challenge for the OTP screen; do not mark the admin as authenticated yet.
4. Submit `challenge_id` and `otp` to `POST /api/auth/2fa/verify`.
5. Save the returned token pair and admin profile only after OTP verification succeeds.

### Resend login OTP

1. Disable the resend action until `challenge.resend_after` has elapsed.
2. Call `POST /api/auth/2fa/resend` with the current `challenge_id`.
3. Replace the old challenge with the returned challenge and restart the countdown.
4. On `otp_resend_cooldown`, use the `Retry-After` value instead of a local guess.

### Startup session restore

1. If no token pair exists, render login.
2. If a token pair exists, call `GET /api/auth/me` with the access token.
3. If the access token is expired, run the single-flight refresh flow and retry `/me`.
4. Render protected admin UI only after `/me` succeeds.

### Logout

Call `POST /api/auth/logout` with `{ "refresh_token": "...", "all_devices": false }` for the current session. Use `all_devices: true` when signing out everywhere. Clear local auth state even if the network request fails after the user confirms logout.

### Password reset

`POST /api/auth/password/forgot` intentionally returns the same generic response whether or not the email exists. Keep that copy generic in the UI. A successful reset returns fresh tokens and admin data; save them atomically and clear the reset challenge.

### Password change

`POST /api/auth/password/change` revokes previous sessions and returns a fresh token pair. Replace the old pair atomically. If the request fails, keep the existing authenticated state unless the error indicates the token itself is no longer valid.

## Common error codes

| HTTP | `error.code` | Frontend action |
|---:|---|---|
| `400` | `invalid_json`, `empty_body`, `body_too_large`, `validation_failed` | Fix request/form state |
| `401` | `invalid_credentials` | Show generic login failure |
| `401` | `invalid_otp` | Stay on OTP screen and show attempts left |
| `401` | `invalid_refresh_token` | Clear auth state and route to login |
| `401` | `refresh_token_reused` | Clear all local auth state and force login |
| `403` | `account_disabled` | Block login and show account-disabled copy |
| `410` | `challenge_expired`, `challenge_already_used` | Discard challenge and restart the flow |
| `429` | `too_many_attempts`, `otp_attempts_exceeded`, `otp_too_many_requests`, `otp_resend_cooldown` | Disable the action and honor `Retry-After` when present |
| `502` | `email_delivery_failed` | Ask the user to retry; OTP was not delivered |

## CORS

The frontend origin must exactly match an entry in the API's `AUTH_ALLOWED_ORIGINS` environment variable. Local defaults are `http://localhost:5173` and `http://localhost:3000`. Do not deploy production with `*`.

