# Frontend Integration Plan (Module 13)

SolidJS frontend ke liye integration plan. Backend Modules 1–12 complete hain — API stable hai, isliye frontend kaam parallel chal sakta hai.

**Out of scope for the frontend:** koi backend change. Agar koi endpoint ki contract yahan inconsistent lage, doc me note karo — silently adapt mat karo.

---

## 0. Setup — yeh sabse pehle

UI root me yeh copy karo:

```
docs/FRONTEND_AGENT_RULES.md      ->  ./AGENTS.md
docs/frontend-integration-plan.md ->  ./PLAN.md
docs/*.md (saari API guides)      ->  ./docs/
```

`AGENTS.md` ko repo root me rakhna zaroori hai — koi bhi coding agent usko padhega aur rules follow karega. Bina uske agent khud conventions guess karega.

**API docs padhna mandatory hai.** Is plan me jo bhi likha hai wo summary hai. Exact field names, validation rules aur error codes ke liye `docs/` me jao. Is plan pe implement karke guess kiye hue field names se zyada time lagega.

---

## 1. Architecture decisions (pehle decide, baad me code)

Ye decisions baad me revert karna mehnga hai. Inhe Phase 1 me hi finalize karo.

| Decision | Recommendation | Why |
|---|---|---|
| Data fetching | TanStack Solid Query | Cache invalidation free milta hai; mutations ke baad `invalidateQueries` se UI turant consistent hota hai. |
| HTTP client | Ek custom `fetch` wrapper | Axios se chhota. Sirf token attach, JSON parse, aur `ApiError` normalization chahiye. |
| Routing | Solid Router | Already plan me hai. |
| Styling | TailwindCSS | Already plan me hai. |
| Token storage | `localStorage` (access) + refresh flow | Backend ka `POST /api/auth/refresh` rotating token deta hai. `auth.md` padh ke implement karo. |
| State | Solid signals + Query cache mix | Server state Query me, UI-only state (sidebar open, form dirty) signals me. Redux/zustand ki zaroorat nahi. |
| Public vs admin | **Do alag apps ya ek app ke do route trees** | Public site SEO chahiye, admin panel nahi. Ek bundle me dono mat daalo. |

### Public / admin separation

Do approaches:

**A. Do alag Solid apps (recommended)** — alag builds, alag deploys. Public site static-friendly ho sakta hai, admin panel protected. Bundle size dono chhota rehta hai.

**B. Ek app, do route trees** — `/` public, `/admin/*` admin. Simpler dev setup, but public visitors download admin code bhi.

Agar time kam hai to B se start karo, deploy se pehle A pe shift kar lena. **A me shift karne ka cost B se zyada hai, isliye A pehle decide karo.**

### API base URL

```js
// .env
VITE_API_URL=https://your-api-id.ap-south-1.lambda-url.on.aws
```

CORS: backend `AUTH_ALLOWED_ORIGINS` me frontend origin add hona chahiye. Bhool gaye to browser console me CORS error aayega — wo backend config issue hai, frontend code nahi.


---

## 2. Shared layer (Phase 1 — dono apps ke liye)

Ek `api/` folder jo dono apps import karein (monorepo me shared package, alag repos me copy-paste).

### `api/client.js`

