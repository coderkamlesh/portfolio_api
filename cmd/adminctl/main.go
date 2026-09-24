// Command adminctl bootstraps and maintains admin accounts directly in Turso.
// It is the only way to create the first admin, since the public API has no
// sign-up endpoint.
//
// Usage (run from the repository root so .env is picked up):
//
//	go run ./cmd/adminctl -action=create -username=kamlesh -email=me@example.com -password='...'
//	go run ./cmd/adminctl -action=list
//	go run ./cmd/adminctl -action=reset-password -username=kamlesh
//	go run ./cmd/adminctl -action=enable-2fa -username=kamlesh
//	go run ./cmd/adminctl -action=disable-2fa -username=kamlesh   # emergency unlock
//	go run ./cmd/adminctl -action=deactivate -username=kamlesh
//
// Tip: omit -password and it is read from stdin instead, which keeps it out of
// your shell history:  echo 'my-secret-passphrase 1' | go run ./cmd/adminctl ...
package main

import (
	"bufio"
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"net/mail"
	"os"
	"strings"
	"time"

	"github.com/coderkamlesh/portfolio_api/internal/database"
	"github.com/coderkamlesh/portfolio_api/internal/models"
	"github.com/coderkamlesh/portfolio_api/internal/pkg/ids"
	"github.com/coderkamlesh/portfolio_api/internal/repository"
	"github.com/coderkamlesh/portfolio_api/internal/security"
	"github.com/joho/godotenv"
)

func main() {
	log.SetFlags(0)

	action := flag.String("action", "create",
		"one of: create, list, reset-password, activate, deactivate, enable-2fa, disable-2fa")
	username := flag.String("username", "", "admin username")
	email := flag.String("email", "", "admin email (required for create)")
	password := flag.String("password", "", "admin password (omit to read it from stdin)")
	flag.Parse()

	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found, using environment variables")
	}
	url, token := os.Getenv("TURSO_DATABASE_URL"), os.Getenv("TURSO_AUTH_TOKEN")
	if url == "" || token == "" {
		log.Fatal("❌ TURSO_DATABASE_URL and TURSO_AUTH_TOKEN must be set")
	}

	ctx := context.Background()
	db, err := database.Connect(url, token)
	if err != nil {
		log.Fatalf("❌ DB connect failed: %v", err)
	}
	defer db.Close()

	admins := repository.NewAdminRepository(db)
	twoFA := repository.NewTwoFARepository(db)

	if err := run(ctx, *action, admins, twoFA, *username, *email, *password); err != nil {
		log.Fatalf("❌ %s: %v", *action, err)
	}
}

// run dispatches the requested action.
func run(ctx context.Context, action string, admins *repository.AdminRepository,
	twoFA *repository.TwoFARepository, username, email, password string) error {

	switch action {
	case "create":
		return createAdmin(ctx, admins, username, email, password)
	case "list":
		return listAdmins(ctx, admins)
	case "reset-password":
		return resetPassword(ctx, admins, username, password)
	case "activate", "deactivate":
		return setActive(ctx, admins, username, action == "activate")
	case "enable-2fa", "disable-2fa":
		return setTwoFA(ctx, twoFA, admins, username, action == "enable-2fa")
	default:
		return fmt.Errorf("unknown -action %q", action)
	}
}

