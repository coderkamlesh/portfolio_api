# Skills Module — Frontend Guide

The Skills module powers the grouped skills section of the public portfolio and the category/skill editor of the admin panel.

- The **public** endpoint is open and returns categories with their skills nested.
- The **admin** endpoints are a full CRUD surface protected by the access token.
- Deleting a category deletes every skill inside it in one atomic step.

## Endpoint map

| Method | Endpoint | Access token required | Success |
|---|---|---:|---:|
| `GET` | `/api/public/skills` | No | `200` |
| `GET` | `/api/admin/skill-categories` | Yes | `200` |
| `POST` | `/api/admin/skill-categories` | Yes | `201` |
| `PUT` | `/api/admin/skill-categories/{id}` | Yes | `200` |
| `DELETE` | `/api/admin/skill-categories/{id}` | Yes | `204` |
| `GET` | `/api/admin/skills` | Yes | `200` |
| `POST` | `/api/admin/skills` | Yes | `201` |
| `PUT` | `/api/admin/skills/{id}` | Yes | `200` |
| `DELETE` | `/api/admin/skills/{id}` | Yes | `204` |

Base URL locally is `http://localhost:8080`; in production it is the deployed Lambda endpoint. All examples below use `[base-url]` as a placeholder.

## Conventions for every endpoint

**Authentication:** none for `/api/public/skills`, otherwise `Authorization: Bearer <access_token>`.

**Request content type:** `application/json`

**Response content type:** `application/json; charset=utf-8` (the `204` responses have no body)

**Cache policy:** every skills response carries:

```http
Cache-Control: no-store
Pragma: no-cache
```

**Request body strictness:** bodies are capped at 64 KiB, must contain exactly one JSON object, and **unknown fields are rejected** (not ignored). An unexpected key returns `400 invalid_json`, so never send extra properties from a form model.

**ID format:** every `id` is a UUID v4 string. Treat it as an opaque key.

**Timestamps:** `created_at` and every other timestamp is an RFC 3339 UTC string, for example `2026-09-24T06:42:11Z`. Parse with `new Date(value)`; do not assume local time.

**No rate limiting** is applied to the skills endpoints.

## Authentication for admin calls

Admin calls need an access token issued by the auth module (`POST /api/auth/login` → `POST /api/auth/2fa/verify`). Send it on every admin request:

```http
Authorization: Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiJiN2Q0ZjZhMi05MWM4LTRlM2ItOGY1Ny0yYTZkMWM5ZTRiMzAiLCJpc3MiOiJwb3J0Zm9saW8tYXBpIiwiaWF0IjoxNzU4Njk2OTMxLCJleHAiOjE3NTg2OTc4MzF9.hT3sK9pQr2vNzX4mYbC1aD8eF6gH0jL2nP5rS7tU9wQ
```

Two distinct failures are possible:

| HTTP | `error.code` | Meaning | Frontend action |
|---:|---|---|---|
| `401` | `missing_token` | Header absent, or not in `Bearer <token>` form | Send the user to sign-in |
| `401` | `invalid_token` | Token malformed, expired, or signed with the wrong key | Try `POST /api/auth/refresh` once, then sign the user out |

Both `401` responses also include `WWW-Authenticate: Bearer realm="portfolio-api"`.

Browsers never send the `Authorization` header on a preflight, so `OPTIONS` requests are answered by CORS middleware with `204` before authentication runs:

```http
HTTP/1.1 204 No Content
Access-Control-Allow-Origin: http://localhost:5173
Access-Control-Allow-Methods: GET, POST, PUT, PATCH, DELETE, OPTIONS
Access-Control-Allow-Headers: Authorization, Content-Type, X-Requested-With
Access-Control-Max-Age: 600
```

`Access-Control-Allow-Origin` is echoed only when the calling origin is allow-listed (see Configuration-dependent behavior). The preflight itself still returns `204` for a blocked origin — the browser is the component that then refuses the real request.

## Response shape guarantees

Some fields are stable and worth relying on:

| Behaviour | Detail |
|---|---|
| `skills` array per category | Always present on the public response, `[]` for an empty category |
| `icon_slug` | Omitted from JSON when the skill has no icon |
| `display_order` | Always present (`0` when never set) |
| `created_at` | Present on admin responses only, never on the public response |
| `skills` in admin category list | Not included — use `GET /api/admin/skills` for a flat list |
| Ordering | Server-side; never re-sort client-side with a different comparator |
| `DELETE` success | `204` with an empty body — do not call `response.json()` |

## Response models

### Public — categories with nested skills

`GET /api/public/skills` returns one object with a single `categories` array:

