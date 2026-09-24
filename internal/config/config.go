package config

import (
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

const (
	// EmailProviderSES delivers OTP mails through AWS SES (default, production).
	EmailProviderSES = "ses"
	// EmailProviderLog only logs the mail body — meant for local development.
	EmailProviderLog = "log"
)

// Config holds every runtime setting of the API. Only the values marked
// required are mandatory at boot; everything else falls back to a sane
// default so a fresh clone can run with just Turso + JWT_SECRET.
type Config struct {
	// ---- Server ----
	ServerPort     string
	AllowedOrigins []string

	// ---- Turso (libSQL) ----
	TursoURL   string
	TursoToken string

	// ---- Tokens ----
	JWTSecret       string
	JWTIssuer       string
	AccessTokenTTL  time.Duration
	RefreshTokenTTL time.Duration

	// ---- Admin auth policy ----
	OTPLength         int
	OTPTTL            time.Duration
	OTPMaxAttempts    int
	OTPMaxPerWindow   int
	OTPWindow         time.Duration
	OTPResendCooldown time.Duration
	MinPasswordLength int
	LoginMaxAttempts  int
	LoginWindow       time.Duration

	// ---- Email / AWS SES ----
	EmailProvider       string
	AWSRegion           string
	AWSAccessKeyID      string
	AWSSecretAccessKey  string
	AWSSessionToken     string
	SESEndpointURL      string
	SESFromEmail        string
	SESFromName         string
	SESReplyTo          string
	SESConfigurationSet string

	// ---- AWS S3 (file uploads, presigned URLs) ----
	// S3Bucket is optional: when empty, UploadsEnabled reports false and the
	// upload endpoints answer 503 instead of failing the boot. That keeps a local
	// clone runnable without any AWS access.
	S3Bucket         string
	S3UploadPrefix   string
	S3PutPresignTTL  time.Duration
	S3GetPresignTTL  time.Duration
	S3MaxImageBytes  int64
	S3MaxResumeBytes int64

	// ---- Analytics ----
	// AnalyticsHashSecret keys the HMAC that turns a visitor IP into a stable
	// identifier. It is a dedicated secret rather than the JWT secret, so neither
	// key widens the other's exposure. When empty, downloads are still counted
	// but unique-visitor counts report zero.
	AnalyticsHashSecret string
}

// Load reads .env (when present) plus the process environment and validates
// the result. It exits the process on an unusable configuration so that a
// misconfigured container fails loudly instead of serving broken traffic.
func Load() *Config {
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found, using environment variables")
	}

	cfg := &Config{
		// PORT is set by the Lambda Web Adapter; SERVER_PORT wins locally.
		ServerPort:     getEnv("SERVER_PORT", getEnv("PORT", "8080")),
		AllowedOrigins: getEnvCSV("AUTH_ALLOWED_ORIGINS", []string{"http://localhost:5173", "http://localhost:3000"}),

		TursoURL:   os.Getenv("TURSO_DATABASE_URL"),
		TursoToken: os.Getenv("TURSO_AUTH_TOKEN"),

		JWTSecret:       os.Getenv("JWT_SECRET"),
		JWTIssuer:       getEnv("JWT_ISSUER", "portfolio-api"),
		AccessTokenTTL:  getEnvMinutes("JWT_ACCESS_TTL_MINUTES", 15),
		RefreshTokenTTL: getEnvDays("JWT_REFRESH_TTL_DAYS", 30),

		OTPLength:         getEnvInt("AUTH_OTP_LENGTH", 6),
		OTPTTL:            getEnvMinutes("AUTH_OTP_TTL_MINUTES", 10),
		OTPMaxAttempts:    getEnvInt("AUTH_OTP_MAX_ATTEMPTS", 5),
		OTPMaxPerWindow:   getEnvInt("AUTH_OTP_MAX_PER_WINDOW", 5),
		OTPWindow:         getEnvMinutes("AUTH_OTP_WINDOW_MINUTES", 60),
		OTPResendCooldown: getEnvSeconds("AUTH_OTP_RESEND_COOLDOWN_SECONDS", 30),
		MinPasswordLength: getEnvInt("AUTH_MIN_PASSWORD_LENGTH", 12),
		LoginMaxAttempts:  getEnvInt("AUTH_LOGIN_MAX_ATTEMPTS", 10),
		LoginWindow:       getEnvMinutes("AUTH_LOGIN_WINDOW_MINUTES", 15),

		EmailProvider:       strings.ToLower(getEnv("EMAIL_PROVIDER", EmailProviderSES)),
		AWSRegion:           getEnv("AWS_REGION", "ap-south-1"),
		AWSAccessKeyID:      os.Getenv("AWS_ACCESS_KEY_ID"),
		AWSSecretAccessKey:  os.Getenv("AWS_SECRET_ACCESS_KEY"),
		AWSSessionToken:     os.Getenv("AWS_SESSION_TOKEN"),
		SESEndpointURL:      os.Getenv("SES_ENDPOINT_URL"),
		SESFromEmail:        os.Getenv("SES_FROM_EMAIL"),
		SESFromName:         getEnv("SES_FROM_NAME", "Portfolio Admin"),
		SESReplyTo:          os.Getenv("SES_REPLY_TO"),
		SESConfigurationSet: os.Getenv("SES_CONFIGURATION_SET"),

		S3Bucket:         os.Getenv("S3_BUCKET"),
		S3UploadPrefix:   getEnv("S3_UPLOAD_PREFIX", "uploads"),
		S3PutPresignTTL:  getEnvMinutes("S3_PUT_PRESIGN_TTL_MINUTES", 15),
		S3GetPresignTTL:  getEnvMinutes("S3_GET_PRESIGN_TTL_MINUTES", 60),
		S3MaxImageBytes:  getEnvInt64("S3_MAX_IMAGE_BYTES", 5<<20),
		S3MaxResumeBytes: getEnvInt64("S3_MAX_RESUME_BYTES", 10<<20),

		AnalyticsHashSecret: os.Getenv("ANALYTICS_HASH_SECRET"),
	}

	cfg.validate()
	return cfg
}

