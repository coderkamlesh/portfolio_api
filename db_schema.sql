-- ============================================================================
-- SCHEMA: PORTFOLIO & RESUME CMS (Turso / libSQL)
-- Aligned with AlgoMaster Job Search Playbook resume structure.
-- ============================================================================

PRAGMA foreign_keys = ON;

-- -------- Admin & Auth --------
CREATE TABLE admin_users (
    id              TEXT PRIMARY KEY,
    username        TEXT NOT NULL UNIQUE,
    email           TEXT NOT NULL UNIQUE,
    password_hash   TEXT NOT NULL,                 -- argon2id
    is_active       INTEGER NOT NULL DEFAULT 1,
    last_login_at   TEXT,
    created_at      TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at      TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE backup_codes (
    id              TEXT PRIMARY KEY,
    admin_id        TEXT NOT NULL REFERENCES admin_users(id) ON DELETE CASCADE,
    code_hash       TEXT NOT NULL,
    used_at         TEXT,
    created_at      TEXT NOT NULL DEFAULT (datetime('now'))
);
CREATE INDEX idx_backup_admin ON backup_codes(admin_id);

CREATE TABLE otp_challenges (
    id              TEXT PRIMARY KEY,
    admin_id        TEXT NOT NULL REFERENCES admin_users(id) ON DELETE CASCADE,
    otp_hash        TEXT NOT NULL,
    purpose         TEXT NOT NULL,                 -- 'LOGIN_2FA' | 'PASSWORD_RESET'
    expires_at      TEXT NOT NULL,
    consumed_at     TEXT,
    attempt_count   INTEGER NOT NULL DEFAULT 0,
    created_at      TEXT NOT NULL DEFAULT (datetime('now'))
);
CREATE INDEX idx_otp_admin_purpose ON otp_challenges(admin_id, purpose, expires_at);

CREATE TABLE refresh_tokens (
    id              TEXT PRIMARY KEY,
    admin_id        TEXT NOT NULL REFERENCES admin_users(id) ON DELETE CASCADE,
    token_hash      TEXT NOT NULL UNIQUE,
    expires_at      TEXT NOT NULL,
    revoked_at      TEXT,
    user_agent      TEXT,
    ip_address      TEXT,
    created_at      TEXT NOT NULL DEFAULT (datetime('now'))
);
CREATE INDEX idx_refresh_admin ON refresh_tokens(admin_id);

-- -------- Profile (Singleton — Contact Header + Summary) --------
CREATE TABLE profile_details (
    id              TEXT PRIMARY KEY,
    singleton_key   INTEGER NOT NULL DEFAULT 1 UNIQUE CHECK (singleton_key = 1),
    full_name       TEXT NOT NULL,
    title           TEXT NOT NULL,
    tagline         TEXT,                          -- short one-liner
    summary         TEXT,                          -- optional: senior/career switch
    email           TEXT NOT NULL,
    phone           TEXT,
    location        TEXT,                          -- city/metro (no full address)
    avatar_url      TEXT,                          -- NOT for US resumes
    linkedin_url    TEXT,
    github_url      TEXT,
    portfolio_url   TEXT,
    twitter_url     TEXT,
    resume_file_url TEXT,
    career_gap_note TEXT,                          -- optional: 1 neutral line
    experience_level TEXT,                         -- 'NEW_GRAD' | 'MID' | 'SENIOR'
    updated_at      TEXT NOT NULL DEFAULT (datetime('now'))
);

-- -------- Skills (Grouped by category, NO ratings) --------
CREATE TABLE skill_categories (
    id              TEXT PRIMARY KEY,
    name            TEXT NOT NULL UNIQUE,          -- 'Languages', 'Backend', 'Cloud'
    display_order   INTEGER NOT NULL DEFAULT 0,
    created_at      TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE skills (
    id              TEXT PRIMARY KEY,
    category_id     TEXT NOT NULL REFERENCES skill_categories(id) ON DELETE CASCADE,
    skill_name      TEXT NOT NULL,
    icon_slug       TEXT,                          -- optional
    display_order   INTEGER NOT NULL DEFAULT 0,
    created_at      TEXT NOT NULL DEFAULT (datetime('now')),
    UNIQUE(category_id, skill_name)
);
CREATE INDEX idx_skills_cat_order ON skills(category_id, display_order);

-- -------- Experience (Reverse chronological, X-Y-Z bullets) --------
CREATE TABLE work_experiences (
    id              TEXT PRIMARY KEY,
    company_name    TEXT NOT NULL,
    company_logo_url TEXT,
    role            TEXT NOT NULL,                 -- job title
    employment_type TEXT,                          -- 'FULL_TIME'|'CONTRACT'|'INTERN'|'PART_TIME'|'REMOTE'
    location        TEXT,
    start_date      TEXT NOT NULL,                 -- YYYY-MM-DD
    end_date        TEXT,
    is_current      INTEGER NOT NULL DEFAULT 0,
    technologies    TEXT,                          -- JSON array
    display_order   INTEGER NOT NULL DEFAULT 0,
    created_at      TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at      TEXT NOT NULL DEFAULT (datetime('now'))
);
CREATE INDEX idx_exp_order ON work_experiences(display_order);

CREATE TABLE experience_bullets (
    id              TEXT PRIMARY KEY,
    experience_id   TEXT NOT NULL REFERENCES work_experiences(id) ON DELETE CASCADE,
    bullet_point    TEXT NOT NULL,                 -- X-Y-Z format
    display_order   INTEGER NOT NULL DEFAULT 0
);
CREATE INDEX idx_bullets_exp ON experience_bullets(experience_id, display_order);

-- -------- Projects (One-line desc + personal contribution + bullets) --------
CREATE TABLE projects (
    id              TEXT PRIMARY KEY,
    title           TEXT NOT NULL,
    tagline         TEXT,                          -- one-line: what it does + who for
    description     TEXT NOT NULL,                 -- what you personally built
    project_type    TEXT,                          -- 'PERSONAL'|'ACADEMIC'|'OPEN_SOURCE'|'INTERNSHIP'
    role            TEXT,                          -- for team projects: "Backend Developer"
    technologies    TEXT NOT NULL,                 -- JSON array
    repo_url        TEXT,
    live_url        TEXT,
    image_url       TEXT,
    start_date      TEXT,
    end_date        TEXT,
    status          TEXT,                          -- 'COMPLETED'|'IN_PROGRESS'
    is_featured     INTEGER NOT NULL DEFAULT 0,
    display_order   INTEGER NOT NULL DEFAULT 0,
    created_at      TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at      TEXT NOT NULL DEFAULT (datetime('now'))
);
CREATE INDEX idx_proj_featured ON projects(is_featured, display_order);

-- ⚡ NEW: Project bullets (AlgoMaster: "Use 1-2 bullets to show your work")
CREATE TABLE project_bullets (
    id              TEXT PRIMARY KEY,
    project_id      TEXT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    bullet_point    TEXT NOT NULL,                 -- technical challenge / scale / tradeoff
    display_order   INTEGER NOT NULL DEFAULT 0
);
CREATE INDEX idx_proj_bullets ON project_bullets(project_id, display_order);

-- -------- Education (Sized to experience level) --------
CREATE TABLE educations (
    id              TEXT PRIMARY KEY,
    institution     TEXT NOT NULL,
    degree          TEXT NOT NULL,
    field_of_study  TEXT,
    start_year      INTEGER NOT NULL,
    end_year        INTEGER,
    grade           TEXT,                          -- legacy
    gpa             TEXT,                          -- e.g., "8.5/10" or "3.8/4.0"
    coursework      TEXT,                          -- JSON array: 3-6 relevant courses
    honors          TEXT,                          -- "Dean's List", "Gold Medal"
    display_order   INTEGER NOT NULL DEFAULT 0,
    created_at      TEXT NOT NULL DEFAULT (datetime('now'))
);
CREATE INDEX idx_edu_order ON educations(display_order);

-- -------- Extras (Certifications, Awards, Publications, etc.) --------
-- AlgoMaster: "worth including when they support the role"
CREATE TABLE extras (
    id              TEXT PRIMARY KEY,
    category        TEXT NOT NULL,                 -- 'CERTIFICATION'|'AWARD'|'PUBLICATION'|'OPEN_SOURCE'|'TALK'|'VOLUNTEER'
    title           TEXT NOT NULL,
    issuer          TEXT,                          -- org / platform
    issued_date     TEXT,                          -- YYYY-MM or YYYY-MM-DD
    credential_url  TEXT,
    description     TEXT,
    display_order   INTEGER NOT NULL DEFAULT 0,
    created_at      TEXT NOT NULL DEFAULT (datetime('now'))
);
CREATE INDEX idx_extras_cat ON extras(category, display_order);

-- -------- Social Links (flexible) --------
CREATE TABLE social_links (
    id              TEXT PRIMARY KEY,
    platform        TEXT NOT NULL,                 -- 'linkedin'|'github'|'twitter'|'medium'
    url             TEXT NOT NULL,
    display_order   INTEGER NOT NULL DEFAULT 0,
    UNIQUE(platform)
);

-- -------- Analytics --------
CREATE TABLE resume_downloads (
    id              TEXT PRIMARY KEY,
    downloaded_at   TEXT NOT NULL DEFAULT (datetime('now')),
    ip_hash         TEXT,
    user_agent      TEXT,
    referrer        TEXT
);
CREATE INDEX idx_downloads_date ON resume_downloads(downloaded_at);

-- -------- Audit Trail --------
CREATE TABLE audit_log (
    id              TEXT PRIMARY KEY,
    admin_id        TEXT REFERENCES admin_users(id) ON DELETE SET NULL,
    entity_type     TEXT NOT NULL,
    entity_id       TEXT,
    action          TEXT NOT NULL,                 -- 'CREATE'|'UPDATE'|'DELETE'
    old_value       TEXT,                          -- JSON snapshot
    new_value       TEXT,                          -- JSON snapshot
    created_at      TEXT NOT NULL DEFAULT (datetime('now'))
);
CREATE INDEX idx_audit_entity ON audit_log(entity_type, entity_id);