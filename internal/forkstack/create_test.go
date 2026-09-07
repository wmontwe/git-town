package forkstack //nolint:testpackage

import (
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/git-town/git-town/v24/internal/config/configdomain"
	"github.com/git-town/git-town/v24/internal/forge/forgedomain"
	"github.com/git-town/git-town/v24/internal/git/gitdomain"
	"github.com/git-town/git-town/v24/internal/gohacks/stringslice"
	"github.com/git-town/git-town/v24/internal/test/forkstackrunner"
	. "github.com/git-town/git-town/v24/pkg/prelude"
	"github.com/shoenig/test/must"
)

func TestForkStackManagedBlock(t *testing.T) {
	t.Parallel()
	layers := []Layer{
		{Branch: "feature-root", LogicalBase: "main"},
		{Branch: "feature/api", LogicalBase: "feature-root"},
	}
	pullRequests := []forkStackPullRequest{
		{Number: 10, HTMLURL: "https://github.com/upstream/project/pull/10", Title: "Feature root"},
		{Number: 11, HTMLURL: "https://github.com/upstream/project/pull/11", Title: "Feature API"},
	}

	commits := gitdomain.Commits{
		{Message: "Add endpoint", SHA: "1111111111111111111111111111111111111111"},
		{Message: "Document endpoint", SHA: "2222222222222222222222222222222222222222"},
	}
	have := forkStackManagedBlock(1, layers, pullRequests, commits)

	must.StrContains(t, have, "[#10](https://github.com/upstream/project/pull/10)")
	must.StrContains(t, have, "**[#11](https://github.com/upstream/project/pull/11) Feature API ← current**")
	must.StrContains(t, have, "logical-base=feature-root")
	must.StrNotContains(t, have, "View stack changes")
	must.StrContains(t, have, "#### Commits to review")
	must.StrContains(t, have, "This PR includes earlier stacked changes because GitHub cannot use a branch from a fork as its base.")
	must.StrContains(t, have, "[`1111111`](https://github.com/upstream/project/pull/11/commits/1111111111111111111111111111111111111111) Add endpoint")
	must.StrContains(t, have, "[`2222222`](https://github.com/upstream/project/pull/11/commits/2222222222222222222222222222222222222222) Document endpoint")

	rootBlock := forkStackManagedBlock(0, layers, pullRequests, commits)
	must.StrNotContains(t, rootBlock, "View stack changes")
	must.StrNotContains(t, rootBlock, "Commits to review")
	must.StrNotContains(t, rootBlock, "/commits/")
	must.StrNotContains(t, rootBlock, "This PR includes earlier stacked changes")
}

func TestForkStackProposalCreatesOnlySelectedBranches(t *testing.T) {
	t.Parallel()
	createdBranches := gitdomain.LocalBranchNames{}
	runner := forkstackrunner.Runner{QueryFunc: func(executable string, args ...string) (string, error) {
		if executable == "git" {
			if slices.Contains(args, "--reverse") {
				return "1111111111111111111111111111111111111111\x00Commit\n", nil
			}
			return "Commit", nil
		}
		endpoint := args[3]
		switch {
		case endpoint == "repos/fork-owner/project":
			return `{"fork":true,"source":{"full_name":"upstream/project"}}`, nil
		case endpoint == "repos/upstream/project":
			return `{"default_branch":"main"}`, nil
		case endpoint == "repos/upstream/project/contents/.github", endpoint == "repos/upstream/project/contents", endpoint == "repos/upstream/project/contents/docs":
			return `[]`, nil
		case strings.Contains(endpoint, "/pulls?") && strings.Contains(endpoint, "feature-root"):
			return `[{"number":100,"html_url":"https://github.com/upstream/project/pull/100","title":"Root","body":"","base":{"ref":"main"},"head":{"ref":"feature-root","repo":{"full_name":"fork-owner/project"}}}]`, nil
		case strings.Contains(endpoint, "/pulls?"):
			return `[]`, nil
		case endpoint == "repos/upstream/project/pulls":
			for _, arg := range args {
				if head, hasHead := strings.CutPrefix(arg, "head=fork-owner:"); hasHead {
					createdBranches = append(createdBranches, gitdomain.LocalBranchName(head))
				}
			}
			return `{"number":101,"html_url":"https://github.com/upstream/project/pull/101","title":"Middle","body":"","base":{"ref":"main"}}`, nil
		case strings.Contains(endpoint, "/pulls/"):
			return `{}`, nil
		default:
			return "", fmt.Errorf("unexpected endpoint: %s", endpoint)
		}
	}}

	err := Create(CreateArgs{
		BranchesToPropose: gitdomain.LocalBranchNames{"feature-middle"},
		ForkRepository:    forgedomain.HostedRepoInfo{Hostname: "github.com", Organization: "fork-owner", Repository: "project"},
		Label:             None[configdomain.ForkStackLabel](),
		Layers: []Layer{
			{Branch: "feature-root", LogicalBase: "main"},
			{Branch: "feature-middle", LogicalBase: "feature-root"},
			{Branch: "unfinished-child", LogicalBase: "feature-middle"},
		},
		MainBranch:     "main",
		ProposalBody:   None[gitdomain.ProposalBody](),
		ProposalTitle:  None[gitdomain.ProposalTitle](),
		SelectedBranch: "feature-middle",
	}, runner, stringslice.NewCollector())

	must.NoError(t, err)
	must.Eq(t, gitdomain.LocalBranchNames{"feature-middle"}, createdBranches)
}