```json
{
  "categories": [
    {
      "id": "b7d4f6a2-91c8-4e3b-8f57-2a6d1c9e4b30",
      "name": "Backend",
      "display_order": 1,
      "skills": [
        {
          "id": "5e8b2c47-0a91-4d6f-b3e8-7c2f9a4d1b60",
          "name": "Go",
          "icon_slug": "go",
          "display_order": 1
        },
        {
          "id": "d1f7a3b9-6c25-4a8e-9b04-3e7d5c1f8a29",
          "name": "PostgreSQL",
          "display_order": 2
        }
      ]
    },
    {
      "id": "c3a9e1f5-2d64-4b8a-9e07-5f1b8d2c6a94",
      "name": "Cloud",
      "display_order": 2,
      "skills": []
    }
  ]
}
```

| Field | Type | Description |
|---|---|---|
| `categories` | array | Ordered categories; `[]` when nothing has been configured |
| `categories[].id` | string | Category UUID |
| `categories[].name` | string | Display label, 1–100 characters |
| `categories[].display_order` | number | Integer ≥ 0; ties are broken by `name` |
| `categories[].skills` | array | Skills of this category, ordered by `display_order` then `name` |
| `categories[].skills[].id` | string | Skill UUID |
| `categories[].skills[].name` | string | Display label, 1–100 characters |
| `categories[].skills[].icon_slug` | string | Optional; absent when unset |
| `categories[].skills[].display_order` | number | Integer ≥ 0 |

An empty portfolio renders `{"categories": []}` with `200`, not `404`. Render a placeholder instead of an error state.

### Admin — categories (flat)

`GET /api/admin/skill-categories` returns a flat list without nested skills:

```json
{
  "categories": [
    {
      "id": "b7d4f6a2-91c8-4e3b-8f57-2a6d1c9e4b30",
      "name": "Backend",
      "display_order": 1,
      "created_at": "2026-09-24T06:42:11Z"
    },
    {
      "id": "c3a9e1f5-2d64-4b8a-9e07-5f1b8d2c6a94",
      "name": "Cloud",
      "display_order": 2,
      "created_at": "2026-09-24T06:44:02Z"
    }
  ]
}
```

### Admin — skills (flat)

`GET /api/admin/skills` returns every skill with its owning category, grouped by category order then skill order:

```json
{
  "skills": [
    {
      "id": "5e8b2c47-0a91-4d6f-b3e8-7c2f9a4d1b60",
      "name": "Go",
      "icon_slug": "go",
      "display_order": 1,
      "category_id": "b7d4f6a2-91c8-4e3b-8f57-2a6d1c9e4b30",
      "created_at": "2026-09-24T06:45:20Z"
    },
    {
      "id": "8a2d6f14-7b93-4c0e-a5d1-9f3b6e802c47",
      "name": "AWS",
      "display_order": 1,
      "category_id": "c3a9e1f5-2d64-4b8a-9e07-5f1b8d2c6a94",
      "created_at": "2026-09-24T06:46:38Z"
    }
  ]
}
```

| Field | Type | Description |
|---|---|---|
| `skills[].category_id` | string | UUID of the owning category; use it to group in the admin table |
| `skills[].created_at` | string | RFC 3339 UTC timestamp |

Empty results render as `{"categories": []}` and `{"skills": []}`.

## TypeScript types

```ts
export interface SkillItem {
  id: string;
  name: string;
  icon_slug?: string;
  display_order: number;
}

export interface SkillCategory {
  id: string;
  name: string;
  display_order: number;
  skills: SkillItem[];
}

export interface PublicSkillsResponse {
  categories: SkillCategory[];
}

export interface AdminSkillCategory {
  id: string;
  name: string;
  display_order: number;
  created_at: string;
}

export interface AdminSkill extends SkillItem {
  category_id: string;
  created_at: string;
}

export interface AdminSkillCategoriesResponse {
  categories: AdminSkillCategory[];
}

export interface AdminSkillsResponse {
  skills: AdminSkill[];
}

export interface CategoryPayload {
  name: string;
  display_order?: number | null;
}

export interface SkillPayload {
  category_id?: string; // required on create, optional on update
  name: string;
  icon_slug?: string;
  display_order?: number | null;
}

export interface ApiError {
  error: {
    code: string;
    message: string;
  };
}
```

`display_order` is optional in the payload types because omitting it is meaningful (see the next section). Leave it `undefined` rather than sending `0` when the admin did not touch the ordering input.

## Request payload rules

Both create and update bodies accept the same shapes; the rules differ slightly between create and update.

