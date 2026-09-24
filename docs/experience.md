# Experience Module — Frontend Guide

The Experience module powers the work experience timeline of the public portfolio and the work experience manager inside the admin panel.

- The **public** endpoint is open and returns work experiences in reverse chronological order with nested bullet points and parsed technology arrays.
- The **admin** endpoints provide full CRUD operations protected by the access token.
- Experience updates and deletions are atomic: updating an experience replaces its bullet points cleanly, and deleting an experience cascades to all its bullets.

## Endpoint map

| Method | Endpoint | Access token required | Success |
|---|---|---:|---:|
| `GET` | `/api/public/experience` | No | `200` |
| `GET` | `/api/admin/experience` | Yes | `200` |
| `POST` | `/api/admin/experience` | Yes | `201` |
| `PUT` | `/api/admin/experience/{id}` | Yes | `200` |
| `DELETE` | `/api/admin/experience/{id}` | Yes | `204` |

Base URL locally is `http://localhost:8080`; in production it is the deployed Lambda endpoint. All examples below use `[base-url]` as a placeholder.

## Conventions for every endpoint

**Authentication:** none for `/api/public/experience`, otherwise `Authorization: Bearer <access_token>`.

**Request content type:** `application/json`

**Response content type:** `application/json; charset=utf-8` (`DELETE` returns `204 No Content` with no body).

**Cache policy:** every experience response carries:

```http
Cache-Control: no-store
Pragma: no-cache
```

**Request body strictness:** bodies are capped at 64 KiB, must contain exactly one JSON object, and **unknown fields are rejected** (not ignored). Extra keys return `400 invalid_json`.

**Date formats:** `start_date` and `end_date` must follow strict `YYYY-MM-DD` format (e.g. `2024-04-01`).

**Rules:**
- `is_current: true` implies `end_date` must be empty or omitted.
- When `end_date` is supplied, `end_date >= start_date`.
- `employment_type` accepts: `FULL_TIME`, `PART_TIME`, `CONTRACT`, `INTERNSHIP`, `FREELANCE` (case-insensitive in input, stored and returned uppercase), or empty/omitted string.
- `technologies`: array of strings (max 20 entries, max 50 chars each).
- `bullets`: array of strings (max 10 entries, max 500 chars each).

- `bullets`: array of strings (max 10 entries, max 500 chars each).

---

## 1. Public Work Experience

### `GET /api/public/experience`

Fetches all work experiences sorted reverse chronologically (`is_current DESC, start_date DESC, display_order ASC, company_name ASC`).

#### Response (`200 OK`)

```json
{
  "experiences": [
    {
      "id": "e4b2d354-9411-482a-a827-023bbbeee9c8",
      "company_name": "Nimbus Labs",
      "company_logo_url": "https://cdn.example.com/nimbus.png",
      "role": "Senior Backend Engineer",
      "employment_type": "FULL_TIME",
      "location": "Noida, India",
      "start_date": "2024-04-01",
      "end_date": "",
      "is_current": true,
      "technologies": ["Go", "PostgreSQL", "Docker"],
      "bullets": [
        "Cut p99 order API latency by 42% by replacing N+1 queries with batched reads."
      ],
      "display_order": 0
    }
  ]
}
```

---

## 2. Admin List Experiences

### `GET /api/admin/experience`

Lists all experiences including audit timestamps (`created_at`, `updated_at`).

#### Response (`200 OK`)

```json
{
  "experiences": [
    {
      "id": "e4b2d354-9411-482a-a827-023bbbeee9c8",
      "company_name": "Nimbus Labs",
      "company_logo_url": "https://cdn.example.com/nimbus.png",
      "role": "Senior Backend Engineer",
      "employment_type": "FULL_TIME",
      "location": "Noida, India",
      "start_date": "2024-04-01",
      "end_date": "",
      "is_current": true,
      "technologies": ["Go", "PostgreSQL"],
      "bullets": [
        "Cut p99 order API latency by 42% by replacing N+1 queries."
      ],
      "display_order": 0,
      "created_at": "2026-09-24T12:00:00Z",
      "updated_at": "2026-09-24T12:00:00Z"
    }
  ]
}
```

---

## 3. Create Experience

### `POST /api/admin/experience`

#### Request Body

```json
{
  "company_name": "Nimbus Labs",
  "company_logo_url": "https://cdn.example.com/nimbus.png",
  "role": "Senior Backend Engineer",
  "employment_type": "full_time",
  "location": "Noida, India",
  "start_date": "2024-04-01",
  "end_date": "",
  "is_current": true,
  "technologies": ["Go", "PostgreSQL"],
  "bullets": [
    "Cut p99 order API latency by 42% by replacing N+1 queries with batched reads."
  ],
  "display_order": 0
}
```

Returns `201 Created` with the created `AdminExperienceView`.

---

## 4. Update Experience

### `PUT /api/admin/experience/{id}`

Updates an experience and atomically replaces all bullets.

Returns `200 OK` with the updated `AdminExperienceView`.

---

## 5. Delete Experience

### `DELETE /api/admin/experience/{id}`

Deletes an experience and cascades to all its bullets.

Returns `204 No Content`.
