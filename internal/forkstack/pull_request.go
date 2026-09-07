package forkstack

type forkStackPullRequest struct {
	Base struct {
		Ref string `json:"ref"`
	} `json:"base"`
	Body    string `json:"body"`
	HTMLURL string `json:"html_url"` //nolint:tagliatelle
	Head    struct {
		Ref  string `json:"ref"`
		Repo *struct {
			FullName string `json:"full_name"` //nolint:tagliatelle
		} `json:"repo"`
	} `json:"head"`
	Labels []forkStackPullRequestLabel `json:"labels"`
	Number int                         `json:"number"`
	Title  string                      `json:"title"`
}