| Field | Type | Required | Rules |
|---|---|---:|---|
| `name` | string | Create: Yes / Update: Yes | Trimmed first; 1–100 characters (Unicode code points, not bytes); must not contain control characters; compared case-insensitively against existing rows in the same scope (category names globally, skill names inside the category) |
| `display_order` | integer or `null` | No | Must be ≥ 0. Omitted or `null` on update keeps the stored value. Omitted or `null` on create stores `0`. Values with a fractional part (for example `1.5`) are rejected as invalid JSON |
| `category_id` | string | Create: Yes / Update: No | Must reference an existing category, otherwise `404 skill_category_not_found`. Omitted on update keeps the current category; sending another category moves the skill |
| `icon_slug` | string | No | Trimmed; at most 64 characters; must not contain control characters; case and characters are preserved (icon providers such as Simple Icons use case-sensitive slugs). Sending `""` clears the stored icon |

Body-wide rules:

- JSON `null` for a string field behaves like an empty string, so `"name": null` fails with `400 validation_failed`.
- Sending an unknown property returns `400 invalid_json`; the API does not ignore extra keys.
- Two JSON objects in one body, or trailing content, returns `400 invalid_json`.
- A body larger than 64 KiB returns `400 body_too_large`.
- An empty body returns `400 empty_body`.
- Duplicate detection is case-insensitive, so `backend` collides with an existing `Backend`. The database also enforces uniqueness; if a collision slips past the pre-check, the API answers `409` instead of `500`.

## Error envelope

Every failed request — except router-level `404`/`405` described later — uses one envelope:

```json
{
  "error": {
    "code": "skill_category_exists",
    "message": "A skill category named \"Backend\" already exists."
  }
}
```

Branch on `error.code`. Use `error.message` for display only; the wording can change.

| HTTP | `error.code` | Cause |
|---:|---|---|
| `400` | `empty_body` | Request body was empty |
| `400` | `invalid_json` | Malformed JSON, multiple objects, or an unknown field |
| `400` | `body_too_large` | Body exceeds 64 KiB |
| `400` | `validation_failed` | Missing `name`/`category_id`, over-long text, control characters, or a negative `display_order` |
| `401` | `missing_token` | Admin request without a usable `Authorization` header |
| `401` | `invalid_token` | Access token malformed or expired |
| `404` | `skill_category_not_found` | Category ID does not exist |
| `404` | `skill_not_found` | Skill ID does not exist |
| `409` | `skill_category_exists` | Category name already taken (case-insensitive) |
| `409` | `skill_exists` | Skill name already used inside that category |
| `500` | `internal_error` | Unexpected server failure |

Example validation failure:

```http
HTTP/1.1 400 Bad Request
Content-Type: application/json; charset=utf-8
Cache-Control: no-store
```

```json
{
  "error": {
    "code": "validation_failed",
    "message": "name must be at most 100 characters."
  }
}
```

Example duplicate failure:

```http
HTTP/1.1 409 Conflict
Content-Type: application/json; charset=utf-8
Cache-Control: no-store
```

```json
{
  "error": {
    "code": "skill_exists",
    "message": "The skill \"Go\" already exists in this category."
  }
}
```

### Router-level errors are not JSON

If the path itself does not exist, the router answers with plain text and no JSON envelope:

```http
HTTP/1.1 404 Not Found
Content-Type: text/plain; charset=utf-8

404 page not found
```

If the path exists but the method is not registered on it (for example `POST /api/public/skills`), the router answers `405` with an `Allow` header and an **empty body**:

```http
HTTP/1.1 405 Method Not Allowed
Allow: GET

```

Guard your fetch wrapper so a failed response without a JSON body does not crash the UI.

## `GET /api/public/skills`

### Endpoint

```http
GET /api/public/skills
```

**Authentication:** None
**Request content type:** Not applicable (no body)
**Successful response:** `200 OK`
**Response content type:** `application/json; charset=utf-8`
**Cache policy:** `no-store`

### Request

No body and no query parameters. The endpoint always returns the complete, ordered set — do grouping client-side only for presentation.

Example request:

```bash
curl -i "[base-url]/api/public/skills"
```

### Flow

1. **Skills configured:** the response contains every category in `display_order` order, each with its skills nested in skill `display_order` order.
2. **Category has no skills:** it is still returned, with `"skills": []`. Decide in the UI whether to hide empty categories.
3. **Nothing configured yet:** the response is `{"categories": []}` with `200`. Show an empty/placeholder state, never an error state.

### Successful responses

```http
HTTP/1.1 200 OK
Content-Type: application/json; charset=utf-8
Cache-Control: no-store
```

