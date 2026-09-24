# Extras Module — Frontend Guide

The Extras module powers the certifications, awards, publications and other highlights shown on the public portfolio, plus their editor in the admin panel.

- The **public** endpoint is open and returns the entries **grouped by category**.
- The **admin** endpoint returns a **flat list** — the admin panel edits rows, not groups.
- The table has no `updated_at` column, so admin responses expose `created_at` only.

## Endpoint map

| Method | Endpoint | Access token required | Success |
|---|---|---:|---:|
| `GET` | `/api/public/extras` | No | `200` |
| `GET` | `/api/admin/extras` | Yes | `200` |
| `POST` | `/api/admin/extras` | Yes | `201` |
| `PUT` | `/api/admin/extras/{id}` | Yes | `200` |
| `DELETE` | `/api/admin/extras/{id}` | Yes | `204` |

## Conventions

**Authentication:** none for `/api/public/extras`, otherwise `Authorization: Bearer <access_token>`.

**Request content type:** `application/json`

**Response content type:** `application/json; charset=utf-8` (`DELETE` returns `204 No Content`).

**Cache policy:** every extras response carries `Cache-Control: no-store` and `Pragma: no-cache`.

**Request body strictness:** capped at 64 KiB, exactly one JSON object, **unknown fields are rejected** (`400 invalid_json`).

**ID format:** every `id` is a UUID v4 string. Treat it as opaque.

## Field rules and limits

| Field | Required | Rules |
|---|---:|---|
| `category` | Yes | one of the six values below — case-insensitive input, stored uppercase |
| `title` | Yes | max 200 chars, no control characters |
| `issuer` | No | max 200 chars (the organisation or platform) |
| `issued_date` | No | **`YYYY-MM` or `YYYY-MM-DD`** — both accepted, nothing else |
| `credential_url` | No | absolute `http`/`https` URL, max 500 chars |
| `description` | No | max 2000 chars |
| `display_order` | No | non-negative int; omit on update to keep the stored order, defaults to `0` on create |

**Accepted categories:** `CERTIFICATION`, `AWARD`, `PUBLICATION`, `OPEN_SOURCE`, `TALK`, `VOLUNTEER`.

**`issued_date` accepts two shapes.** This module is different from the other content modules: a certification is usually month-level (`2024-03`) while an award carries a full date (`2024-03-15`). The API stores whichever you send and returns it unchanged, so parse it defensively in the frontend — check the string length before constructing a `Date`. Values like `2024`, `2024/03`, `2024-13` and `2024-02-31` are rejected with `400 validation_failed`.

**Ordering:** the admin list uses `category ASC, display_order ASC, id ASC`. The public response groups follow a fixed category order — `CERTIFICATION`, `AWARD`, `PUBLICATION`, `OPEN_SOURCE`, `TALK`, `VOLUNTEER` — and only non-empty categories are included, so the frontend never renders an empty section.

Any rule violation returns `400` with `code: "validation_failed"` and a message naming the offending field. A missing entry returns `404` with `code: "extra_not_found"`.

---

## 1. Public list (grouped)

### `GET /api/public/extras`

```json
{
  "categories": [
    {
      "category": "CERTIFICATION",
      "extras": [
        {
          "id": "3f9b1c22-7a44-4d0e-9b18-2c5d6e7f8a90",
          "category": "CERTIFICATION",
          "title": "AWS Certified Solutions Architect",
          "issuer": "Amazon Web Services",
          "issued_date": "2024-03",
          "credential_url": "https://aws.amazon.com/certification",
          "description": "Validated design of highly available systems on AWS.",
          "display_order": 0
        }
      ]
    },
    {
      "category": "AWARD",
      "extras": [
        {
          "id": "5c1d2e33-8b55-4e1f-ac29-3d6e7f8a9b01",
          "category": "AWARD",
          "title": "Best Backend Project",
          "issuer": "AlgoMaster",
          "issued_date": "2024-03-15",
          "display_order": 0
        }
      ]
    }
  ]
}
```

Every entry repeats its own `category` inside `extras[]` as well, so an item stays self-describing if you render it outside its group.

Optional fields (`issuer`, `issued_date`, `credential_url`, `description`) are **omitted from the JSON entirely** when empty — they are not returned as `null`. Use a fallback such as `issuer ?? ""`.

## 2. Admin list (flat)

### `GET /api/admin/extras`

```json
{
  "extras": [
    {
      "id": "3f9b1c22-7a44-4d0e-9b18-2c5d6e7f8a90",
      "category": "CERTIFICATION",
      "title": "AWS Certified Solutions Architect",
      "issuer": "Amazon Web Services",
      "issued_date": "2024-03",
      "credential_url": "https://aws.amazon.com/certification",
      "description": "Validated design of highly available systems on AWS.",
      "display_order": 0,
      "created_at": "2026-09-24T12:00:00Z"
    }
  ]
}
```

## 3. Create

### `POST /api/admin/extras`

```json
{
  "category": "certification",
  "title": "AWS Certified Solutions Architect",
  "issuer": "Amazon Web Services",
  "issued_date": "2024-03",
  "credential_url": "https://aws.amazon.com/certification",
  "description": "Validated design of highly available systems on AWS.",
  "display_order": 0
}
```

Returns `201 Created` with the created entry including its `id` and `created_at`. `category` comes back upper-cased.

## 4. Update

### `PUT /api/admin/extras/{id}`

Send the full object — every editable field is replaced. Omit `display_order` to keep the current position. Send `""` for a field to clear it. Returns `200 OK` with the updated entry.

## 5. Delete

### `DELETE /api/admin/extras/{id}`

Returns `204 No Content`. Deleting an unknown id returns `404 extra_not_found`.
