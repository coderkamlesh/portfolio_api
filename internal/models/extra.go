package models

import "time"

// Extra maps extras. IssuedDate is a calendar date stored either as YYYY-MM or
// as YYYY-MM-DD, because a certification is usually month-level while an award
// carries a full date. The table has no updated_at column.
type Extra struct {
	ID            string
	Category      string
	Title         string
	Issuer        string
	IssuedDate    string
	CredentialURL string
	Description   string
	DisplayOrder  int
	CreatedAt     time.Time
}

// Extra categories accepted in extras.category, taken from the column comment in
// db_schema.sql.
const (
	ExtraCategoryCertification = "CERTIFICATION"
	ExtraCategoryAward         = "AWARD"
	ExtraCategoryPublication   = "PUBLICATION"
	ExtraCategoryOpenSource    = "OPEN_SOURCE"
	ExtraCategoryTalk          = "TALK"
	ExtraCategoryVolunteer     = "VOLUNTEER"
)

// ExtraCategories lists every accepted category in a stable order. It is also
// the grouping order of the public endpoint.
var ExtraCategories = []string{
	ExtraCategoryCertification,
	ExtraCategoryAward,
	ExtraCategoryPublication,
	ExtraCategoryOpenSource,
	ExtraCategoryTalk,
	ExtraCategoryVolunteer,
}