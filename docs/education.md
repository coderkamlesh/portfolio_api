# Education Module — Frontend Guide

The Education module powers the education section of the public portfolio and the education manager inside the admin panel.

- The **public** endpoint is open and returns the entries in `display_order`.
- The **admin** endpoints provide full CRUD operations protected by the access token.
- The table has no `updated_at` column, so admin responses expose `created_at` only. There is no "last edited" information for this module.

## Endpoint map

| Method | Endpoint | Access token required | Success |
|---|---|---:|---:|
| `GET` | `/api/public/education` | No | `200` |
| `GET` | `/api/admin/education` | Yes | `200` |
| `POST` | `/api/admin/education` | Yes | `201` |
| `PUT` | `/api/admin/education/{id}` | Yes | `200` |
| `DELETE` | `/api/admin/education/{id}` | Yes | `204` |

## Conventions

**Authentication:** none for `/api/public/education`, otherwise `Authorization: Bearer <access_token>`.

**Request content type:** `application/json`

**Response content type:** `application/json; charset=utf-8` (`DELETE` returns `204 No Content`).

**Cache policy:** every education response carries `Cache-Control: no-store` and `Pragma: no-cache`.

**Request body strictness:** capped at 64 KiB, exactly one JSON object, **unknown fields are rejected** (`400 invalid_json`).

**ID format:** every `id` is a UUID v4 string. Treat it as opaque.

**Wrapper key:** both list endpoints return the array under the key `education` (singular), not `educations`. Match the API exactly.

## Field rules and limits

| Field | Required | Rules |
|---|---:|---|
| `institution` | Yes | max 200 chars, no control characters |
| `degree` | Yes | max 150 chars, no control characters |
| `start_year` | Yes | integer, 1950 or later, at most current year + 10 |
| `end_year` | No | integer, must be `>= start_year`, at most current year + 10 |
| `field_of_study` | No | max 150 chars |
| `grade` | No | max 50 chars |
| `gpa` | No | max 20 chars, free text such as `8.5/10` or `3.8/4.0` — the API does not parse the scale |
| `coursework` | No | max 6 entries, max 100 chars each, blank entries dropped, always returned as `[]` |
| `honors` | No | max 200 chars |
| `display_order` | No | non-negative int; omit on update to keep the stored order, defaults to `0` on create |

**In-progress degree:** simply omit `end_year` (or send `null`). The response then omits `end_year` too, so render "Present" instead of a year.

**Ordering:** both listings use `display_order ASC, id ASC`.

Any rule violation returns `400` with `code: "validation_failed"` and a message naming the offending field. A missing entry returns `404` with `code: "education_not_found"`.

---

## 1. Public list

### `GET /api/public/education`

```json
{
  "education": [
    {
      "id": "7d2c9a11-4b3e-4f6a-9c8d-1e2f3a4b5c6d",
      "institution": "NIT Rourkela",
      "degree": "B.Tech",
      "field_of_study": "Computer Science and Engineering",
      "start_year": 2020,
      "end_year": 2026,
      "grade": "First Class",
      "gpa": "8.5/10",
      "coursework": [
        "Data Structures",
        "Operating Systems",
        "DBMS"
      ],
      "honors": "Dean's List",
      "display_order": 0
    }
  ]
}
```

An in-progress degree comes back **without** the `end_year` key:

```json
{
  "institution": "NIT Rourkela",
  "degree": "M.Tech",
  "start_year": 2026
}
```

## 2. Admin list

### `GET /api/admin/education`

Same array as the public list, with `created_at` added to every entry:

```json
{
  "education": [
    {
      "id": "7d2c9a11-4b3e-4f6a-9c8d-1e2f3a4b5c6d",
      "institution": "NIT Rourkela",
      "start_year": 2020,
      "end_year": 2026,
      "created_at": "2026-09-24T12:00:00Z"
    }
  ]
}
```

## 3. Create

### `POST /api/admin/education`

```json
{
  "institution": "NIT Rourkela",
  "degree": "B.Tech",
  "field_of_study": "Computer Science and Engineering",
  "start_year": 2020,
  "end_year": 2026,
  "grade": "First Class",
  "gpa": "8.5/10",
  "coursework": ["Data Structures", "Operating Systems", "DBMS"],
  "honors": "Dean's List",
  "display_order": 0
}
```

Returns `201 Created` with the created entry including its `id` and `created_at`.

## 4. Update

### `PUT /api/admin/education/{id}`

Send the full object — every editable field is replaced. Omit `display_order` to keep the current position. To mark a degree as finished, send the year; to mark it in progress again, send `"end_year": null`. Returns `200 OK` with the updated entry.

## 5. Delete

### `DELETE /api/admin/education/{id}`

Returns `204 No Content`. Deleting an unknown id returns `404 education_not_found`.
