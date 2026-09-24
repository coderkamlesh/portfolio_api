# Portfolio API — Module Roadmap

Sequence-wise implementation plan. Har module ka structure same:
**Models → Repository → Service → Handler → Routes → Test**

---

## Legend

- ✅ Done
- 🔄 In Progress
- ⬜ Pending

---

## Module 1: Auth ✅

**Tables:** `admin_users`, `admin_2fa`, `otp_challenges`, `refresh_tokens`

**Scope:**
- Password hashing (argon2id)
- Login (username/email + password)
- Mandatory email OTP 2FA (auto-provisioned, cannot be disabled)
- JWT access + rotating refresh tokens
- Password forgot/reset/change
- Logout / token revocation

**Endpoints:**
- `POST /api/auth/login`
- `POST /api/auth/2fa/verify`
- `POST /api/auth/2fa/resend`
- `POST /api/auth/refresh`
- `POST /api/auth/password/forgot`
- `POST /api/auth/password/reset`
- `GET /api/auth/me`
- `POST /api/auth/logout`
- `POST /api/auth/password/change`
- `GET /api/auth/2fa`
- `POST /api/auth/2fa/email/enable`

**Status:** ✅ Complete

---

## Module 2: Profile ✅

**Table:** `profile_details` (singleton)

**Scope:**
- Get public profile (name, title, summary, contact)
- Update profile (admin only)
- Singleton enforcement (always 1 row)

**Endpoints:**
- `GET /api/public/profile`
- `GET /api/admin/profile`
- `PUT /api/admin/profile`

**Files:**
- `internal/models/profile.go`
- `internal/repository/profile_repo.go`
- `internal/service/profile_service.go`
- `internal/handler/profile_handler.go`

**Status:** ✅ Complete

---

## Module 3: Skills ✅

**Tables:** `skill_categories`, `skills`

**Scope:**
- Public: grouped skills (category → skills[])
- Admin CRUD for both categories and skills
- Reorder (display_order)

**Endpoints:**
- `GET /api/public/skills`
- `GET /api/admin/skill-categories`
- `POST /api/admin/skill-categories`
- `PUT /api/admin/skill-categories/{id}`
- `DELETE /api/admin/skill-categories/{id}`
- `GET /api/admin/skills`
- `POST /api/admin/skills`
- `PUT /api/admin/skills/{id}`
- `DELETE /api/admin/skills/{id}`

**Files:**
- `internal/models/skill.go`
- `internal/repository/skill_repo.go`
- `internal/service/skill_service.go`
- `internal/handler/skill_handler.go`
- `internal/service/skill_service_test.go`

**Status:** ✅ Complete

---

## Module 4: Experience ✅

**Tables:** `work_experiences`, `experience_bullets`

**Scope:**
- Public: reverse chronological list with bullets
- Admin CRUD (experience + nested bullets)
- Transactional save (experience + bullets ek saath)

**Endpoints:**
- `GET /api/public/experience`
- `GET /api/admin/experience`
- `POST /api/admin/experience`
- `PUT /api/admin/experience/{id}`
- `DELETE /api/admin/experience/{id}`

**Files:**
- `internal/models/experience.go`
- `internal/repository/experience_repo.go`
- `internal/service/experience_service.go`
- `internal/handler/experience_handler.go`

---

## Module 5: Projects ✅

**Tables:** `projects`, `project_bullets`

**Scope:**
- Public: list (featured first, then display_order)
- Public: single project detail
- Admin CRUD (project + nested bullets)
- Featured toggle

**Endpoints:**
- `GET /api/public/projects`
- `GET /api/public/projects/{id}`
- `GET /api/admin/projects`
- `POST /api/admin/projects`
- `PUT /api/admin/projects/{id}`
- `DELETE /api/admin/projects/{id}`

**Files:**
- `internal/models/project.go`
- `internal/repository/project_repo.go`
- `internal/service/project_service.go`
- `internal/handler/project_handler.go`

**Status:** ✅ Complete

---

## Module 6: Education ✅

**Table:** `educations`

**Scope:**
- Public: list (display_order)
- Admin CRUD

**Endpoints:**
- `GET /api/public/education`
- `GET /api/admin/education`
- `POST /api/admin/education`
- `PUT /api/admin/education/{id}`
- `DELETE /api/admin/education/{id}`

**Files:**
- `internal/models/education.go`
- `internal/repository/education_repo.go`
- `internal/service/education_service.go`
- `internal/handler/education_handler.go`

