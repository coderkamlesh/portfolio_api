# Portfolio API

Backend API for a personal portfolio & resume CMS. Powers the public portfolio site and a secure admin panel to manage projects, experience, skills, and generate an ATS-friendly PDF resume on the fly.

- **Runtime:** Go 1.27
- **Router:** go-chi/chi v5
- **Database:** Turso (libSQL / Turso Database)
- **Deploy:** AWS Lambda (container image via ECR) + Lambda Web Adapter
- **CI/CD:** GitHub Actions

---

## Tech Stack

| Layer | Choice | Why |
|---|---|---|
| Language | Go 1.27 | Fast, static binary, tiny cold start |
| Router | go-chi/chi v5 | Lightweight, `net/http`-compatible, zero deps |
| DB Driver | `turso.tech/database/tursogo-serverless` | Pure Go, HTTP-based, Lambda-friendly |
| Database | Turso | Edge SQLite, generous free tier |
| PDF | go-pdf/fpdf | ATS-friendly, text-based PDF |
| Auth | JWT + TOTP/Email 2FA | Stateless + secure admin |
| Container | Alpine + Lambda Web Adapter | Run as normal HTTP server, deploy to Lambda |

---

## Project Structure

```
portfolio_api/
├── cmd/
│   └── api/
│       └── main.go              # Entry point
├── internal/
│   ├── config/                  # Env loading
│   ├── database/                # Turso connection
│   ├── models/                  # Domain structs
│   ├── repository/              # SQL queries
│   ├── service/                 # Business logic
│   ├── handler/                 # HTTP handlers
│   ├── middleware/              # Auth, CORS, Logger
│   └── router/                  # Chi route setup
├── migrations/                  # SQL schema reference
├── .env.example
├── Dockerfile
├── go.mod
└── README.md
```

---

## Prerequisites

- Go 1.27+
- Docker (for container builds)
- Turso CLI (`turso db show`, `turso db tokens create`)
- AWS account (Lambda + ECR access)

---

## Environment Variables

Copy `.env.example` to `.env` and fill in:

| Variable | Description |
|---|---|
| `TURSO_DATABASE_URL` | `turso://<db-name>-<org>.turso.io` |
| `TURSO_AUTH_TOKEN` | Token from `turso db tokens create <db>` |
| `SERVER_PORT` | Local HTTP port (default `8080`) |
| `JWT_SECRET` | Long random string for signing tokens |

Generate a strong `JWT_SECRET`:

```bash
openssl rand -base64 48
```

---

## Local Development

```bash
# Install dependencies
go mod download

# Run
go run cmd/api/main.go
```

Server starts at `http://localhost:8080`.

Health check:

```bash
curl http://localhost:8080/
# {"status":"ok","service":"portfolio-api"}
```

---

## Database Setup (Turso)

```bash
# Create DB (one time)
turso db create portfoliov2

# Get URL
turso db show portfoliov2 --url

# Create auth token
turso db tokens create portfoliov2

# Apply schema
turso db shell portfoliov2 < migrations/001_init.sql
```

### Schema Overview

| Table | Purpose |
|---|---|
| `admin_users` | Admin accounts (argon2id hashed) |
| `admin_2fa` | Per-admin 2FA config (TOTP / Email) |
| `backup_codes` | One-time recovery codes |
| `otp_challenges` | Email OTP challenges |
| `refresh_tokens` | JWT refresh tokens |
| `profile_details` | Singleton profile row |
| `skill_categories`, `skills` | Grouped skills |
| `work_experiences`, `experience_bullets` | Experience + X-Y-Z bullets |
| `projects`, `project_bullets` | Projects + bullets |
| `educations` | Education history |
| `extras` | Certifications, awards, publications |
| `social_links` | Flexible social platforms |
| `resume_downloads` | Analytics |
| `audit_log` | Change history |

---

## Docker

Build locally (arm64 for Lambda):

```bash
docker buildx build --platform linux/arm64 -t portfolio-api:local .
```

Run locally:

```bash
docker run -p 8080:8080 --env-file .env portfolio-api:local
```

> The Lambda Web Adapter is bundled in the image but only activates inside AWS Lambda. Locally, the container behaves like a normal HTTP server.

---

## Deployment

### Flow

```
git push → GitHub Actions → Docker build (arm64) → ECR push → Lambda update
```

### One-Time AWS Setup

1. **Create ECR repository** (e.g. `portfolio-api`) in `ap-south-1`.
2. **Create IAM user** with `AmazonEC2ContainerRegistryFullAccess` and `AWSLambda_FullAccess`.
3. **Add GitHub Secrets** (repo → Settings → Secrets → Actions):

   | Secret | Value |
   |---|---|
   | `AWS_ACCESS_KEY_ID` | IAM user access key |
   | `AWS_SECRET_ACCESS_KEY` | IAM user secret key |
   | `ECR_REPOSITORY` | ECR repo name |
   | `LAMBDA_FUNCTION_NAME` | Lambda function name |

4. **First push** → image lands in ECR (Lambda step auto-skips).
5. **Create Lambda function** in console:
   - Container image → ECR → `latest` tag
   - **Architecture: arm64**
   - Memory: 512 MB, Timeout: 30s (tune later)
   - Add env vars: `TURSO_DATABASE_URL`, `TURSO_AUTH_TOKEN`, `JWT_SECRET`
   - Function URL or API Gateway trigger
6. **Second push** → Lambda auto-updates with new image.

---

## API Endpoints

### Public

| Method | Endpoint | Status |
|---|---|---|
| GET | `/` | Live |
| GET | `/api/public/profile` | Planned |
| GET | `/api/public/projects` | Planned |
| GET | `/api/public/experience` | Planned |
| GET | `/api/public/skills` | Planned |
| GET | `/api/public/resume/download` | Planned |

### Admin (JWT required)

| Method | Endpoint | Status |
|---|---|---|
| POST | `/api/auth/login` | Planned |
| POST | `/api/auth/2fa/verify` | Planned |
| CRUD | `/api/admin/projects` | Planned |
| CRUD | `/api/admin/experience` | Planned |
| CRUD | `/api/admin/skills` | Planned |
| CRUD | `/api/admin/education` | Planned |
| CRUD | `/api/admin/extras` | Planned |
| PUT | `/api/admin/profile` | Planned |

---

## Roadmap

- [x] Turso schema
- [x] Go project scaffolding
- [x] Config + DB connection
- [x] Chi router + root endpoint
- [x] Dockerfile + GitHub Actions deploy
- [ ] Models + repositories
- [ ] Public API handlers
- [ ] Auth (JWT + 2FA)
- [ ] Admin CRUD
- [ ] PDF resume generation
- [ ] S3 image uploads (presigned URLs)
- [ ] SolidJS frontend (public site + admin panel)

---

## License

Private project. All rights reserved.