```json
{
  "categories": [
    {
      "id": "b7d4f6a2-91c8-4e3b-8f57-2a6d1c9e4b30",
      "name": "Backend",
      "display_order": 1,
      "skills": [
        { "id": "5e8b2c47-0a91-4d6f-b3e8-7c2f9a4d1b60", "name": "Go", "icon_slug": "go", "display_order": 1 },
        { "id": "d1f7a3b9-6c25-4a8e-9b04-3e7d5c1f8a29", "name": "PostgreSQL", "display_order": 2 }
      ]
    },
    { "id": "c3a9e1f5-2d64-4b8a-9e07-5f1b8d2c6a94", "name": "Cloud", "display_order": 2, "skills": [] }
  ]
}
```

| Field | Type | Description |
|---|---|---|
| `categories` | array | Ordered category list |
| `categories[].skills` | array | Nested skills; `[]` when the category is empty |

### Error response

| HTTP | Error code | Cause |
|---:|---|---|
| `500` | `internal_error` | Unexpected server failure |

No `401` or `404` is possible here — the route is public and an empty portfolio is a success.

## `GET /api/admin/skill-categories`

### Endpoint

```http
GET /api/admin/skill-categories
```

**Authentication:** Bearer JWT (`Authorization: Bearer <access_token>`)
**Request content type:** Not applicable (no body)
**Successful response:** `200 OK`
**Response content type:** `application/json; charset=utf-8`
**Cache policy:** `no-store`

### Request

No body and no query parameters.

Example request:

```bash
curl -i "[base-url]/api/admin/skill-categories" \
  -H "Authorization: Bearer $ACCESS_TOKEN"
```

### Flow

1. **Authorized:** returns every category as a flat list in `display_order` order, each with `created_at`.
2. **Unauthorized:** `401 missing_token` or `401 invalid_token` — see the authentication section.
3. **Empty list:** `{"categories": []}` with `200`; render an empty admin table with a "create category" action.

Note that the admin list has **no nested `skills` array**. Pair it with `GET /api/admin/skills` and group by `category_id` when building the editor tree.

### Successful responses

```http
HTTP/1.1 200 OK
Content-Type: application/json; charset=utf-8
Cache-Control: no-store
```

```json
{
  "categories": [
    {
      "id": "b7d4f6a2-91c8-4e3b-8f57-2a6d1c9e4b30",
      "name": "Backend",
      "display_order": 1,
      "created_at": "2026-09-24T06:42:11Z"
    },
    {
      "id": "c3a9e1f5-2d64-4b8a-9e07-5f1b8d2c6a94",
      "name": "Cloud",
      "display_order": 2,
      "created_at": "2026-09-24T06:44:02Z"
    }
  ]
}
```

| Field | Type | Description |
|---|---|---|
| `categories[].id` | string | Category UUID; use it as the key when creating skills |
| `categories[].created_at` | string | RFC 3339 UTC timestamp |

### Error response

| HTTP | Error code | Cause |
|---:|---|---|
| `401` | `missing_token` | No usable `Authorization` header |
| `401` | `invalid_token` | Access token malformed or expired |
| `500` | `internal_error` | Unexpected server failure |

## `POST /api/admin/skill-categories`

### Endpoint

```http
POST /api/admin/skill-categories
```

**Authentication:** Bearer JWT
**Request content type:** `application/json`
**Successful response:** `201 Created`
**Response content type:** `application/json; charset=utf-8`
**Cache policy:** `no-store`

### Request

```json
{
  "name": "Cloud & DevOps",
  "display_order": 3
}
```

| Field | Type | Required | Description |
|---|---|---:|---|
| `name` | string | Yes | 1–100 characters after trimming; unique across categories (case-insensitive); no control characters |
| `display_order` | integer or `null` | No | ≥ 0; stored as `0` when omitted |

Example request:

```bash
curl -i -X POST "[base-url]/api/admin/skill-categories" \
  -H "Authorization: Bearer $ACCESS_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"name": "Cloud & DevOps", "display_order": 3}'
```

### Flow

1. **Accepted:** the category is created and the response body is the created object. Take `id` from the response, never from optimistic local state.
2. **Name already taken:** `409 skill_category_exists`, for example when `devops` collides with an existing `DevOps`. Show the message on the name input and keep the form open.
3. **Name missing or blank:** `400 validation_failed` with message `name is required.`
4. **Name over 100 characters or containing control characters:** `400 validation_failed`.
5. **Negative `display_order`:** `400 validation_failed` with message `display_order must not be negative.`
6. **Unknown JSON field or malformed body:** `400 invalid_json` or `400 empty_body`.

### Successful responses

