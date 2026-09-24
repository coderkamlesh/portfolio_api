# Projects Module — Frontend Guide

The Projects module powers the project showcase of the public portfolio and the project manager inside the admin panel.

- The **public** endpoints are open: a list ordered featured-first, and a single-project detail.
- The **admin** endpoints provide full CRUD operations protected by the access token.
- Every write that touches a project and its bullets is atomic: an update replaces the bullets wholesale, a delete removes them with the project.

## Endpoint map

| Method | Endpoint | Access token required | Success |
|---|---|---:|---:|
| `GET` | `/api/public/projects` | No | `200` |
| `GET` | `/api/public/projects/{id}` | No | `200` |
| `GET` | `/api/admin/projects` | Yes | `200` |
| `POST` | `/api/admin/projects` | Yes | `201` |
| `PUT` | `/api/admin/projects/{id}` | Yes | `200` |
| `DELETE` | `/api/admin/projects/{id}` | Yes | `204` |

## Conventions

**Authentication:** none for `/api/public/*`, otherwise `Authorization: Bearer <access_token>`.

**Request content type:** `application/json`

**Response content type:** `application/json; charset=utf-8` (`DELETE` returns `204 No Content`).

**Cache policy:** every projects response carries `Cache-Control: no-store` and `Pragma: no-cache`.

**Request body strictness:** capped at 64 KiB, exactly one JSON object, **unknown fields are rejected** (`400 invalid_json`).

**ID format:** every `id` is a UUID v4 string. Treat it as opaque.

**Timestamps:** RFC 3339 UTC strings, e.g. `2026-09-24T12:00:00Z`. Admin responses only.

## Field rules and limits

| Field | Required | Rules |
|---|---:|---|
| `title` | Yes | max 150 chars, no control characters |
| `description` | Yes | max 4000 chars, no control characters |
| `tagline` | No | max 200 chars |
| `role` | No | max 150 chars |
| `project_type` | No | `PERSONAL`, `ACADEMIC`, `OPEN_SOURCE`, `INTERNSHIP` — case-insensitive input, stored uppercase, empty allowed |
| `status` | No | `COMPLETED`, `IN_PROGRESS` — case-insensitive input, stored uppercase, empty allowed |
| `technologies` | No | max 30 entries, max 50 chars each, blank entries dropped, always returned as `[]` |
| `repo_url`, `live_url`, `image_url` | No | absolute `http`/`https` URL, max 500 chars |
| `start_date`, `end_date` | No | strict `YYYY-MM-DD` |
| `is_featured` | No | boolean, defaults to `false` |
| `bullets` | No | max 20 entries, max 300 chars each, blank entries dropped, always returned as `[]` |
| `display_order` | No | non-negative int; omit on update to keep the stored order, defaults to `0` on create |

**Cross-field rules:**
- `end_date` set → `start_date` must also be set.
- `end_date` must not be earlier than `start_date`.
- `status = IN_PROGRESS` → `end_date` must be empty.

Any rule violation returns `400` with `code: "validation_failed"` and a message naming the offending field. A missing project returns `404` with `code: "project_not_found"`.

**Ordering:** public and admin listings use `is_featured DESC, display_order ASC, id ASC`. Featured projects come first; within a group the lower `display_order` wins.

---

## 1. Public project list

### `GET /api/public/projects`

```json
{
  "projects": [
    {
      "id": "9c1f0d1e-3b2a-4c5d-8e9f-0a1b2c3d4e5f",
      "title": "AlgoMaster",
      "tagline": "Practice platform for DSA interview preparation.",
      "description": "Built the backend that serves 40k monthly practice sessions.",
      "project_type": "PERSONAL",
      "role": "Backend Developer",
      "technologies": ["Go", "PostgreSQL"],
      "repo_url": "https://github.com/kamlesh/algomaster",
      "live_url": "https://algomaster.example.com",
      "image_url": "https://cdn.example.com/algomaster.png",
      "start_date": "2024-04-01",
      "end_date": "2025-12-31",
      "status": "COMPLETED",
      "is_featured": true,
      "bullets": [
        "Cut p95 submission latency by 38% with a Redis-backed result cache."
      ],
      "display_order": 0
    }
  ]
}
```

## 2. Public project detail

### `GET /api/public/projects/{id}`

Same project object, wrapped one level deeper:

```json
{
  "project": {
    "id": "9c1f0d1e-3b2a-4c5d-8e9f-0a1b2c3d4e5f",
    "title": "AlgoMaster",
    "description": "Built the backend that serves 40k monthly practice sessions.",
    "technologies": ["Go", "PostgreSQL"],
    "bullets": [
      "Cut p95 submission latency by 38% with a Redis-backed result cache."
    ]
  }
}
```

## 3. Admin list

### `GET /api/admin/projects`

Same array as the public list, with `created_at` and `updated_at` added to every entry.

## 4. Create

### `POST /api/admin/projects`

```json
{
  "title": "AlgoMaster",
  "tagline": "Practice platform for DSA interview preparation.",
  "description": "Built the backend that serves 40k monthly practice sessions.",
  "project_type": "personal",
  "role": "Backend Developer",
  "technologies": ["Go", "PostgreSQL"],
  "repo_url": "https://github.com/kamlesh/algomaster",
  "live_url": "https://algomaster.example.com",
  "image_url": "https://cdn.example.com/algomaster.png",
  "start_date": "2024-04-01",
  "end_date": "2025-12-31",
  "status": "completed",
  "is_featured": true,
  "bullets": [
    "Cut p95 submission latency by 38% with a Redis-backed result cache.",
    "Designed the judge queue that isolates test execution per request."
  ],
  "display_order": 0
}
```

Returns `201 Created` with the created project including its `id`, `created_at` and `updated_at`.

## 5. Update

### `PUT /api/admin/projects/{id}`

Send the full object — every editable field is replaced, and `bullets` replaces the stored set entirely. Omit `display_order` to keep the current position. Returns `200 OK` with the updated project.

## 6. Delete

### `DELETE /api/admin/projects/{id}`

Removes the project and its bullets. Returns `204 No Content`. Deleting an unknown id returns `404 project_not_found`.
