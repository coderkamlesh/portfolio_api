# Frontend Agent Rules

SolidJS frontend ke liye implementation rules. Is file ko **UI repo root me `AGENTS.md`** ke naam se rakho — coding agents yahi padhte hain.

**Purpose:** backend API ke against frontend likhte waqt agent ko repeated mistakes se bachana. Har rule ne ek real integration failure prevent karta hai.

---

## 1. Hard rules (inhe todna nahi hai)

1. **`docs/` padho, guess nahi karo.** Har endpoint ka exact field name, validation rule aur error code doc me likha hai. Guess kiye field names se zyada time lagega.
2. **Switch on `error.code`, never on `error.message`.** Message user-facing hai aur badal sakta hai. `code` stable contract hai.
3. **Non-JSON responses ko JSON mat samjho.** Resume download raw PDF bytes hai. `response.json()` throw karega.
4. **API secrets frontend me kabhi nahi.** Sirf `VITE_API_URL` public hai. Koi bhi token/secret hardcode mat karo.
5. **Backend ko modify mat karo.** Agar contract inconsistent lage, note karo aur report karo — silently adapt mat karo.
6. **`innerHTML` dangerously use mat karo.** Solid ka `{value}` text escape karta hai by default. XSS surface mat kholo.
7. **Infinite retry loop mat banao.** 401 pe ek refresh + ek retry. Phir bhi 401 → logout.
8. **Optimistic update nested data pe mat karo.** Experience/projects bullets transactional save hain — refetch karo.

---

## 2. The API client

Ek `api/` folder, dono apps (public + admin) share karein.

### Response parsing — sabse common bug

Har call me response type decide karna **zaroori** hai:

| Endpoint | Response | Flag |
|---|---|---|
| Almost everything | JSON | default |
| `/api/public/resume/download` | PDF bytes | `{ raw: true }` |
| S3 presigned upload | S3 response, not JSON | `fetch` directly, API through nahi |

```js
// WRONG — PDF hai, JSON nahi
const data = await fetch(`${API}/api/public/resume/download`).then((r) => r.json());

// RIGHT — plain anchor, browser handle karega
<a href={`${API}/api/public/resume/download`} download>Download resume</a>
```

### Error normalization

---

## 3. Auth rules

1. **2FA mandatory hai.** Backend pe email OTP compulsory hai, runtime bypass nahi hai. "Skip for now" ya "remember me" ka koi path nahi banao.
2. **Refresh token rotating hai.** Har refresh naya token deta hai, purana invalid. Isliye refresh ke baad **naya token store karo**, purana mat rakho.
3. **Access token short-lived hai** (~15 min default). Sirf ispe rely mat karo, refresh flow zaroori hai.
4. **Protected routes me guard lagao.** Route level pe check karo, sirf component render pe nahi.
5. **Token logout pe clear karo.** Local storage aur in-memory dono.

`docs/auth.md` pehle padho — is module me rat traps hain.

---

## 4. Upload rules (S3)

1. **3-step flow follow karo:** presign → direct S3 PUT → key save.
2. **Upload API ke through nahi jaana chahiye.** Browser seedha S3 ko `PUT` kare. API ke through bhejne se Lambda memory/bandwidth waste hota hai aur timeout hota hai.
3. **Returned headers exactly bhejo.** `Content-Type` signed hota hai — change kiya to S3 `403` deta hai.
4. **Key save karo, URL nahi.** URL expire hota hai. Content me key store karo, render ke waqt URL maango.
5. **CORS bucket pe configured hona chahiye.** Agar browser console me CORS error aaye aur code sahi lage, **pehle bucket CORS check karo**, code nahi.

Details: `docs/uploads.md`.

---

## 5. Data fetching rules

1. **Server state Query cache me.** Solid signals me mat rakho — cache invalidation free milti hai.
2. **Mutation ke baad affected query invalidate karo.** Inconsistent UI isse bada problem hai.
3. **Har query ke liye loading state.** Blank screen kabhi nahi.
4. **Har list ke liye empty state.** "Nothing here yet" — khaali screen bug dikhti hai.
5. **Nested bullets me refetch karo, optimistic update nahi.** Experience + bullets ek hi transactional request me save hote hain.
6. **Error state actionable ho.** Raw `error.message` mat dikhao, user ko next step batao.