```http
HTTP/1.1 201 Created
Content-Type: application/json; charset=utf-8
Cache-Control: no-store
```

```json
{
  "id": "f9b2d7c4-1e58-4a3f-9d62-8c4a7b1e5f30",
  "name": "Cloud & DevOps",
  "display_order": 3,
  "created_at": "2026-09-24T07:05:44Z"
}
```

| Field | Type | Description |
|---|---|---|
| `id` | string | Server-assigned UUID, needed for later updates and deletes |
| `name` | string | Trimmed value as stored |
| `display_order` | number | Stored order |
| `created_at` | string | RFC 3339 UTC timestamp |

### Error response

| HTTP | Error code | Cause |
|---:|---|---|
| `400` | `validation_failed` | `name` missing, blank, too long, or with control characters; or negative `display_order` |
| `400` | `invalid_json`, `empty_body`, `body_too_large` | Malformed, empty, or oversized body |
| `401` | `missing_token`, `invalid_token` | Access token missing or invalid |
| `409` | `skill_category_exists` | Name already used by another category |
| `500` | `internal_error` | Unexpected server failure |

## `PUT /api/admin/skill-categories/{id}`

### Endpoint

```http
PUT /api/admin/skill-categories/{id}
```

**Authentication:** Bearer JWT
**Request content type:** `application/json`
**Successful response:** `200 OK`
**Response content type:** `application/json; charset=utf-8`
**Cache policy:** `no-store`

### Request

```json
{
  "name": "Cloud & Platform",
  "display_order": 2
}
```

| Field | Type | Required | Description |
|---|---|---:|---|
| `name` | string | Yes | Same rules as create; the edited category itself is not treated as a collision, so changing only letter case is allowed |
| `display_order` | integer or `null` | No | ≥ 0; omit it (or send `null`) to keep the stored order, so a rename never silently resets a category to `0` |

Example request:

```bash
curl -i -X PUT "[base-url]/api/admin/skill-categories/b7d4f6a2-91c8-4e3b-8f57-2a6d1c9e4b30" \
  -H "Authorization: Bearer $ACCESS_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"name": "Cloud & Platform"}'
```

### Flow

1. **Accepted:** the category is renamed and/or reordered; the response is the updated category. `id` and `created_at` never change.
2. **Unknown id:** `404 skill_category_not_found`. Refresh the list — another session may have deleted it.
3. **Name taken by a different category:** `409 skill_category_exists`; the stored row is left untouched.
4. **`display_order` omitted:** the stored order is preserved. Send an explicit integer to reorder.

### Successful responses

```http
HTTP/1.1 200 OK
Content-Type: application/json; charset=utf-8
Cache-Control: no-store
```

```json
{
  "id": "b7d4f6a2-91c8-4e3b-8f57-2a6d1c9e4b30",
  "name": "Cloud & Platform",
  "display_order": 2,
  "created_at": "2026-09-24T06:42:11Z"
}
```

### Error response

| HTTP | Error code | Cause |
|---:|---|---|
| `400` | `validation_failed` | `name` invalid, or negative `display_order` |
| `400` | `invalid_json`, `empty_body`, `body_too_large` | Malformed, empty, or oversized body |
| `401` | `missing_token`, `invalid_token` | Access token missing or invalid |
| `404` | `skill_category_not_found` | Category id does not exist |
| `409` | `skill_category_exists` | Name already used by another category |
| `500` | `internal_error` | Unexpected server failure |

## `DELETE /api/admin/skill-categories/{id}`

### Endpoint

```http
DELETE /api/admin/skill-categories/{id}
```

**Authentication:** Bearer JWT
**Request content type:** Not applicable (no body)
**Successful response:** `204 No Content`
**Response content type:** none (empty body)
**Cache policy:** `no-store`

### Request

No body; the `{id}` path segment is the category UUID.

Example request:

```bash
curl -i -X DELETE "[base-url]/api/admin/skill-categories/c3a9e1f5-2d64-4b8a-9e07-5f1b8d2c6a94" \
  -H "Authorization: Bearer $ACCESS_TOKEN"
```

### Flow

1. **Accepted:** the category **and every skill inside it** is deleted in one atomic step. There is no partial state and no undo. The response is `204` with an empty body — do not call `response.json()`.
2. **Unknown id:** `404 skill_category_not_found`.
3. **Repeating the same delete:** `404 skill_category_not_found`; treat it as "already deleted".

Because skills are deleted with their category, remove those skills from local state after a successful delete instead of refetching and diffing.

### Successful responses

```http
HTTP/1.1 204 No Content
Cache-Control: no-store
```

No response body.

### Error response

