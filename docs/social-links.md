# Social Links Module — Frontend Guide

The Social Links module powers the social icons in the site header or footer, plus their editor in the admin panel.

- The **public** endpoint is open and returns the links in display order.
- The **admin** endpoint is a **single transactional replace** — there is no per-link create or delete. You always send the complete set.
- The table stores only `id`, `platform`, `url` and `display_order`. There is **no `created_at` and no `updated_at`**, so this module exposes no timestamps at all.

## Endpoint map

| Method | Endpoint | Access token required | Success |
|---|---|---:|---:|
| `GET` | `/api/public/social-links` | No | `200` |
| `PUT` | `/api/admin/social-links` | Yes | `200` |

## The replace semantics — read this first

`PUT /api/admin/social-links` **replaces the entire stored set** in one transaction. The `links` array you send becomes the complete new state:

- a platform present in the payload is inserted (with a freshly generated `id`)
- a platform **absent** from the payload is **deleted**
- the whole swap is atomic, so a visitor never sees a half-written set

**Always load the current list, edit it, and send the whole thing back.** Never send a single link expecting a partial update.

**An empty `links` array is rejected** with `400 validation_failed` and the message `links must contain at least one entry.` This is deliberate: the write deletes every row before inserting, so accepting an empty list would let a buggy panel wipe the whole section. If you genuinely need to clear it, that requires a schema-level decision — raise it rather than working around the guard.

## Conventions

**Authentication:** none for `/api/public/social-links`, otherwise `Authorization: Bearer <access_token>`.

**Request content type:** `application/json`

**Response content type:** `application/json; charset=utf-8`

**Cache policy:** every response carries `Cache-Control: no-store` and `Pragma: no-cache`.

**Request body strictness:** capped at 64 KiB, exactly one JSON object, **unknown fields are rejected** (`400 invalid_json`).

## Field rules and limits

| Field | Required | Rules |
|---|---:|---|
| `platform` | Yes | one of `linkedin`, `github`, `twitter`, `medium` — input is case-insensitive and trimmed, stored and returned **lowercase** |
| `url` | Yes | absolute `http`/`https` URL, max 500 chars, **no whitespace or control characters anywhere** |
| `display_order` | No | non-negative int; **defaults to the array position**, so sending the list in the order you want is enough |

**At most 4 entries.** The `links` array is capped at the number of accepted platforms (4), and the cap is checked before any per-entry validation runs. Extra entries are rejected even if their `platform` values would be valid.

**Platform values are lowercase here.** This differs from the projects and extras modules, where enums are stored uppercase. The schema lists these four in lowercase, so they stay lowercase. Compare against lowercase literals, or normalise with `platform.toLowerCase()`.

**The same platform may appear only once per payload.** A repeat is rejected with `409` and `code: "social_link_conflict"`, naming the duplicated platform. Note that `github` and `GitHub` count as the same platform, because the value is lower-cased before the duplicate check.

**Do not put spaces in a URL.** A space anywhere inside a URL — including in the host — is rejected with `400` and `url must not contain whitespace.` Only leading and trailing whitespace is trimmed for you; anything in the middle is an error. Without this rule a link like `https://github.com/kam lesh` would be stored and then percent-encoded by the browser into a dead link. Encode user-facing spaces as `%20` if a path genuinely needs one.

**Ordering:** `display_order ASC, id ASC`.

**Timestamps:** none. Do not expect `created_at` or `updated_at` in any response.

---

## 1. Public list

### `GET /api/public/social-links`

```json
{
  "social_links": [
    {
      "id": "8b2e3f44-9a66-4b2d-8c07-1d2e3f4a5b60",
      "platform": "linkedin",
      "url": "https://linkedin.com/in/kamlesh",
      "display_order": 0
    },
    {
      "id": "9c3f4a55-0b77-4c3e-9d18-2e3f4a5b6c71",
      "platform": "github",
      "url": "https://github.com/kamlesh",
      "display_order": 1
    }
  ]
}
```

With nothing configured, `social_links` is an empty array — never `null`.

## 2. Replace the set

### `PUT /api/admin/social-links`

```json
{
  "links": [
    { "platform": "linkedin", "url": "https://linkedin.com/in/kamlesh" },
    { "platform": "github", "url": "https://github.com/kamlesh" },
    { "platform": "medium", "url": "https://medium.com/@kamlesh", "display_order": 5 }
  ]
}
```

Returns `200 OK` with the server's view of the new set, so refresh your local state from the response rather than trusting the request:

```json
{
  "social_links": [
    {
      "id": "a1b2c3d4-1111-4222-8333-444455556666",
      "platform": "linkedin",
      "url": "https://linkedin.com/in/kamlesh",
      "display_order": 0
    }
  ]
}
```

### Error responses

| Situation | Status | `code` | Message |
|---|---:|---|---|
| `links` missing or empty | `400` | `validation_failed` | `links must contain at least one entry.` |
| more than 4 entries | `400` | `validation_failed` | `links must contain at most 4 entries.` |
| unknown or missing `platform` | `400` | `validation_failed` | `platform must be one of linkedin, github, twitter, medium.` |
| malformed `url` | `400` | `validation_failed` | names the problem, e.g. `url must be an http or https URL.` |
| whitespace inside `url` | `400` | `validation_failed` | `url must not contain whitespace.` |
| negative `display_order` | `400` | `validation_failed` | `display_order must not be negative.` |
| duplicate platform in one payload | `409` | `social_link_conflict` | `A social link for "github" already exists in this payload.` |
| unknown JSON key | `400` | `invalid_json` | names the offending field |