func TestForkStackProposalCreate(t *testing.T) {
	t.Parallel()
	created := 0
	labeledPullRequest := 0
	updatedBodies := []string{}
	runner := forkstackrunner.Runner{QueryFunc: func(executable string, args ...string) (string, error) {
		if executable == "git" {
			if slices.Contains(args, "--reverse") {
				commitRange := args[len(args)-1]
				switch commitRange {
				case "main..feature-root":
					return "1111111111111111111111111111111111111111\x00Add root feature\n", nil
				case "feature-root..feature-api":
					return "2222222222222222222222222222222222222222\x00Add API\n3333333333333333333333333333333333333333\x00Test API\n", nil
				}
			}
			return "", nil
		}
		must.EqOp(t, "gh", executable)
		endpoint := args[3]
		switch {
		case endpoint == "repos/fork-owner/project":
			return `{"fork":true,"full_name":"fork-owner/project","default_branch":"main","source":{"full_name":"upstream/project"}}`, nil
		case endpoint == "repos/upstream/project":
			return `{"fork":false,"full_name":"upstream/project","default_branch":"main"}`, nil
		case endpoint == "repos/upstream/project/contents/.github":
			return `[{"name":"PULL_REQUEST_TEMPLATE.md","path":".github/PULL_REQUEST_TEMPLATE.md","type":"file"}]`, nil
		case endpoint == "repos/upstream/project/contents/.github/PULL_REQUEST_TEMPLATE.md":
			return "Template content", nil
		case strings.Contains(endpoint, "/pulls?"):
			return `[]`, nil
		case endpoint == "repos/upstream/project/pulls":
			created++
			must.True(t, slices.Contains(args, "head_repo=project"))
			return fmt.Sprintf(`{"number":%d,"html_url":"https://github.com/upstream/project/pull/%d","title":"created","body":"","base":{"ref":"main"}}`, 100+created, 100+created), nil
		case endpoint == "repos/upstream/project/issues/102/labels":
			must.True(t, slices.Contains(args, "labels[]=stacked-change"))
			labeledPullRequest = 102
			return `[]`, nil
		case strings.Contains(endpoint, "/pulls/"):
			for _, arg := range args {
				if body, hasBody := strings.CutPrefix(arg, "body="); hasBody {
					updatedBodies = append(updatedBodies, body)
				}
			}
			return `{}`, nil
		default:
			return "", fmt.Errorf("unexpected endpoint: %s", endpoint)
		}
	}}
	opcode := CreateArgs{
		BranchesToPropose: gitdomain.LocalBranchNames{"feature-root", "feature-api"},
		ForkRepository:    forgedomain.HostedRepoInfo{Hostname: "github.com", Organization: "fork-owner", Repository: "project"},
		Label:             Some(configdomain.ForkStackLabel("stacked-change")),
		Layers: []Layer{
			{Branch: "feature-root", LogicalBase: "main"},
			{Branch: "feature-api", LogicalBase: "feature-root"},
		},
		MainBranch:     "main",
		ProposalBody:   Some(gitdomain.ProposalBody("User content")),
		ProposalTitle:  None[gitdomain.ProposalTitle](),
		SelectedBranch: "feature-api",
	}

	finalMessages := stringslice.NewCollector()
	err := Create(opcode, runner, finalMessages)

	must.NoError(t, err)
	must.Eq(t, []string{
		"Created or updated fork-compatible logical stack with 2 pull requests in upstream/project; merge bottom-first",
		"https://github.com/upstream/project/pull/101",
		"https://github.com/upstream/project/pull/102",
	}, finalMessages.Result())
	must.EqOp(t, 2, created)
	must.EqOp(t, 102, labeledPullRequest)
	must.Len(t, 2, updatedBodies)
	must.StrContains(t, updatedBodies[0], "Template content")
	must.StrNotContains(t, updatedBodies[0], "Commits to review")
	must.StrNotContains(t, updatedBodies[0], "/commits/")
	must.StrNotContains(t, updatedBodies[0], "This PR includes earlier stacked changes")
	must.StrContains(t, updatedBodies[1], "User content")
	must.StrContains(t, updatedBodies[1], "/pull/102/commits/2222222222222222222222222222222222222222")
	must.StrContains(t, updatedBodies[1], "/pull/102/commits/3333333333333333333333333333333333333333")
	must.StrNotContains(t, updatedBodies[1], "Template content")
	must.StrNotContains(t, updatedBodies[1], "1111111111111111111111111111111111111111")
}