```js
const BASE = import.meta.env.VITE_API_URL;

export class ApiError extends Error {
  constructor(status, code, message) {
    super(message);
    this.status = status;
    this.code = code; // machine-readable, switch on this
  }
}

async function request(path, { method = "GET", body, auth = true, raw = false } = {}) {
  const headers = {};
  if (body !== undefined) headers["Content-Type"] = "application/json";
  if (auth) {
    const token = getAccessToken();
    if (token) headers.Authorization = `Bearer ${token}`;
  }

  const res = await fetch(BASE + path, {
    method,
    headers,
    body: body === undefined ? undefined : JSON.stringify(body),
  });

  if (res.status === 401 && auth) {
    // Token expired — try one refresh, then retry once.
    await refreshSession();
    return request(path, { method, body, auth, raw });
  }

  if (!res.ok) {
    const payload = await res.json().catch(() => ({}));
    const err = payload.error ?? {};

---

## 3. Phase breakdown

Har phase ke end pe kuch **visible aur testable** hona chahiye. Phase khatam mat maano jab sirf files likhi hain — jab user ne screen pe dekha ki kaam hua.

### Phase 0 — Foundation (1–2 din)

- [ ] `AGENTS.md` + `PLAN.md` + `docs/` UI root me copy
- [ ] Vite + SolidJS + Tailwind + Solid Router scaffold
- [ ] TanStack Solid Query install
- [ ] `api/client.js` + `api/endpoints.js`
- [ ] `.env` me `VITE_API_URL`
- [ ] Backend CORS me frontend origin verify
- [ ] Empty layout render ho raha hai

**Done ka matlab:** app chalta hai, `/api/public/profile` network tab me 200 dikhata hai.

### Phase 1 — Public site (3–4 din)

Order isliye ye hai: **profile pehle**, kyunki baaki har section usi se derive hota hai.

- [ ] Hero section — name, title, summary (`/api/public/profile`)
- [ ] Skills grid — category grouped (`/api/public/skills`)
- [ ] Experience timeline — reverse chronological + bullets
- [ ] Projects grid + detail page (`/api/public/projects/{id}`)
- [ ] Education list
- [ ] Extras section
- [ ] Social links footer
- [ ] **Resume download button** (naya — neeche detail)

#### Resume download button

```jsx
// Simplest correct form. Nothing else needed.
<a href={`${API}/api/public/resume/download`} download="resume.pdf">
  Download resume
