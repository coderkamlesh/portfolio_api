# Analytics — Frontend Guide

Analytics counts resume downloads. The public site triggers a count by downloading the PDF; the admin panel reads the numbers.

- Counts are **write-on-read**: the download endpoint records the visit, so there is no separate tracking beacon to call or lose.
- Visitor IPs are **never stored**. They are HMAC-hashed into a stable opaque identifier so the same visitor can be counted together across requests.
- The tracking write is **best effort**. A visitor receives the PDF even when the analytics insert fails.

## Endpoint map

| Method | Endpoint | Access token required | Success |
|---|---|---:|---:|
| `GET` | `/api/public/resume/download` | No | `200` (records the download) |
| `GET` | `/api/admin/analytics/downloads?days=30` | Yes | `200` |

## Reading the dashboard data

```http
GET /api/admin/analytics/downloads?days=30
Authorization: Bearer <access_token>
```

```json
{
  "total": 142,
  "unique_ips": 96,
  "by_day": [
    { "date": "2026-08-26", "count": 0 },
    { "date": "2026-08-27", "count": 2 },
    { "date": "2026-09-24", "count": 7 }
  ],
  "from": "2026-08-26",
  "to": "2026-09-24"
}
```

| Field | Type | Notes |
|---|---|---|
| `total` | number | Downloads inside the window. |
| `unique_ips` | number | Distinct hashed identifiers. See the warning below. |
| `by_day` | array | One point per day, oldest first. |
| `by_day[].date` | string | `YYYY-MM-DD`, UTC. |
| `by_day[].count` | number | `0` for days with no downloads. |
| `from` / `to` | string | Inclusive window bounds, `YYYY-MM-DD`, UTC. |

### `by_day` is always complete and always dense

The response contains **exactly `days` points**, one per calendar day, with explicit zeros for quiet days. A chart does not need to fill gaps, and a missing day is never a dropped row — the API pads them.

That is why `days` and `by_day.length` always agree. If you see a mismatch, you are reading a different response than the one described here.

The window **always ends today** (UTC) and starts `days - 1` days earlier. There is no arbitrary from/to picker; the range is derived server-side so the chart can never request an unbounded series.

### Query parameters

| Parameter | Default | Accepted | Behaviour |
|---|---:|---|---|
| `days` | `30` | `1`–`365` | `0`, negative, non-numeric or missing falls back to `30`. Above `365` is clamped to `365`. |

Out-of-range values are corrected, not rejected. A malformed value is treated as "use the default" because this is a dashboard hint, not a data contract — so a bad URL still renders a chart instead of an error page.

## The `unique_ips` warning

`unique_ips` counts **distinct HMAC hashes**, not distinct people.

- With `ANALYTICS_HASH_SECRET` set, the same IP produces the same hash on every request, so one visitor browsing from one network counts once per window.
- It cannot be reversed into an IP. A leaked table does not hand over visitor addresses.
- The same person on mobile data, office wifi and home counts as three. Treat the number as an upper bound on distinct visitors, not a headcount.
- Corporate NAT, CGNAT and campus networks can collapse many people into one hash, making it a lower bound in the other direction.

It is a directional signal, not a metric to report precisely. Pair it with `total`: a high total and a low unique count usually means repeat visits.

**When `ANALYTICS_HASH_SECRET` is empty**, hashes are stored as empty and excluded from the distinct count, so `unique_ips` is `0` while `total` stays correct. This is deliberate. The alternative — counting every empty hash as one value — would report a confident-looking `1` for an unbounded number of visitors. If you see `unique_ips: 0` with a non-zero `total`, the server needs the secret configured; it is not a data problem.

## Rendering the chart

```jsx
const { data } = useQuery({
  queryKey: ["analytics", "downloads", days],
  queryFn: () => api.get(`/api/admin/analytics/downloads?days=${days}`).then((r) => r.data),
});

// data.by_day is dense and ordered, so it can be used directly as the axis.
const max = Math.max(1, ...data.by_day.map((p) => p.count));
```

Points are ordered oldest-first and already cover the full range, so no date sorting or gap-filling belongs in the frontend. Clamp the axis maximum to at least `1` — an all-zero window otherwise divides by zero when you compute bar heights.

A range selector only needs to send a different `days` value; the server derives the dates.

## Error responses

| Situation | Status | `code` |
|---|---:|---|
| missing or invalid access token | `401` | `unauthorized` |
| token expired | `401` | `unauthorized` |
| database unreachable | `503` | `service_unavailable` |

There are no `400`s. Every parameter problem is silently corrected to a valid range.

## Privacy summary

| Stored | Not stored |
|---|---|
| HMAC of the IP | Raw IP address |
| User agent (truncated to 255 chars) | Any cookie or fingerprint |
| Referrer (truncated to 255 chars) | Anything that identifies a person across sites |

The hash is a **general-purpose** HMAC-SHA256 keyed by a dedicated secret. It is not reversible and is not a cryptographic commitment to anything. A separate `ANALYTICS_HASH_SECRET` is used rather than `JWT_SECRET` so that leaking either key does not compromise the other.

Because a stable hash is required for counting, the same visitor is correlatable across requests within the table. That is inherent to unique-visitor counting; do not add any client-side identifier on top of it.
