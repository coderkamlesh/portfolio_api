package service

import (
	"fmt"
	"net/http"

	"github.com/coderkamlesh/portfolio_api/internal/apierr"
)

func newErr(status int, code, message string) error {
	return apierr.New(status, code, message)
}

func wrapErr(err error, status int, code, message string) error {
	return apierr.Wrap(err, status, code, message)
}

// newRetryErr builds a 429 that also tells the client when to come back.
func newRetryErr(code, message string, retryAfter int) error {
	e := apierr.New(http.StatusTooManyRequests, code, message)
	e.RetryAfter = retryAfter
	return e
}

// Stable machine-readable error codes returned in `error.code`. The admin
// panel switches on these instead of parsing messages.
const (
	CodeInvalidCredentials = "invalid_credentials"
	CodeAccountDisabled    = "account_disabled"
	CodeRateLimited        = "too_many_attempts"

	CodeInvalidChallenge = "invalid_challenge"
	CodeChallengeExpired = "challenge_expired"
	CodeChallengeUsed    = "challenge_already_used"
	CodeInvalidOTP       = "invalid_otp"
	CodeOTPAttempts      = "otp_attempts_exceeded"
	CodeOTPTooMany       = "otp_too_many_requests"
	CodeOTPCooldown      = "otp_resend_cooldown"

	CodeInvalidRefreshToken = "invalid_refresh_token"
	CodeRefreshTokenReused  = "refresh_token_reused"

	CodeEmailDeliveryFailed = "email_delivery_failed"
	CodeWeakPassword        = "weak_password"
	CodeInvalidPassword     = "invalid_password"
	CodeSamePassword        = "password_unchanged"
	CodeProfileNotFound     = "profile_not_found"

	CodeSkillCategoryNotFound = "skill_category_not_found"
	CodeSkillNotFound         = "skill_not_found"
	CodeSkillCategoryExists   = "skill_category_exists"
	CodeSkillExists           = "skill_exists"
	CodeExperienceNotFound    = "experience_not_found"
	CodeProjectNotFound       = "project_not_found"
	CodeEducationNotFound     = "education_not_found"
	CodeExtraNotFound         = "extra_not_found"
	CodeSocialLinkConflict    = "social_link_conflict"
)

func errInvalidCredentials() error {
	return newErr(http.StatusUnauthorized, CodeInvalidCredentials, "Invalid username or password.")
}

func errAccountDisabled() error {
	return newErr(http.StatusForbidden, CodeAccountDisabled, "This admin account is disabled.")
}

func errRateLimited(retryAfter int) error {
	return newRetryErr(CodeRateLimited,
		fmt.Sprintf("Too many failed attempts. Try again in %d seconds.", retryAfter), retryAfter)
}

func errInvalidChallenge() error {
	return newErr(http.StatusBadRequest, CodeInvalidChallenge, "This verification request is not valid. Please sign in again.")
}

func errChallengeExpired() error {
	return newErr(http.StatusGone, CodeChallengeExpired, "This code has expired. Please sign in again or request a new code.")
}

func errChallengeUsed() error {
	return newErr(http.StatusGone, CodeChallengeUsed, "This code was already used. Please sign in again.")
}

func errInvalidOTP(attemptsLeft int) error {
	return newErr(http.StatusUnauthorized, CodeInvalidOTP,
		fmt.Sprintf("Incorrect code. %d attempt(s) left.", attemptsLeft))
}

func errOTPAttemptsExceeded() error {
	return newErr(http.StatusTooManyRequests, CodeOTPAttempts, "Too many incorrect codes. Please sign in again.")
}

func errOTPTooManyRequests() error {
	return newRetryErr(CodeOTPTooMany, "Too many codes requested. Please try again later.", 300)
}

func errOTPCooldown(retryAfter int) error {
	return newRetryErr(CodeOTPCooldown,
		fmt.Sprintf("Please wait %d seconds before requesting another code.", retryAfter), retryAfter)
}

func errInvalidRefreshToken() error {
	return newErr(http.StatusUnauthorized, CodeInvalidRefreshToken, "Your session has expired. Please sign in again.")
}

func errRefreshTokenReused() error {
	return newErr(http.StatusUnauthorized, CodeRefreshTokenReused,
		"This session was already used. All sessions have been revoked for safety — please sign in again.")
}

func errEmailDeliveryFailed(cause error) error {
	return wrapErr(cause, http.StatusBadGateway, CodeEmailDeliveryFailed,
		"Could not send the verification email. Please try again in a moment.")
}

func errWeakPassword(reason string) error {
	return newErr(http.StatusBadRequest, CodeWeakPassword, reason)
}

func errInvalidPassword() error {
	return newErr(http.StatusUnauthorized, CodeInvalidPassword, "The current password is incorrect.")
}

func errSamePassword() error {
	return newErr(http.StatusBadRequest, CodeSamePassword, "The new password must be different from the current one.")
}

func errProfileNotFound() error {
	return newErr(http.StatusNotFound, CodeProfileNotFound, "The profile has not been configured yet.")
}

