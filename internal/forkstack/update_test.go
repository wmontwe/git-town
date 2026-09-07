package forkstack //nolint:testpackage

import (
	"fmt"
	"strings"
	"testing"

	"github.com/git-town/git-town/v24/internal/config/configdomain"
	"github.com/git-town/git-town/v24/internal/forge/forgedomain"
	"github.com/git-town/git-town/v24/internal/test/forkstackrunner"
	. "github.com/git-town/git-town/v24/pkg/prelude"
	"github.com/shoenig/test/must"
)

func TestPromoteNewStackRoot(t *testing.T) {
	t.Parallel()
	removedLabel := false
	updatedBody := ""
	existingBody := `User content

<!-- git-town-fork-stack:start -->
### Stack

- **[#11](https://github.com/upstream/project/pull/11) Child ← current**

<!-- git-town-fork-stack:logical-base=feature-root -->

#### Commits to review

- [old commit](https://github.com/upstream/project/pull/11/commits/aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa)

> This PR includes earlier stacked changes because GitHub cannot use a branch from a fork as its base.
<!-- git-town-fork-stack:end -->`
	runner := forkstackrunner.Runner{QueryFunc: func(executable string, args ...string) (string, error) {
		must.EqOp(t, "gh", executable)
		endpoint := args[3]
		switch {
		case endpoint == "repos/fork-owner/project":
			return `{"fork":true,"source":{"full_name":"upstream/project"}}`, nil
		case strings.Contains(endpoint, "/pulls?") && strings.Contains(endpoint, "feature-root"):
			return `[]`, nil
		case strings.Contains(endpoint, "/pulls?"):
			return fmt.Sprintf(`[{"number":11,"html_url":"https://github.com/upstream/project/pull/11","title":"Child","body":%q,"labels":[{"name":"stacked-change"}],"base":{"ref":"main"},"head":{"ref":"feature-child","repo":{"full_name":"fork-owner/project"}}}]`, existingBody), nil
		case endpoint == "repos/upstream/project/issues/11/labels/stacked-change":
			removedLabel = true
			return `{}`, nil
		case endpoint == "repos/upstream/project/pulls/11":
			for _, arg := range args {
				if body, hasBody := strings.CutPrefix(arg, "body="); hasBody {
					updatedBody = body
				}
			}
			return `{}`, nil
		default:
			return "", fmt.Errorf("unexpected endpoint: %s", endpoint)
		}
	}}

	err := Update(UpdateArgs{
		ForkRepository: forgedomain.HostedRepoInfo{Hostname: "github.com", Organization: "fork-owner", Repository: "project"},
		Label:          Some(configdomain.ForkStackLabel("stacked-change")),
		Layers: []Layer{
			{Branch: "feature-root", LogicalBase: "main"},
			{Branch: "feature-child", LogicalBase: "feature-root"},
		},
	}, runner)

	must.NoError(t, err)
	must.True(t, removedLabel)
	must.StrContains(t, updatedBody, "User content")
	must.StrNotContains(t, updatedBody, "Commits to review")
	must.StrNotContains(t, updatedBody, "/commits/")
	must.StrNotContains(t, updatedBody, "This PR includes earlier stacked changes")
}

func TestUpdateCommitLinks(t *testing.T) {
	t.Parallel()
	updatedBody := ""
	existingBody := `User content

<!-- git-town-fork-stack:start -->
### Stack

- [#10](https://github.com/upstream/project/pull/10) Root
- **[#11](https://github.com/upstream/project/pull/11) Child ← current**

<!-- git-town-fork-stack:logical-base=feature-root -->

#### Commits to review

- [old commit](https://github.com/upstream/project/pull/11/commits/aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa)

> This PR includes earlier stacked changes because GitHub cannot use a branch from a fork as its base.
<!-- git-town-fork-stack:end -->`
	runner := forkstackrunner.Runner{QueryFunc: func(executable string, args ...string) (string, error) {
		if executable == "git" {
			return "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb\x00Rebased commit\n", nil
		}
		must.EqOp(t, "gh", executable)
		endpoint := args[3]
		switch {
		case endpoint == "repos/fork-owner/project":
			return `{"fork":true,"source":{"full_name":"upstream/project"}}`, nil
		case strings.Contains(endpoint, "/pulls?") && strings.Contains(endpoint, "feature-root"):
			return `[{"number":10,"html_url":"https://github.com/upstream/project/pull/10","title":"Root","body":"","base":{"ref":"main"},"head":{"ref":"feature-root","repo":{"full_name":"fork-owner/project"}}}]`, nil
		case strings.Contains(endpoint, "/pulls?"):
			return fmt.Sprintf(`[{"number":11,"html_url":"https://github.com/upstream/project/pull/11","title":"Child","body":%q,"base":{"ref":"main"},"head":{"ref":"feature-child","repo":{"full_name":"fork-owner/project"}}}]`, existingBody), nil
		case endpoint == "repos/upstream/project/pulls/11":
			for _, arg := range args {
				if body, hasBody := strings.CutPrefix(arg, "body="); hasBody {
					updatedBody = body
				}
			}
			return `{}`, nil
		default:
			return "", fmt.Errorf("unexpected endpoint: %s", endpoint)
		}
	}}

	err := Update(UpdateArgs{
		ForkRepository: forgedomain.HostedRepoInfo{Hostname: "github.com", Organization: "fork-owner", Repository: "project"},
		Label:          None[configdomain.ForkStackLabel](),
		Layers: []Layer{
			{Branch: "feature-root", LogicalBase: "main"},
			{Branch: "feature-child", LogicalBase: "feature-root"},
		},
	}, runner)

	must.NoError(t, err)
	must.StrContains(t, updatedBody, "User content")
	must.StrContains(t, updatedBody, "[#10](https://github.com/upstream/project/pull/10) Root")
	must.StrContains(t, updatedBody, "/pull/11/commits/bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb")
	must.StrContains(t, updatedBody, "Rebased commit")
	must.StrNotContains(t, updatedBody, "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa")
}