**Status:** ✅ Complete

---

## Module 7: Extras ✅

**Table:** `extras`

**Scope:**
- Public: grouped by category (CERTIFICATION, AWARD, PUBLICATION, etc.)
- Admin CRUD

**Endpoints:**
- `GET /api/public/extras`
- `GET /api/admin/extras`
- `POST /api/admin/extras`
- `PUT /api/admin/extras/{id}`
- `DELETE /api/admin/extras/{id}`

**Files:**
- `internal/models/extra.go`
- `internal/repository/extra_repo.go`
- `internal/service/extra_service.go`
- `internal/handler/extra_handler.go`

**Status:** ✅ Complete

---

## Module 8: Social Links ✅

**Table:** `social_links`

**Scope:**
- Public: list
- Admin CRUD (upsert by platform)

**Endpoints:**
- `GET /api/public/social-links`
- `PUT /api/admin/social-links`

**Files:**
- `internal/models/social_link.go`
- `internal/repository/social_link_repo.go`
- `internal/service/social_link_service.go`
- `internal/handler/social_link_handler.go`

**Status:** ✅ Complete

---

## Module 9: File Upload (S3) ✅

**AWS:** S3 (ap-south-1), presigned URLs

**Scope:**
- Presigned PUT URL generate karo (client direct S3 pe upload kare)
- Presigned GET URL (agar bucket private ho)
- File type + size validation
- Used in: avatar, project images, company logos, resume file

**Endpoints:**
- `POST /api/admin/upload/presign` (body: fileType, context)
- `DELETE /api/admin/upload/{key}`

**Files:**
- `internal/config/s3.go`
- `internal/service/upload_service.go`
- `internal/handler/upload_handler.go`

**Dependencies:**
- `github.com/aws/aws-sdk-go-v2/config`
- `github.com/aws/aws-sdk-go-v2/service/s3`

---

## Module 10: Resume PDF Generation ✅

**Library:** `github.com/go-pdf/fpdf`

**Scope:**
- DB se saara data fetch (profile, skills, experience, projects, education, extras)
- ATS-friendly PDF template
- On-demand generate + stream
- Analytics: `resume_downloads` me entry

**Endpoints:**
- `GET /api/public/resume/download`

**Files:**
- `internal/resume/data.go` (template contract)
- `internal/resume/renderer.go` (fpdf layout)
- `internal/resume/format.go` (ASCII sanitizer + date/byte helpers)
- `internal/resume/resume_test.go`
- `internal/service/resume_service.go`
- `internal/handler/resume_handler.go`

**Doc:** `docs/resume.md`

**Status:** ✅ Complete

**Notes:**
- Single column, selectable text, no icons / symbol fonts. Contact details body flow me, kyunki ATS parsers page header skip karte hain.
- **ASCII-only by design.** fpdf core fonts CP1252 hain aur string bytes verbatim likhte hain, isliye `•` (U+2022) mojibake ban jaata. `sanitize()` curly quotes, en/em-dashes, bullets, NBSP aur accented Latin ko ASCII me fold karta hai; unknown rune → visible `?`.
- Analytics write best-effort — insert fail ho to visitor ko PDF phir bhi milta hai.
- Missing profile → `404 profile_not_found` (500 nahi). Naam ke bina resume render karna bekaar hai.

---

## Module 11: Analytics ✅

**Table:** `resume_downloads`

**Scope:**
- Resume download tracking (IP hash, user agent, referrer)
- Admin dashboard: download count over time

**Endpoints:**
- `GET /api/admin/analytics/downloads`

**Files:**
- `internal/models/analytics.go`
- `internal/repository/analytics_repo.go`
- `internal/service/analytics_service.go`
- `internal/handler/resume_handler.go` (`AnalyticsHandler`)

**Config:** `ANALYTICS_HASH_SECRET`

**Doc:** `docs/analytics.md`

**Status:** ✅ Complete

**Notes:**
- IP **HMAC-SHA256** with a dedicated secret. `JWT_SECRET` reuse nahi — do alag keys ka blast radius alag rehta hai.
- Empty secret ⇒ empty hash ⇒ `unique_ips` **0, 1 nahi**. Unconfigured deployment pe confident-looking `1` galat hota; SQL NULL/empty rows distinct count se exclude hote hain.
- Window `days` 1–365 clamp; 0 / negative / non-numeric → default 30. `by_day` always dense hai (missing days explicit `0`) — chart ke liye ready.
- Dono bounds `startOfToday` se derive hote hain. Mid-day timestamp pe `+24h` *kal* de deta, jis se future day report hota aur series me phantom zero aata.