| HTTP | Error code | Cause |
|---:|---|---|
| `401` | `missing_token`, `invalid_token` | Access token missing or invalid |
| `404` | `skill_category_not_found` | Category id does not exist |
| `500` | `internal_error` | Unexpected server failure |

## `GET /api/admin/skills`

### Endpoint

```http
GET /api/admin/skills
```

**Authentication:** Bearer JWT
**Request content type:** Not applicable (no body)
**Successful response:** `200 OK`
**Response content type:** `application/json; charset=utf-8`
**Cache policy:** `no-store`

### Request

No body and no query parameters — there is no server-side filter by category. Fetch once and group by `category_id` in the client, or filter locally.

Example request:

```bash
curl -i "[base-url]/api/admin/skills" \
  -H "Authorization: Bearer $ACCESS_TOKEN"
```

### Flow

1. **Authorized:** returns every skill as a flat list, ordered by the owning category's `display_order`, then the category `name`, then the skill's `display_order`, then the skill `name`.
2. **Empty list:** `{"skills": []}` with `200`.
3. **Grouping for the editor:** join on `category_id` against `GET /api/admin/skill-categories`. Both lists are ordered, so preserving response order keeps the UI consistent with the public site.

### Successful responses

```http
HTTP/1.1 200 OK
Content-Type: application/json; charset=utf-8
Cache-Control: no-store
```

```json
{
  "skills": [
    {
      "id": "5e8b2c47-0a91-4d6f-b3e8-7c2f9a4d1b60",
      "name": "Go",
      "icon_slug": "go",
      "display_order": 1,
      "category_id": "b7d4f6a2-91c8-4e3b-8f57-2a6d1c9e4b30",
      "created_at": "2026-09-24T06:45:20Z"
    },
    {
      "id": "d1f7a3b9-6c25-4a8e-9b04-3e7d5c1f8a29",
      "name": "PostgreSQL",
      "display_order": 2,
      "category_id": "b7d4f6a2-91c8-4e3b-8f57-2a6d1c9e4b30",
      "created_at": "2026-09-24T06:46:03Z"
    },
    {
      "id": "8a2d6f14-7b93-4c0e-a5d1-9f3b6e802c47",
      "name": "AWS",
      "icon_slug": "amazonaws",
      "display_order": 1,
      "category_id": "c3a9e1f5-2d64-4b8a-9e07-5f1b8d2c6a94",
      "created_at": "2026-09-24T06:47:38Z"
    }
  ]
}
```

| Field | Type | Description |
|---|---|---|
| `skills[].id` | string | Skill UUID |
| `skills[].name` | string | Display label |
| `skills[].icon_slug` | string | Optional; absent when unset |
| `skills[].display_order` | number | Order inside its category |
| `skills[].category_id` | string | Owning category UUID |
| `skills[].created_at` | string | RFC 3339 UTC timestamp |

### Error response

| HTTP | Error code | Cause |
|---:|---|---|
| `401` | `missing_token`, `invalid_token` | Access token missing or invalid |
| `500` | `internal_error` | Unexpected server failure |

## `POST /api/admin/skills`

### Endpoint

```http
POST /api/admin/skills
```

**Authentication:** Bearer JWT
**Request content type:** `application/json`
**Successful response:** `201 Created`
**Response content type:** `application/json; charset=utf-8`
**Cache policy:** `no-store`

### Request

```json
{
  "category_id": "c3a9e1f5-2d64-4b8a-9e07-5f1b8d2c6a94",
  "name": "Terraform",
  "icon_slug": "terraform",
  "display_order": 2
}
```

| Field | Type | Required | Description |
|---|---|---:|---|
| `category_id` | string | Yes | Must be an existing category, otherwise `404 skill_category_not_found` |
| `name` | string | Yes | 1–100 characters after trimming; unique inside the category (case-insensitive); no control characters |
| `icon_slug` | string | No | ≤ 64 characters; stored as-is (case preserved); omit or send `""` for no icon |
| `display_order` | integer or `null` | No | ≥ 0; stored as `0` when omitted |

The same `name` is allowed in two different categories — uniqueness is scoped to the category.

Example request:

```bash
curl -i -X POST "[base-url]/api/admin/skills" \
  -H "Authorization: Bearer $ACCESS_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"category_id": "c3a9e1f5-2d64-4b8a-9e07-5f1b8d2c6a94", "name": "Terraform", "icon_slug": "terraform", "display_order": 2}'
```

### Flow

