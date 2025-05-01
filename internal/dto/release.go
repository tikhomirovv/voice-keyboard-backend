package dto

type Release struct {
	Version   string `json:"version"`
	PubDate   string `json:"pub_date"`
	URL       string `json:"url"`
	Signature string `json:"signature"`
	Notes     string `json:"notes"`
}

type ReleasesOptions struct {
	Env            string `json:"env"`
	Target         string `json:"target"`
	Arch           string `json:"arch"`
	CurrentVersion string `json:"current_version"`
}
