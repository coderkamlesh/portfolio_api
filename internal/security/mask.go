package security

import "strings"

// MaskEmail hides the local part of an address so OTP screens can confirm
// where the code went without disclosing the full address:
//
//	kamlesh.sharma@gmail.com -> k*************e@gmail.com
func MaskEmail(email string) string {
	at := strings.LastIndex(email, "@")
	if at <= 0 {
		return "***"
	}
	local, domain := email[:at], email[at:]
	switch len(local) {
	case 1:
		return local + "***" + domain
	case 2:
		return local[:1] + "***" + local[1:] + domain
	default:
		return local[:1] + strings.Repeat("*", len(local)-2) + local[len(local)-1:] + domain
	}
}