1. **Accepted:** the skill is created and returned. Use the returned `id` for later updates and deletes.
2. **Category does not exist:** `404 skill_category_not_found`, and nothing is written. Refresh the category list and re-select.
3. **Name already used in that category:** `409 skill_exists`. Adding the same name under another category succeeds.
4. **Missing `name` or `category_id`:** `400 validation_failed`.
5. **Invalid `icon_slug` or `display_order`:** `400 validation_failed`.

### Successful responses

```http
HTTP/1.1 201 Created
Content-Type: application/json; charset=utf-8
Cache-Control: no-store
```

```json
{
  "id": "3c7e9a12-4b60-4d8f-a2c5-6e1b8f3d9a47",
  "name": "Terraform",
  "icon_slug": "terraform",
  "display_order": 2,
  "category_id": "c3a9e1f5-2d64-4b8a-9e07-5f1b8d2c6a94",
  "created_at": "2026-09-24T07:12:09Z"
}
```

| Field | Type | Description |
|---|---|---|
| `id` | string | Server-assigned skill UUID |
| `name` | string | Trimmed value as stored |
| `icon_slug` | string | Omitted when empty |
| `display_order` | number | Stored order |
| `category_id` | string | Category the skill was filed under |
| `created_at` | string | RFC 3339 UTC timestamp |

### Error response

| HTTP | Error code | Cause |
|---:|---|---|
| `400` | `validation_failed` | Missing `category_id`/`name`, over-long text, control characters, or negative `display_order` |
| `400` | `invalid_json`, `empty_body`, `body_too_large` | Malformed, empty, or oversized body |
| `401` | `missing_token`, `invalid_token` | Access token missing or invalid |
| `404` | `skill_category_not_found` | `category_id` does not exist |
| `409` | `skill_exists` | Name already used inside that category |
| `500` | `internal_error` | Unexpected server failure |

## `PUT /api/admin/skills/{id}`

### Endpoint

```http
PUT /api/admin/skills/{id}
```

**Authentication:** Bearer JWT
**Request content type:** `application/json`
**Successful response:** `200 OK`
**Response content type:** `application/json; charset=utf-8`
**Cache policy:** `no-store`

### Request

Rename, re-icon and reorder — and optionally move the skill to another category:

```json
{
  "category_id": "b7d4f6a2-91c8-4e3b-8f57-2a6d1c9e4b30",
  "name": "Terraform",
  "icon_slug": "terraform",
  "display_order": 3
}
```

| Field | Type | Required | Description |
|---|---|---:|---|
| `category_id` | string | No | Omit (or send `""`) to keep the current category; send another UUID to move the skill. The target must exist |
| `name` | string | Yes | Same rules as create; the skill itself is not treated as a collision, so a pure case change is allowed |
| `icon_slug` | string | No | ≤ 64 characters; `""` clears the stored icon |
| `display_order` | integer or `null` | No | ≥ 0; omit to keep the stored order |

Example request:

```bash
curl -i -X PUT "[base-url]/api/admin/skills/3c7e9a12-4b60-4d8f-a2c5-6e1b8f3d9a47" \
  -H "Authorization: Bearer $ACCESS_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"name": "Terraform", "display_order": 3}'
```

### Flow

1. **Accepted:** the skill is updated in place; `id` and `created_at` never change.
2. **Unknown skill id:** `404 skill_not_found`.
3. **Target category missing:** `404 skill_category_not_found` when `category_id` does not exist. The skill keeps its current category.
4. **Name taken in the target category:** `409 skill_exists`. Moving a skill into a category that already holds that name is rejected and the skill stays where it was.
5. **`icon_slug` empty:** clears the icon, and the `icon_slug` key disappears from the response.

### Successful responses

```http
HTTP/1.1 200 OK
Content-Type: application/json; charset=utf-8
Cache-Control: no-store
```

```json
{
  "id": "3c7e9a12-4b60-4d8f-a2c5-6e1b8f3d9a47",
  "name": "Terraform",
  "icon_slug": "terraform",
  "display_order": 3,
  "category_id": "b7d4f6a2-91c8-4e3b-8f57-2a6d1c9e4b30",
  "created_at": "2026-09-24T07:12:09Z"
}
```

### Error response

| HTTP | Error code | Cause |
|---:|---|---|
| `400` | `validation_failed` | `name` invalid, over-long `icon_slug`, control characters, or negative `display_order` |
| `400` | `invalid_json`, `empty_body`, `body_too_large` | Malformed, empty, or oversized body |
| `401` | `missing_token`, `invalid_token` | Access token missing or invalid |
| `404` | `skill_not_found` | Skill id does not exist |
| `404` | `skill_category_not_found` | Target `category_id` does not exist |
| `409` | `skill_exists` | Name already used in the target category |
| `500` | `internal_error` | Unexpected server failure |