---

## 6. Module-specific rules

### Resume PDF

- Plain `<a href>` + `download` use karo. `fetch` + blob sirf tab jab programmatic action chahiye.

---

## 7. Workflow rules

1. **Chhote, reviewable chunks me kaam karo.** Ek phase poora karke dikhao, phir agla.
2. **Har phase ke baad manually verify karo.** Sirf files likhna "done" nahi hai — screen pe dekh ke confirm karo.
3. **Koi dependency add karne se pehle poochho.** Existing stack me solve ho sakta hai.
4. **Refactor mat karo jo task ka hissa nahi hai.** Scope creep se bacho.
5. **Config files (tsconfig, eslint, tailwind config) bina poochhe mat badlo.**
6. **Comment English me likho.** Conversation Hinglish ho sakti hai, code English.
7. **Guess kiye APIs ko verify nahi kiya to bol ke ruko.** Hallucinated library call likhne se behtar clarification hai.

---

## 8. Stack

| Purpose | Tool |
|---|---|
| Framework | SolidJS |
| Router | Solid Router |
| Styling | TailwindCSS |
| Data fetching | TanStack Solid Query |
| HTTP | Custom `fetch` wrapper (Axios nahi) |
| Build | Vite |
| Language | JavaScript (ya TypeScript — pehle decide karo) |

**Existing dependencies se solve ho to naya package mat add karo.**

---

## 9. Definition of done

Koi bhi kaam complete tab maana jayega jab:

- [ ] Feature screen pe render ho raha hai
- [ ] Koi console error nahi
- [ ] Loading + empty + error states sab hain
- [ ] Mobile pe theek dikhta hai
- [ ] Koi hardcoded URL ya secret nahi
- [ ] Build pass hota hai
- [ ] Backend ke against manually verify kiya

---

## 10. Reference

| Doc | Covers |
|---|---|
| `docs/auth.md` | Login, 2FA, refresh, token flow |
| `docs/admin-login.md` | Admin panel login specifics |
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

**In sab ko implement karne se pehle padho.** Ye file rules deti hai, `docs/` me exact contract hai.

- Blob use karo to `URL.revokeObjectURL()` **zaroor** call karo — warna memory leak.
- PDF ASCII-only hai. Content me curly quotes/bullets/accented chars mat likho — PDF me fold ho jaate hain.
- `404 profile_not_found` ka matlab hai admin ne profile banaya hi nahi. **Ye frontend bug nahi.**

### Analytics

- `by_day` already dense hai (missing days `0` ke saath). **Gap fill ya sort karne ki zaroorat nahi.**
- Axis max `Math.max(1, ...)` — warna all-zero window pe divide by zero hoga.
- `unique_ips` ko "distinct log" mat bolo. Wo "distinct IP hashes" hai. Label: **"Unique visitors (by network)"**.
- `days` 1–365 ke beech. Range selector server se pucho, dates frontend me derive mat karo.

### Audit log

- `old_value` / `new_value` **strings hain, objects nahi.** `JSON.parse()` zaroori hai.
- Parse **lazily** karo — sirf jab row expand ho. 50 rows = 50 full snapshots.
- `created_at` **UTC** hai. Relative time dikhao, exact UTC tooltip me. Local me parse mat karo.
- Entries **newest first** hain — client-side sort ki zaroorat nahi.
- `entity_type` dropdown values **hardcode** karo. API isko validate nahi karta, typo pe empty result milega (error nahi).
- `action` ke valid values: `CREATE`, `UPDATE`, `DELETE`. Kuch bhi bhejo to `400`.


Har non-2xx response ko ek typed error me convert karo:

```js
export class ApiError extends Error {
  constructor(status, code, message) {
    super(message);
    this.status = status;
    this.code = code;
  }
}
```

UI hamesha `code` se branch kare. `message` sirf display ke liye.

### Token refresh

- `401` aaye to ek baar refresh karo, phir request retry karo.
- **Refresh calls serialize karo.** Token rotating hai — do concurrent refresh me ek doosre ka token invalidate kar deta hai aur user logout ho jaata hai.
- Refresh fail → user logout, login page pe bhejo.