func errProfileValidation(field string) error {
	return newErr(http.StatusBadRequest, "validation_failed", field+" is required.")
}

func errInvalidProfileEmail() error {
	return newErr(http.StatusBadRequest, "validation_failed", "email must be a valid email address.")
}

func errSkillCategoryNotFound() error {
	return newErr(http.StatusNotFound, CodeSkillCategoryNotFound, "This skill category does not exist.")
}

func errSkillNotFound() error {
	return newErr(http.StatusNotFound, CodeSkillNotFound, "This skill does not exist.")
}

func errSkillCategoryExists(name string) error {
	return newErr(http.StatusConflict, CodeSkillCategoryExists,
		fmt.Sprintf("A skill category named %q already exists.", name))
}

func errSkillExists(name string) error {
	return newErr(http.StatusConflict, CodeSkillExists,
		fmt.Sprintf("The skill %q already exists in this category.", name))
}

func errSkillValidation(field string) error {
	return newErr(http.StatusBadRequest, "validation_failed", field+" is required.")
}

func errSkillValidationTooLong(field string, max int) error {
	return newErr(http.StatusBadRequest, "validation_failed",
		fmt.Sprintf("%s must be at most %d characters.", field, max))
}

func errSkillValidationControlChars(field string) error {
	return newErr(http.StatusBadRequest, "validation_failed",
		field+" must not contain control characters.")
}

func errSkillValidationDisplayOrder() error {
	return newErr(http.StatusBadRequest, "validation_failed", "display_order must not be negative.")
}

func errExperienceNotFound() error {
	return newErr(http.StatusNotFound, CodeExperienceNotFound, "This experience entry does not exist.")
}

// errExperienceRequired reports a missing required field of an experience
// payload.
func errExperienceRequired(field string) error {
	return newErr(http.StatusBadRequest, "validation_failed", field+" is required.")
}

// errExperienceValidation reports a malformed experience payload with a message
// that names the offending field.
func errExperienceValidation(message string) error {
	return newErr(http.StatusBadRequest, "validation_failed", message)
}

func errProjectNotFound() error {
	return newErr(http.StatusNotFound, CodeProjectNotFound, "This project does not exist.")
}

// errProjectRequired reports a missing required field of a project payload.
func errProjectRequired(field string) error {
	return newErr(http.StatusBadRequest, "validation_failed", field+" is required.")
}

// errProjectValidation reports a malformed project payload with a message that
// names the offending field.
func errProjectValidation(message string) error {
	return newErr(http.StatusBadRequest, "validation_failed", message)
}

func errEducationNotFound() error {
	return newErr(http.StatusNotFound, CodeEducationNotFound, "This education entry does not exist.")
}

// errEducationRequired reports a missing required field of an education payload.
func errEducationRequired(field string) error {
	return newErr(http.StatusBadRequest, "validation_failed", field+" is required.")
}

// errEducationValidation reports a malformed education payload with a message
// that names the offending field.
func errEducationValidation(message string) error {
	return newErr(http.StatusBadRequest, "validation_failed", message)
}

func errExtraNotFound() error {
	return newErr(http.StatusNotFound, CodeExtraNotFound, "This extra entry does not exist.")
}

// errExtraRequired reports a missing required field of an extra payload.
func errExtraRequired(field string) error {
	return newErr(http.StatusBadRequest, "validation_failed", field+" is required.")
}

// errExtraValidation reports a malformed extra payload with a message that names
// the offending field.
func errExtraValidation(message string) error {
	return newErr(http.StatusBadRequest, "validation_failed", message)
}

// errSocialLinkValidation reports a malformed social link payload.
func errSocialLinkValidation(message string) error {
	return newErr(http.StatusBadRequest, "validation_failed", message)
}

// errUploadValidation reports an upload request that breaks a rule. The message
// names the offending field so the admin panel can point at the right input.
func errUploadValidation(message string) error {
	return newErr(http.StatusBadRequest, "validation_failed", message)
}

// errUploadRequired reports a missing required field of an upload request.
func errUploadRequired(field string) error {
	return newErr(http.StatusBadRequest, "validation_failed", field+" is required.")
}

// errUploadsDisabled reports that no bucket is configured. The rest of the API
// keeps working, so this is a 503 rather than a boot failure.
func errUploadsDisabled() error {
	return newErr(http.StatusServiceUnavailable, "uploads_disabled",
		"File uploads are not configured on this deployment.")
}

// errAuditValidation reports a malformed audit query.
func errAuditValidation(message string) error {
	return newErr(http.StatusBadRequest, "validation_failed", message)
}

// errSocialLinkRequired reports a missing required field of a social link.
func errSocialLinkRequired(field string) error {
	return newErr(http.StatusBadRequest, "validation_failed", field+" is required.")
}

// errSocialLinkExists reports a platform that appears twice in one payload. The
// UNIQUE index on social_links.platform would reject the write, but catching it
// here returns a precise message instead of a generic 409.
func errSocialLinkExists(platform string) error {
	return newErr(http.StatusConflict, CodeSocialLinkConflict,
		fmt.Sprintf("A social link for %q already exists in this payload.", platform))
}