---

## Module 12: Audit Log ✅

**Table:** `audit_log`

**Scope:**
- Service-layer hook — every admin write logs old + new JSON
- Admin view: paginated audit trail

**Endpoints:**
- `GET /api/admin/audit-log`

**Files:**
- `internal/service/audit.go` (actor context + `Audit.Record`)
- `internal/service/audit_query_service.go`
- `internal/repository/audit.go` (List/Count)
- `internal/handler/audit_handler.go`
- `internal/handler/respond.go` (`adminContext`)

**Doc:** `docs/audit-log.md`

**Status:** ✅ Complete

**Notes:**
- **Service layer, not middleware.** Generic HTTP middleware ko `old_value` nahi pata hota — service ke paas record ka before/after state hota hai. Middleware se sirf "kaun, kab, kis route pe" milta, "kyun badla" nahi.
- Actor ID handler inject karta hai (`service.WithActor(ctx, adminID)`), isliye kisi bhi service method ka signature nahi badla — zero test churn. 20 call sites switched.
- 8 entity types. Social links ka diff **per platform** nahi, poora set snapshot hai (API `PUT /social-links` se replace karti hai), isliye uska `entity_id` khali rehta hai.
- `limit` 1–200, `offset` 0–10000 clamp. Sirf unknown `action` pe `400` — baaki query params silently correct hote hain, kyunki wo dashboard hints hain, strict contract nahi.
- Audit write failure logged + swallowed: full audit table se admin ka edit rollback nahi hona chahiye. Ek trail row khona behtar hai edit se.

---

## Module 13: Frontend — SolidJS ⬜

**Repo:** alag (ya monorepo me `web/` folder)

**Integration plan:** `docs/frontend-integration-plan.md`
**Agent rules:** `docs/FRONTEND_AGENT_RULES.md` — ise UI root me copy karo
**API docs:** `docs/*.md` — UI root me `docs/` folder bana ke saari files copy karo

### 13.1 Public Site
- Hero (profile)
- Skills grid
- Experience timeline
- Projects cards
- Education
- Extras
- Resume download button
- Contact / social links

### 13.2 Admin Panel
- Login + 2FA flow
- Dashboard
- CRUD screens for: profile, skills, experience, projects, education, extras, social links
- Image upload (S3 presigned)
- Audit log viewer
- Analytics viewer

**Stack:**
- SolidJS + Solid Router
- TailwindCSS
- TanStack Solid Query
- Axios / fetch wrapper

---

## Cross-Cutting Concerns (har module me)

| Concern | Implementation |
|---|---|
| **Validation** | Handler level — simple checks; complex ho to `go-playground/validator` |
| **Error handling** | Central `internal/errors` package — typed errors → HTTP status mapping |
| **Logging** | `slog` (Go 1.21+) — structured logs |
| **CORS** | `go-chi/cors` middleware |
| **Auth middleware** | JWT verify + admin role check |
| **Rate limiting** | Login + 2FA endpoints pe (optional — `httprate`) |
| **Request ID** | `chi/middleware.RequestID` |
| **Panic recovery** | `chi/middleware.Recoverer` |

---

## Suggested Sequence

```
1. Auth              ✅
2. Profile           ✅
3. Skills            ✅
4. Experience        ✅
5. Projects          ✅
6. Education         ✅
7. Extras            ✅
8. Social Links      ✅
9. File Upload (S3)  ✅
10. Resume PDF        ✅
11. Analytics         ✅
12. Audit Log         ✅
13. Frontend          ⬜ ← next
```

**Kyun ye order:**
- Profile pehle (baaki sab isi ke saath render honge)
- Content modules (skills → experience → projects → education → extras) — PDF inhi pe depend karta hai
- Upload + PDF baad me (in sab ka data chahiye)
- Analytics + Audit — production polish
- Frontend sabse aakhir (API stable hone ke baad)

---

## Har Module Ka Checklist

```
[ ] models/*.go          — structs + JSON tags
[ ] repository/*.go      — SQL queries
[ ] service/*.go         — business logic
[ ] handler/*.go         — HTTP layer
[ ] router me routes     — public + admin
[ ] manual test (Bruno/curl)
[ ] commit
```