</a>
```

**Do galtiyan jo yahan common hain:**

| Mistake | Consequence |
|---|---|
| `fetch()` + `response.json()` | PDF parse nahi hoga, exception throw hoga. Response raw bytes hai. |
| `fetch()` + blob, object URL revoke nahi kiya | Memory leak har click pe |

Agar programmatic chahiye to `docs/resume.md` me blob example hai — use tabhi jab zarurat ho.

**PDF ASCII-only hai.** Website pe `Spring Böt` dikh sakta hai, PDF me `Spring Bot` jayega. Content ASCII rakho taaki dono same dikhein.

### Phase 2 — Auth (2–3 din)

- [ ] Login form (`POST /api/auth/login`)
- [ ] **2FA OTP step** — login ke baad mandatory hai, skip nahi ho sakta
- [ ] Token storage + in-memory access token
- [ ] Refresh flow (`POST /api/auth/refresh`)
- [ ] Protected route wrapper
- [ ] Logout

**2FA ko skip mat karna.** Backend pe email OTP **mandatory** hai aur runtime toggle nahi hai. Isliye "remember me" ya "skip for now" ka koi path nahi hai — jo bhi flow banao, OTP step compulsory rahega.

`docs/auth.md` carefully padhna — refresh token **rotating** hai, matlab har refresh naya token deta hai aur purana invalid ho jaata hai. Concurrent refresh calls serialize karna padega, warna race condition me user logout ho jaata hai.

### Phase 3 — Admin content CRUD (5–7 din)

Har entity ke liye same pattern: list → form → create/update → delete.

- [ ] Profile editor
- [ ] Skills: categories + skills, reorder via `display_order`
- [ ] Experience editor + nested bullets (transactional save — ek request me dono)
- [ ] Projects editor + nested bullets + featured toggle
- [ ] Education editor
- [ ] Extras editor
- [ ] Social links editor

#### Nested bullets — ek important detail

Experience aur projects me bullets **nested** hain, aur API ek hi request me experience + bullets dono save karti hai (transactional). Isliye:

- Bullets ke liye alag endpoint nahi hai, aur nahi hoga.
- Form submit karte waqt bullets array ke saath bhejo.
- Ek bullet add/edit/delete karne ke liye poora experience dobara save karo.

Iska matlab: form me "save" ke baad turant refetch karo. Optimistic update se list out-of-sync ho sakti hai.

#### Image uploads (3 din)


---

## 4. Cross-cutting concerns

Ye har phase me lagte hain, alag task nahi.

| Concern | Rule |
|---|---|
| Loading state | Har query ke liye skeleton ya spinner. Blank screen kabhi nahi. |
| Error state | `ApiError.code` switch karo, message directly mat dikhao. User ko actionable text chahiye. |
| Empty state | Har list me "nothing here yet" state. Khali screen bug dikhti hai. |
| Optimistic updates | Sirf simple single-record mutations me. Nested bullets me mat karo. |
| Cache invalidation | Mutation ke baad affected query `invalidateQueries`. Inconsistent UI se bada problem ye hai. |
| Token expiry | 401 pe ek refresh + retry. Refresh fail → logout. Infinite retry loop mat banao. |
| XSS | Koi bhi `innerHTML` dangerously use mat karo. Solid ka `{value}` text escape karta hai by default. |
| Secrets | Koi API secret frontend me nahi. Sirf `VITE_API_URL` public hai. |
| Responsive | Mobile first. Admin panel bhi mobile pe usable hona chahiye. |

---

## 5. Testing

Minimum bar:

- [ ] Public site har section khula, koi API error nahi
- [ ] Login → OTP → dashboard poora flow
- [ ] Ek CRUD cycle har entity type pe (create → edit → delete)
- [ ] Ek image upload end-to-end
- [ ] Resume download click → file save hoti hai
- [ ] Analytics chart data se render hota hai
- [ ] Audit log filter + pagination
- [ ] Token expiry pe auto-refresh
- [ ] Offline / server down pe graceful error

Automated tests optional hain, but **auth flow aur token refresh ke liye likhna strongly recommended** — wo sabse zyada bug-prone area hai aur manually test karna tedious hai.

---

## 6. Definition of done

Module 13 complete tab maana jayega jab:

- [ ] Public site saare content sections render karta hai, mobile + desktop
- [ ] Resume download button kaam karta hai
- [ ] Admin login + 2FA flow poora
- [ ] Saari 7 content entities ka CRUD
- [ ] Image upload S3 se end-to-end
- [ ] Analytics chart
- [ ] Audit log viewer
- [ ] Koi console error nahi (browser me)
- [ ] Koi hardcoded API URL nahi
- [ ] Production build successfully deploy ho

---

## 7. Open questions

Ye decide karne se pehle clarify karo — inka answer implementation change karta hai:

1. **Public aur admin ek app me ya alag?** (Section 1 me recommend kiya tha — alag)
2. **Resume upload vs generate?** Abhi generate hota hai. Agar admin PDF manually upload karna chahta hai to `uploads.md` ka `kind: "resume"` flow use hoga — but ye alag feature hai.
3. **Dark mode?** Spec me nahi tha. Cheap hai but har component me effort lagta hai.
4. **Deploy kahan?** Static host (Vercel/Netlify/Cloudflare) ya kuch aur. Ye CORS config affect karta hai.
5. **Content language?** Resume English me hai. Public site bilingual chahiye to i18n layer add karni padegi — pehle se plan karo, baad me retrofit painful hai.

---

## Appendix: naye endpoints ka quick reference

| Method | Endpoint | Auth | Doc |
|---|---|---:|---|
| `GET` | `/api/public/resume/download` | No | `docs/resume.md` |
| `GET` | `/api/admin/analytics/downloads?days=30` | Yes | `docs/analytics.md` |
| `GET` | `/api/admin/audit-log?limit=50&offset=0` | Yes | `docs/audit-log.md` |

Baaki saare endpoints Modules 1–9 ke docs me hain: `auth.md`, `profile.md`, `skills.md`, `experience.md`, `projects.md`, `education.md`, `extras.md`, `social-links.md`, `uploads.md`, `admin-login.md`.

**In sab ko padho.** Ye appendix sirf naye modules ka index hai, replacement nahi.

- [ ] Upload flow (3 steps — `docs/uploads.md` padho)
- [ ] Upload component reusable across profile, projects, experience

**Sabse common failure yahin hai:** S3 upload `fetch(upload_url, ...)` se **directly** hona chahiye, API ke through nahi. Aur returned `headers` **exactly** jaane chahiye — `Content-Type` signed hai, change kiya to S3 `403` dega.

CORS bucket pe configured hona chahiye warna preflight fail hoga jabki code sahi ho. Browser console me CORS error aaye to pehle bucket check karo, code nahi.

Uploaded file ka **key** save karo, URL nahi. URL expire hota hai.

### Phase 4 — Dashboard (2–3 din)

- [ ] Analytics chart (`/api/admin/analytics/downloads`)
- [ ] Audit log viewer (`/api/admin/audit-log`)

#### Analytics chart

```jsx
const { data } = useQuery({
  queryKey: ["analytics", days],
  queryFn: () => api.adminAnalytics(days),
});

