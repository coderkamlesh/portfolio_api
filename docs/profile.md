# Profile Module — Frontend Guide

The Profile module exposes the singleton portfolio profile. The public site reads it without authentication; the admin panel reads and fully replaces it with an access token.

## Endpoint map

| Method | Endpoint | Access token required | Success |
|---|---|---:|---:|
| `GET` | `/api/public/profile` | No | `200` |
| `GET` | `/api/admin/profile` | Yes | `200` |
| `PUT` | `/api/admin/profile` | Yes | `200` |

All profile responses include `Cache-Control: no-store`. Admin requests use:

```http
Authorization: Bearer <access_token>
```

## Response model

Public response fields are top-level; there is no nested `profile` object:

```json
{
  "full_name": "Kamlesh Kumar",
  "title": "Senior Software Engineer",
  "tagline": "Backend and distributed systems",
  "summary": "I build reliable APIs and web products.",
  "email": "kamlesh@example.com",
  "phone": "+91 9999999999",
  "location": "India",
  "avatar_url": "https://cdn.example.com/avatar.jpg",
  "linkedin_url": "https://www.linkedin.com/in/example",
  "github_url": "https://github.com/example",
  "portfolio_url": "https://example.com",
  "twitter_url": "https://x.com/example",
  "resume_file_url": "https://cdn.example.com/resume.pdf",
  "career_gap_note": "Career break for family responsibilities.",
  "experience_level": "senior"
}
```

Optional fields with empty values are omitted from JSON responses. Treat every field except `full_name`, `title`, and `email` as optional in frontend types.

The admin response contains the same public fields plus metadata:

```json
{
  "full_name": "Kamlesh Kumar",
  "title": "Senior Software Engineer",
  "email": "kamlesh@example.com",
  "id": "01J...",
  "updated_at": "2026-09-24T12:30:00Z"
}
```

`updated_at` is an RFC 3339 UTC timestamp. The profile ID remains stable after the first save.

## TypeScript types

```ts
export interface PublicProfile {
  full_name: string;
  title: string;
  tagline?: string;
  summary?: string;
  email: string;
  phone?: string;
  location?: string;
  avatar_url?: string;
  linkedin_url?: string;
  github_url?: string;
  portfolio_url?: string;
  twitter_url?: string;
  resume_file_url?: string;
  career_gap_note?: string;
  experience_level?: string;
}

export interface AdminProfile extends PublicProfile {
  id: string;
  updated_at: string;
}

export interface UpdateProfileRequest {
  full_name: string;
  title: string;
  tagline: string;
  summary: string;
  email: string;
  phone: string;
  location: string;
  avatar_url: string;
  linkedin_url: string;
  github_url: string;
  portfolio_url: string;
  twitter_url: string;
  resume_file_url: string;
  career_gap_note: string;
  experience_level: string;
}
```

## Public profile

```http
GET /api/public/profile
```

Use this for the public portfolio hero/contact data. If the profile has not been configured yet, the API returns `404 profile_not_found`; render a safe public fallback instead of a technical error.

## Admin profile read

```http
GET /api/admin/profile
Authorization: Bearer <access_token>
```

Use this to populate the admin form. A `404 profile_not_found` means first-time setup: render an empty form and save it with `PUT`.

## Create or replace profile

```http
PUT /api/admin/profile
Authorization: Bearer <access_token>
Content-Type: application/json
```

This is a full replacement, not a patch. Send every editable field. Omitted string fields deserialize as empty strings, and empty optional fields clear their stored values.

Required fields:

| Field | Rule |
|---|---|
| `full_name` | Required after trimming whitespace |
| `title` | Required after trimming whitespace |
| `email` | Required, trimmed, lowercased, and syntax-validated |

Optional fields are trimmed before saving. Email validation checks address syntax only; it does not verify DNS or mailbox existence. URL fields and `experience_level` are currently free-form strings.

Example:

```ts
async function saveProfile(
  accessToken: string,
  body: UpdateProfileRequest,
): Promise<AdminProfile> {
  const response = await fetch(`${API_BASE_URL}/api/admin/profile`, {
    method: "PUT",
    headers: {
      Authorization: `Bearer ${accessToken}`,
      "Content-Type": "application/json",
    },
    body: JSON.stringify(body),
  });

  if (!response.ok) {
    throw await response.json();
  }
  return response.json();
}
```

On success, replace admin form state with the returned `AdminProfile`; do not reuse stale local field values.

## Error handling

Errors use the shared envelope:

```json
{
  "error": {
    "code": "validation_failed",
    "message": "full_name is required."
  }
}
```

| HTTP | `error.code` | Meaning |
|---:|---|---|
| `400` | `empty_body` | JSON body is required |
| `400` | `invalid_json` | Body is invalid, has multiple objects, or contains an unknown field |
| `400` | `body_too_large` | Body exceeds the request limit |
| `400` | `validation_failed` | Required field missing or email syntax invalid |
| `401` | `missing_token`, `invalid_token` | Admin access token is absent or invalid |
| `404` | `profile_not_found` | Profile has not been created yet |
| `500` | `internal_error` | Unexpected server failure |

Request bodies are limited to 64 KiB and unknown JSON fields are rejected. Use `error.code` for UI behavior and `error.message` only for display.

## Admin form flow

1. Load `GET /api/admin/profile` after auth initialization.
2. On `200`, merge the response into a controlled form, converting absent optional fields to `""` for inputs.
3. On `404 profile_not_found`, show the same form in first-time setup mode.
4. On save, submit the complete form object through `PUT`.
5. On success, update local state from the response and show the new `updated_at` value.

There is no optimistic-locking version or ETag. If two admin sessions save at the same time, the last successful write wins.