## `DELETE /api/admin/skills/{id}`

### Endpoint

```http
DELETE /api/admin/skills/{id}
```

**Authentication:** Bearer JWT
**Request content type:** Not applicable (no body)
**Successful response:** `204 No Content`
**Response content type:** none (empty body)
**Cache policy:** `no-store`

### Request

No body; the `{id}` path segment is the skill UUID.

Example request:

```bash
curl -i -X DELETE "[base-url]/api/admin/skills/3c7e9a12-4b60-4d8f-a2c5-6e1b8f3d9a47" \
  -H "Authorization: Bearer $ACCESS_TOKEN"
```

### Flow

1. **Accepted:** that single skill is removed; its category stays. The response is `204` with an empty body — do not call `response.json()`.
2. **Unknown id:** `404 skill_not_found`.
3. **Repeating the same delete:** `404 skill_not_found`; treat it as "already deleted".
4. **Deleting a skill does not touch the category**, and deleting a category does not need a separate skill delete (see `DELETE /api/admin/skill-categories/{id}`).

### Successful responses

```http
HTTP/1.1 204 No Content
Cache-Control: no-store
```

No response body.

### Error response

| HTTP | Error code | Cause |
|---:|---|---|
| `401` | `missing_token`, `invalid_token` | Access token missing or invalid |
| `404` | `skill_not_found` | Skill id does not exist |
| `500` | `internal_error` | Unexpected server failure |

## Configuration-dependent behavior

| Variable | Default | Effect |
|---|---:|---|
| `AUTH_ALLOWED_ORIGINS` | `http://localhost:5173,http://localhost:3000` | CORS origin allow-list. A listed origin gets `Access-Control-Allow-Origin` + `Access-Control-Allow-Credentials: true`. `*` allows every origin but drops the credentials header |
| `SERVER_PORT` (or Lambda's `PORT`) | `8080` | Port the API listens on locally, so it decides your local base URL |
| `JWT_ACCESS_TTL_MINUTES` | `15` | Access-token lifetime, which determines how often admin screens can hit `401 invalid_token` and need a refresh |

These limits are **not** configurable at runtime — they are constants of the skills module:

| Limit | Value | Effect |
|---|---:|---|
| `name` length | 100 characters | Longer labels fail with `400 validation_failed` |
| `icon_slug` length | 64 characters | Longer slugs fail with `400 validation_failed` |
| Request body size | 64 KiB | Larger bodies fail with `400 body_too_large` |

No rate limiting applies to any skills endpoint, unlike the auth endpoints. `display_order` has no upper bound, so leave headroom (100, 200, 300 …) when the UI assigns positions, and remember that equal values fall back to alphabetical ordering by `name`.

## Client handling

### Public site

1. Call `GET /api/public/skills` on page load (or at build time for a static export). No token needed.
2. Render `categories` in response order; do not re-sort. Treat `icon_slug` as optional and fall back to a text chip.
3. On `500 internal_error`, retry once after a short delay, then render a non-blocking placeholder — the rest of the page must still render.
4. Because responses are `no-store`, add your own short-lived cache (for example a few minutes in memory or at the CDN) if the page is visited often.

### Admin panel

1. Ensure a valid access token before the first admin call; admin screens are useless without one.
2. Load `GET /api/admin/skill-categories` and `GET /api/admin/skills` together, then group skills by `category_id`. Show categories with no skills as empty groups so the admin can add one.
3. On any write, take `id`, `name`, `icon_slug`, `display_order` and `created_at` **from the response**, not from the submitted form: the server trims whitespace, stores `0` for omitted orders, and assigns IDs.
4. On `401 invalid_token`, call `POST /api/auth/refresh` once and replay the request; on `401 missing_token`, send the user to sign-in. Never retry a `401` more than once.
5. Map failures in the UI by code:
   - `400 validation_failed` → show `error.message` on the offending field.
   - `404 skill_category_not_found` / `404 skill_not_found` → refetch the lists and tell the user the row is gone.
   - `409 skill_category_exists` / `409 skill_exists` → keep the form open and highlight the name field; nothing was saved.
   - `500 internal_error` → retry once; if it fails again, show a generic error.
6. Never retry `POST` automatically without a guard: a repeated create does not duplicate data (the second attempt fails with `409`), but it does surface a confusing error. Submit-once with a disabled button is the simplest correct approach.
7. After deleting a category, drop its skills from local state too, because they were deleted with it in the same operation.

### Not provided by this module

There is no bulk reorder endpoint, no pagination, no search, and no server-side filter by category. Reordering means one `PUT` per moved row with an explicit `display_order`.


