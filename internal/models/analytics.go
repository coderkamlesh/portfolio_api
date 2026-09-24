package models

import "time"

// ResumeDownload maps resume_downloads. IPHash is never a raw address: it is an
// HMAC of the IP keyed by a dedicated secret, so a leaked table cannot be
// reversed into visitor addresses while still counting the same visitor together
// across requests.
type ResumeDownload struct {
	ID           string
	DownloadedAt time.Time
	IPHash       string
	UserAgent    string
	Referrer     string
}

// DownloadPoint is one day in the download chart.
type DownloadPoint struct {
	Date  string `json:"date"`
	Count int64  `json:"count"`
}

// DownloadSummary is the aggregate view shown by the admin dashboard.
type DownloadSummary struct {
	Total     int64           `json:"total"`
	UniqueIPs int64           `json:"unique_ips"`
	ByDay     []DownloadPoint `json:"by_day"`
	From      string          `json:"from"`
	To        string          `json:"to"`
}
