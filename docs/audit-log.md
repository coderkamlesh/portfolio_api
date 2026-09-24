# Audit Log — Frontend Guide

The audit log is an append-only trail of every content change made through the admin API. It answers "who changed what, when, and what did it look like before".

- Rows are written by the **service layer**, on every create, update and delete of a content record.
- `old_value` and `new_value` are **JSON snapshots of the record as it was**, not field diffs. The frontend decides whether to diff or pretty-print them.
- Rows are immutable. There is no write, update or delete endpoint.
- The log is admin-only and read-only.

## Endpoint map

| Method | Endpoint | Access token required | Success |
|---|---|---:|---:|
| `GET` | `/api/admin/audit-log` | Yes | `200` |

## Reading the trail

```http
GET /api/admin/audit-log?limit=50&offset=0
Authorization: Bearer <access_token>
```

```json
{
  "entries": [
    {
      "id": "01JQ...",
      "admin_id": "01JQ...",
      "entity_type": "project",
      "entity_id": "01JQ...",
      "action": "UPDATE",
      "old_value": "{\"title\":\"Old title\",\"featured\":false}",
      "new_value": "{\"title\":\"New title\",\"featured\":true}",
      "created_at": "2026-09-24T14:32:10Z"
    }
  ],
  "total": 248,
  "limit": 50,
  "offset": 0
}
```

| Field | Type | Notes |
|---|---|---|
| `id` | string | Row ID. |
| `admin_id` | string | Acting admin. Omitted when the write had no authenticated actor. |
| `entity_type` | string | Which content table. See below. |
| `entity_id` | string | The record. Omitted when there is no single record ID. |
| `action` | string | `CREATE`, `UPDATE` or `DELETE`. |
| `old_value` | string | JSON snapshot before the change. Omitted on create. |
| `new_value` | string | JSON snapshot after the change. Omitted on delete. |
| `created_at` | string | `YYYY-MM-DDTHH:MM:SSZ`, UTC, second precision. |
| `total` | number | Rows matching the filters, **ignoring pagination**. |
| `limit` / `offset` | number | The values actually applied, after clamping. |

### `old_value` and `new_value` are strings

They are **not nested objects**. They arrive as JSON-encoded strings and must be parsed before use:

```js
const oldValue = entry.old_value ? JSON.parse(entry.old_value) : null;
```

Parsing is safe to do lazily — only when the user expands a row. A page of 50 rows carries 50 full snapshots, which is why pretty-printing every row by default is a performance mistake.

An empty or absent side means the action does not have one, not that the value was null:

| Action | `old_value` | `new_value` |
|---|---|---|
| `CREATE` | absent | full record |
| `UPDATE` | full record | full record |
| `DELETE` | full record | absent |

## Filters

| Parameter | Accepted | Notes |
|---|---|---|
| `entity_type` | see below | Case-insensitive. Omit or leave empty for "any". |
| `action` | `CREATE`, `UPDATE`, `DELETE` | Case-insensitive. Omit or leave empty for "any". |
| `limit` | `1`–`200` | `0`, negative or non-numeric falls back to `50`. Above `200` is clamped to `200`. |
| `offset` | `0`–`10000` | Negative or non-numeric falls back to `0`. Above `10000` is clamped to `10000`. |

Filters combine with AND. `?entity_type=project&action=DELETE` returns only deleted projects.

```js
const params = new URLSearchParams({
  entity_type: "project",   // omit the key entirely to disable the filter
  action: "DELETE",
  limit: "25",
  offset: "0",
});
```

**Send an absent filter as an absent key, not as an empty string.** `?action=` is handled identically to omitting it, but omitting it keeps the URL readable and avoids any future ambiguity about whether empty means "all" or "none".

### The only `400` in this module

An unrecognised `action` is rejected, because a typo would otherwise silently return an unfiltered page and look like the filter simply matched nothing:

```json
{
  "error": {
    "code": "validation_failed",
    "message": "action must be one of CREATE, UPDATE, DELETE."
  }
}
```

This is the only validation error. Bad pagination is corrected silently. `entity_type` is **not** validated — an unknown value returns an empty page with `total: 0`, so a typo there looks like "no results" rather than an error. Use the known list below to build the filter UI.


## Entity types

| `entity_type` | Covers |
|---|---|
| `profile` | The singleton profile record |
| `skill_category` | Skill categories |
| `skill` | Individual skills |
| `experience` | Work experiences |
| `project` | Projects |
| `education` | Education records |
| `extra` | Extra sections (certifications, languages, interests) |
| `social_link` | The social link set |

`social_link` is worth understanding separately. The API replaces the **whole set** in one call, so an update row holds the complete before-and-after state of every link, not one link. Its `entity_id` is empty because there is no single record.

## Pagination

`total` is the count of rows matching the filters, not the count on the current page. Page count is therefore `Math.ceil(total / limit)`.

Entries are returned **newest first**, so the trail reads as a reverse-chronological activity feed with no client-side sorting.

Because the API uses offset pagination, rows inserted while the user is paging can shift later pages. That is acceptable for an audit viewer, but it means page 2 can repeat an entry the user just saw. If you need a stable cursor, filter by a known `created_at` and treat the offset as a soft position rather than an exact one.

The `offset` ceiling of `10000` exists so a crafted URL cannot make the database walk an unbounded number of rows. If a user genuinely needs to go past it, narrow the filters instead of raising the limit.

## Suggested UI

A filter bar plus an expandable list is enough. Do not build a paginated table that shows all columns at once — the JSON snapshots are wide and mostly noise until someone asks for them.

1. **Header** — filter selects for `entity_type` and `action`, both with an "All" option that omits the key.
2. **List** — one row per entry: action badge, entity type, entity ID, relative timestamp.
3. **Expand** — parse and pretty-print `old_value` and `new_value` side by side. For `UPDATE`, highlight keys whose values differ.

```jsx
function AuditDiff({ oldRaw, newRaw }) {
  const before = oldRaw ? JSON.parse(oldRaw) : null;
  const after = newRaw ? JSON.parse(newRaw) : null;

  if (!before || !after) {
    // CREATE or DELETE — one side only, so there is nothing to diff.
    return <pre>{JSON.stringify(before ?? after, null, 2)}</pre>;
  }

  const keys = [...new Set([...Object.keys(before), ...Object.keys(after)])];
  return keys.map((key) => {
    const changed = JSON.stringify(before[key]) !== JSON.stringify(after[key]);
    return <div class={changed ? "changed" : ""}>{key}</div>;
  });
}
```

Use **relative time** in the list (`2 hours ago`) and the exact UTC timestamp in a tooltip or expanded row. `created_at` has second precision and is always UTC, so do not parse it as local time.

## Error responses

| Situation | Status | `code` |
|---|---:|---|
| missing or invalid access token | `401` | `unauthorized` |
| token expired | `401` | `unauthorized` |
| unknown `action` | `400` | `validation_failed` |
| database unreachable | `503` | `service_unavailable` |

## What is not audited

Reads are not audited. Viewing the admin panel, opening a record or downloading the resume produces no row. The trail records **changes only**.

Auth activity is also outside this trail: login, OTP verification and token refresh are handled by the auth module and are not written here. Two `admin_id` values for the same entity on the same day means the same person edited twice, not necessarily two different people — the app has a single admin account in normal use.
