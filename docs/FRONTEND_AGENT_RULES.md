# Frontend Agent Rules

Is file ko UI repo root me `AGENTS.md` ke naam se rakho.

**Purpose:** API contract follow karne se bachana. Har rule ek real integration failure prevent karta hai.

Stack, routing, styling, UI/UX — wo frontend repo ki apni baat hai. Yahan sirf API flow hai.

---

## Hard rules

1. **`docs/` padho, guess nahi karo.** Field names, validation rules aur error codes wahan exact likhe hain.
2. **`error.code` pe branch karo, `error.message` pe nahi.** Message badal sakta hai, `code` stable contract hai.
3. **Non-JSON responses ko JSON mat samjho.** Resume download raw PDF bytes hai.
4. **Secrets frontend me nahi.** Sirf public API base URL. Koi token/secret hardcode mat karo.
5. **Backend modify mat karo.** Contract inconsistent lage to report karo, silently adapt mat karo.
6. **Retry loop mat banao.** 401 pe ek refresh + ek retry, phir bhi 401 → logout.
7. **Nested data pe optimistic update nahi.** Experience/projects bullets transactional hain — refetch karo.

---

## Response types

| Endpoint | Response |
|---|---|
| Almost everything | JSON |
| `/api/public/resume/download` | PDF bytes (raw) |
| S3 presigned upload | S3 ka response, JSON nahi |

## Auth flow

1. **2FA mandatory hai.** Email OTP compulsory, bypass nahi. "Skip for now" ka path nahi.
2. **Password step tokens nahi deta.** `POST /api/auth/login` ke baad authenticated nahi — OTP verify ke baad hota hai.
3. **Refresh token rotating hai.** Har refresh naya token deta hai, purana invalid. Poora token pair atomically replace karo.
4. **Refresh serialize karo.** Do concurrent refresh me ek doosre ka token invalidate kar deta hai. Ek single-flight refresh, baaki queue me.
5. **Access token short-lived hai** (~15 min). Sirf ispe rely mat karo.
6. **Refresh token `Authorization` header me nahi.** Sirf `POST /api/auth/refresh` body me.
7. **Refresh fail →** tokens, admin aur challenge state clear, login pe bhejo.
8. **Logout pe token clear karo**, storage medium koi bhi ho.

Details: `docs/auth.md`.

---

## Upload flow (S3)

1. **3-step:** presign → seedha S3 `PUT` → key save.
2. **Upload API ke through nahi.** Browser seedha S3 ko `PUT` kare — API proxy se timeout/bandwidth waste hota hai.
3. **Returned headers exactly bhejo.** `Content-Type` signed hai, change kiya to S3 `403`.
4. **Key save karo, URL nahi.** URL expire hota hai.
5. **CORS error aaye to pehle bucket CORS check karo**, code nahi.

Details: `docs/uploads.md`.

---

## Auth ke bahar

- **Transactional saves refetch karo**, optimistic update nahi.
- **Error message actionable ho** — raw `error.message` mat dikhao.
- **Blob URL banaya to use ke baad revoke karo**, warna memory leak.
- **`404 profile_not_found` frontend bug nahi** — admin ne profile banaya hi nahi.
- **API already dense/order karke deta hai** jahan doc me likha hai — client-side sort/gap fill redundant hai.

---

## Reference

| Doc | Covers |
|---|---|
| `docs/auth.md` | Login, 2FA, refresh, password reset/change |
| `docs/profile.md` | Profile endpoints |
| `docs/skills.md` | Skills + categories |
| `docs/experience.md` | Experience + nested bullets |
| `docs/projects.md` | Projects + nested bullets |
| `docs/education.md` | Education |
| `docs/extras.md` | Extras |
| `docs/social-links.md` | Social links |
| `docs/uploads.md` | S3 presigned uploads |
| `docs/resume.md` | Resume PDF download |
| `docs/analytics.md` | Download analytics |
| `docs/audit-log.md` | Audit trail viewer |
| `PLAN.md` | Phase breakdown aur timeline |

Har endpoint ke exact contract `docs/` me hai. Ye file rules deti hai, wahi contract deta hai.