// createAdmin inserts a new admin (or resets the credentials of an existing
// username, making the command idempotent for provisioning scripts).
func createAdmin(ctx context.Context, admins *repository.AdminRepository, username, email, password string) error {
	username = strings.TrimSpace(username)
	email = strings.ToLower(strings.TrimSpace(email))
	if username == "" {
		return errors.New("-username is required")
	}
	if _, err := mail.ParseAddress(email); err != nil {
		return fmt.Errorf("-email must be a valid address: %w", err)
	}

	password, err := resolvePassword(password)
	if err != nil {
		return err
	}
	if err := security.ValidatePassword(password, 12); err != nil {
		return fmt.Errorf("weak password (must %s)", err)
	}

	hash, err := security.HashPassword(password)
	if err != nil {
		return err
	}

	now := time.Now().UTC()
	account := &models.AdminUser{
		ID:           ids.New(),
		Username:     username,
		Email:        email,
		PasswordHash: hash,
		IsActive:     true,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	if err := admins.Upsert(ctx, account); err != nil {
		return err
	}

	fmt.Printf("✅ Admin %q <%s> saved (argon2id). 2FA=EMAIL_OTP applies on login.\n", username, email)
	return nil
}

// listAdmins prints every admin account.
func listAdmins(ctx context.Context, admins *repository.AdminRepository) error {
	accounts, err := admins.List(ctx)
	if err != nil {
		return err
	}
	if len(accounts) == 0 {
		fmt.Println("ℹ️  No admin accounts yet. Create one with -action=create.")
		return nil
	}

	fmt.Printf("%-38s %-16s %-30s %-8s %s\n", "ID", "USERNAME", "EMAIL", "ACTIVE", "LAST LOGIN")
	for _, a := range accounts {
		lastLogin := "never"
		if a.LastLoginAt != nil {
			lastLogin = a.LastLoginAt.UTC().Format(time.RFC3339)
		}
		fmt.Printf("%-38s %-16s %-30s %-8t %s\n", a.ID, a.Username, a.Email, a.IsActive, lastLogin)
	}
	return nil
}

// resetPassword rotates the password of an existing admin.
func resetPassword(ctx context.Context, admins *repository.AdminRepository, username, password string) error {
	account, err := findAdmin(ctx, admins, username)
	if err != nil {
		return err
	}

	password, err = resolvePassword(password)
	if err != nil {
		return err
	}
	if err := security.ValidatePassword(password, 12); err != nil {
		return fmt.Errorf("weak password (must %s)", err)
	}

	hash, err := security.HashPassword(password)
	if err != nil {
		return err
	}
	if err := admins.UpdatePassword(ctx, account.ID, hash, time.Now().UTC()); err != nil {
		return err
	}
	fmt.Printf("✅ Password updated for %q. Existing sessions die on their next refresh.\n", account.Username)
	return nil
}

// setActive enables or disables an account.
func setActive(ctx context.Context, admins *repository.AdminRepository, username string, active bool) error {
	account, err := findAdmin(ctx, admins, username)
	if err != nil {
		return err
	}
	if err := admins.SetActive(ctx, account.ID, active, time.Now().UTC()); err != nil {
		return err
	}
	fmt.Printf("✅ %q is_active=%t\n", account.Username, active)
	return nil
}

// setTwoFA flips the EMAIL_OTP row so an admin can be unlocked out-of-band when
// SES or the mailbox is unavailable.
func setTwoFA(ctx context.Context, twoFA *repository.TwoFARepository, admins *repository.AdminRepository,
	username string, enabled bool) error {

	account, err := findAdmin(ctx, admins, username)
	if err != nil {
		return err
	}

	if !enabled {
		if err := twoFA.SetEnabled(ctx, account.ID, models.TwoFAMethodEmailOTP, false, time.Now().UTC()); err != nil {
			if errors.Is(err, models.ErrNotFound) {
				fmt.Printf("ℹ️  %q has no 2FA row to disable.\n", account.Username)
				return nil
			}
			return err
		}
		fmt.Printf("⚠️  Email OTP disabled for %q. Re-enable before exposing the panel!\n", account.Username)
		return nil
	}

	now := time.Now().UTC()
	if err := twoFA.Upsert(ctx, &models.TwoFactorConfig{
		ID:          ids.New(),
		AdminID:     account.ID,
		Method:      models.TwoFAMethodEmailOTP,
		IsEnabled:   true,
		ConfirmedAt: &now,
		CreatedAt:   now,
	}); err != nil {
		return err
	}
	fmt.Printf("✅ Email OTP 2FA enabled for %q.\n", account.Username)
	return nil
}

// findAdmin resolves -username or -email to an account.
func findAdmin(ctx context.Context, admins *repository.AdminRepository, identifier string) (*models.AdminUser, error) {
	identifier = strings.TrimSpace(identifier)
	if identifier == "" {
		return nil, errors.New("-username (or -email) is required")
	}

	account, err := admins.FindByIdentifier(ctx, identifier)
	if err != nil {
		if errors.Is(err, models.ErrNotFound) {
			return nil, fmt.Errorf("no admin found for %q", identifier)
		}
		return nil, err
	}
	return account, nil
}

// resolvePassword returns the -password flag value, or reads one line from
// stdin so the password never has to appear in the shell history.
func resolvePassword(flagValue string) (string, error) {
	if flagValue != "" {
		return flagValue, nil
	}

	fmt.Fprint(os.Stderr, "Password (echoed by your terminal; pipe it in to keep it out of history): ")
	reader := bufio.NewReader(os.Stdin)
	line, err := reader.ReadString('\n')
	if err != nil && line == "" {
		return "", fmt.Errorf("could not read password from stdin: %w", err)
	}
	password := strings.TrimRight(line, "\r\n")
	if password == "" {
		return "", errors.New("password must not be empty")
	}
	return password, nil
}