const max = Math.max(1, ...data.by_day.map((p) => p.count));
```

`by_day` **already dense hai** — har din ka point aata hai, missing days `0` ke saath. Gaps fill karne ki zaroorat nahi, aur sorting bhi nahi.

`unique_ips` ko exact metric ki tarah present mat karo — wo "distinct IP hashes" hai, distinct log nahi. Ek banda wifi + mobile dono use kare to 2 count hota hai. Label rakho: **"Unique visitors (by network)"**.

Agar `unique_ips: 0` hai aur `total` 0 se zyada, backend pe `ANALYTICS_HASH_SECRET` missing hai. Ye frontend bug nahi.

#### Audit log viewer

- [ ] Filter bar: entity type + action dropdowns
- [ ] Paginated list, newest first
- [ ] Expandable row → parsed JSON diff

**Do jagah atakna hai:**

1. `old_value` / `new_value` **strings hain, objects nahi.** `JSON.parse()` zaroori hai, warna UI me `[object Object]` dikhega.
2. `created_at` **UTC** hai. Relative time dikhao, exact timestamp tooltip me. Local time me parse mat karo warna timestamps off lagein ge.

Unknown `action` pe API `400` deta hai, lekin `entity_type` validate nahi hota — typo pe empty result milega, error nahi. Isliye dropdown ke values hardcode karo, free text mat do.

Details: `docs/audit-log.md`.

    throw new ApiError(res.status, err.code ?? "unknown", err.message ?? res.statusText);
  }

  return raw ? res : res.json();
}
```

**Do rules jo is file me non-negotiable hain:**

1. **Never parse a non-JSON response as JSON.** Resume download PDF hai, uploads presigned URLs JSON hain. Har call me `raw` flag sahi set karo, warna `response.json()` throw karega.
2. **Switch on `code`, never on `message`.** Message user-facing hai aur kabhi bhi change ho sakta hai. `code` stable contract hai.

Infinite retry loop mat banao — refresh ke baad dobara `401` aaye to logout karna padega.

### `api/endpoints.js`

Ek jagah saare endpoints, taaki koi URL typo na ho:

```js
export const api = {
  // public
  publicProfile:   ()    => request("/api/public/profile"),
  publicSkills:    ()    => request("/api/public/skills"),
  publicExperience:()    => request("/api/public/experience"),
  publicProjects:  ()    => request("/api/public/projects"),
  publicProject:   (id)  => request(`/api/public/projects/${id}`),
  publicEducation: ()    => request("/api/public/education"),
  publicExtras:    ()    => request("/api/public/extras"),
  publicSocial:    ()    => request("/api/public/social-links"),
  resumeDownload:  ()    => request("/api/public/resume/download", { auth: false, raw: true }),

  // auth
  login:     (b) => request("/api/auth/login", { method: "POST", body: b, auth: false }),
  verify2FA: (b) => request("/api/auth/2fa/verify", { method: "POST", body: b, auth: false }),

  // admin
  adminAnalytics: (days = 30) => request(`/api/admin/analytics/downloads?days=${days}`),
  auditLog: (params) => request(`/api/admin/audit-log?${new URLSearchParams(params)}`),
};
```

Full list `docs/` me hai — yahan sirf naye Modules 10–12 ke endpoints add hue hain. Baaki patterns same hain.