// validate fails the boot when a required setting is missing or nonsensical.
func (c *Config) validate() {
	if c.TursoURL == "" || c.TursoToken == "" {
		log.Fatal("TURSO_DATABASE_URL and TURSO_AUTH_TOKEN must be set")
	}
	if len(c.JWTSecret) < 32 {
		log.Fatal("JWT_SECRET must be set and at least 32 characters long")
	}
	if c.EmailProvider != EmailProviderSES && c.EmailProvider != EmailProviderLog {
		log.Fatalf("EMAIL_PROVIDER must be %q or %q (got %q)", EmailProviderSES, EmailProviderLog, c.EmailProvider)
	}
	if c.EmailProvider == EmailProviderSES {
		if c.SESFromEmail == "" {
			log.Fatal("SES_FROM_EMAIL must be set when EMAIL_PROVIDER=ses")
		}
		if c.AWSRegion == "" {
			log.Fatal("AWS_REGION must be set when EMAIL_PROVIDER=ses")
		}
		if (c.AWSAccessKeyID == "") != (c.AWSSecretAccessKey == "") {
			log.Fatal("AWS_ACCESS_KEY_ID and AWS_SECRET_ACCESS_KEY must be set together (omit both to use the Lambda IAM role)")
		}
	}
	if c.OTPLength < 4 || c.OTPLength > 10 {
		log.Fatal("AUTH_OTP_LENGTH must be between 4 and 10")
	}
	if c.MinPasswordLength < 8 {
		log.Fatal("AUTH_MIN_PASSWORD_LENGTH must be at least 8")
	}
	// S3 stays optional so a fresh clone boots with only Turso + JWT_SECRET.
	if c.S3Bucket != "" {
		if c.AWSRegion == "" {
			log.Fatal("AWS_REGION must be set when S3_BUCKET is set")
		}
		if c.S3PutPresignTTL <= 0 || c.S3GetPresignTTL <= 0 {
			log.Fatal("S3_PUT_PRESIGN_TTL_MINUTES and S3_GET_PRESIGN_TTL_MINUTES must be positive")
		}
		if c.S3MaxImageBytes <= 0 || c.S3MaxResumeBytes <= 0 {
			log.Fatal("S3_MAX_IMAGE_BYTES and S3_MAX_RESUME_BYTES must be positive")
		}
	}
}

// UsesSES reports whether real mails are sent through AWS SES.
func (c *Config) UsesSES() bool { return c.EmailProvider == EmailProviderSES }

// UploadsEnabled reports whether S3 is configured. When it is false the upload
// endpoints answer 503 and the rest of the API keeps working, so a developer can
// run the project without any AWS access.
func (c *Config) UploadsEnabled() bool { return c.S3Bucket != "" }

// MailFrom renders the RFC 5322 From header, e.g. `Portfolio Admin <no-reply@x.com>`.
func (c *Config) MailFrom() string {
	if c.SESFromName == "" {
		return c.SESFromEmail
	}
	return c.SESFromName + " <" + c.SESFromEmail + ">"
}

func getEnv(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	return fallback
}

func getEnvInt(key string, fallback int) int {
	if v, ok := os.LookupEnv(key); ok && strings.TrimSpace(v) != "" {
		n, err := strconv.Atoi(strings.TrimSpace(v))
		if err != nil {
			log.Fatalf("%s must be an integer (got %q)", key, v)
		}
		return n
	}
	return fallback
}

func getEnvInt64(key string, fallback int64) int64 {
	if v, ok := os.LookupEnv(key); ok && strings.TrimSpace(v) != "" {
		n, err := strconv.ParseInt(strings.TrimSpace(v), 10, 64)
		if err != nil {
			log.Fatalf("%s must be an integer (got %q)", key, v)
		}
		return n
	}
	return fallback
}

func getEnvMinutes(key string, fallback int) time.Duration {
	return time.Duration(getEnvInt(key, fallback)) * time.Minute
}

func getEnvSeconds(key string, fallback int) time.Duration {
	return time.Duration(getEnvInt(key, fallback)) * time.Second
}

func getEnvDays(key string, fallback int) time.Duration {
	return time.Duration(getEnvInt(key, fallback)) * 24 * time.Hour
}

func getEnvCSV(key string, fallback []string) []string {
	v, ok := os.LookupEnv(key)
	if !ok || strings.TrimSpace(v) == "" {
		return fallback
	}
	out := make([]string, 0, len(fallback))
	for _, p := range strings.Split(v, ",") {
		if trimmed := strings.TrimSpace(p); trimmed != "" {
			out = append(out, trimmed)
		}
	}
	if len(out) == 0 {
		return fallback
	}
	return out
}