func TestForkStackProposalUpdateDoesNotLoadTemplate(t *testing.T) {
	t.Parallel()
	updatedBody := ""
	runner := forkstackrunner.Runner{QueryFunc: func(executable string, args ...string) (string, error) {
		if executable == "git" {
			return "4444444444444444444444444444444444444444\x00Update feature\n", nil
		}
		must.EqOp(t, "gh", executable)
		endpoint := args[3]
		switch {
		case endpoint == "repos/fork-owner/project":
			return `{"fork":true,"source":{"full_name":"upstream/project"}}`, nil
		case endpoint == "repos/upstream/project":
			return `{"default_branch":"main"}`, nil
		case strings.Contains(endpoint, "/pulls?"):
			return `[{"number":100,"html_url":"https://github.com/upstream/project/pull/100","title":"Existing","body":"Existing body","base":{"ref":"main"},"head":{"ref":"feature","repo":{"full_name":"fork-owner/project"}}}]`, nil
		case endpoint == "repos/upstream/project/pulls/100":
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
	finalMessages := stringslice.NewCollector()
	opcode := CreateArgs{
		BranchesToPropose: gitdomain.LocalBranchNames{"feature"},
		ForkRepository:    forgedomain.HostedRepoInfo{Hostname: "github.com", Organization: "fork-owner", Repository: "project"},
		Label:             Some(configdomain.ForkStackLabel("stacked-change")),
		Layers:            []Layer{{Branch: "feature", LogicalBase: "main"}},
		MainBranch:        "main",
		ProposalBody:      Some(gitdomain.ProposalBody("Replacement body")),
		SelectedBranch:    "feature",
	}

	err := Create(opcode, runner, finalMessages)

	must.NoError(t, err)
	must.StrContains(t, updatedBody, "Existing body")
	must.StrNotContains(t, updatedBody, "Replacement body")
	must.StrContains(t, updatedBody, forkStackBlockStart)
	must.StrContains(t, updatedBody, forkStackBlockEnd)
	must.StrNotContains(t, updatedBody, "/commits/")
	must.StrNotContains(t, updatedBody, "This PR includes earlier stacked changes")
	must.Eq(t, []string{
		"Created or updated fork-compatible logical stack with 1 pull requests in upstream/project; merge bottom-first",
		"https://github.com/upstream/project/pull/100",
	}, finalMessages.Result())
}

func TestReplaceForkStackBlock(t *testing.T) {
	t.Parallel()
	original := gitdomain.ProposalBody("User content\n\n" + forkStackBlockStart + "\nold\n" + forkStackBlockEnd).String()

	have := replaceForkStackBlock(original, forkStackBlockStart+"\nnew\n"+forkStackBlockEnd)

	must.StrContains(t, have, "User content")
	must.StrContains(t, have, "new")
	must.StrNotContains(t, have, "\nold\n")
	must.EqOp(t, 1, strings.Count(have, forkStackBlockStart))
}
