package models

// SocialLink maps social_links. The table stores only id, platform, url and
// display_order — there are no created_at or updated_at columns, so the admin
// response carries no timestamps for this module.
type SocialLink struct {
	ID           string
	Platform     string
	URL          string
	DisplayOrder int
}

// Social platforms accepted in social_links.platform, taken from the column
// comment in db_schema.sql. Unlike the other content modules these stay
// lowercase, because the schema lists them that way and the values are matched
// case-insensitively on write.
const (
	SocialPlatformLinkedIn = "linkedin"
	SocialPlatformGitHub   = "github"
	SocialPlatformTwitter  = "twitter"
	SocialPlatformMedium   = "medium"
)

// SocialPlatforms lists every accepted platform in a stable order. It is also
// the display order of the public endpoint.
var SocialPlatforms = []string{
	SocialPlatformLinkedIn,
	SocialPlatformGitHub,
	SocialPlatformTwitter,
	SocialPlatformMedium,
}