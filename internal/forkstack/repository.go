package forkstack

type forkStackRepository struct {
	DefaultBranch string `json:"default_branch"` //nolint:tagliatelle
	Fork          bool   `json:"fork"`
	FullName      string `json:"full_name"` //nolint:tagliatelle
	Source        *struct {
		FullName string `json:"full_name"` //nolint:tagliatelle
	} `json:"source"`
}
