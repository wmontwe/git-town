package forkstack

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strings"

	"github.com/git-town/git-town/v24/internal/config/configdomain"
	"github.com/git-town/git-town/v24/internal/forge/forgedomain"
	"github.com/git-town/git-town/v24/internal/git/gitdomain"
	"github.com/git-town/git-town/v24/internal/gohacks/stringslice"
	"github.com/git-town/git-town/v24/internal/gohacks/stringss"
	"github.com/git-town/git-town/v24/internal/subshell/subshelldomain"
	. "github.com/git-town/git-town/v24/pkg/prelude"
)

const (
	forkStackBlockStart = "<!-- git-town-fork-stack:start -->"
	forkStackBlockEnd   = "<!-- git-town-fork-stack:end -->"
)

// CreateArgs contains the inputs for creating a fork-compatible stack.
type CreateArgs struct {
	BranchesToPropose gitdomain.LocalBranchNames
	ForkRepository    forgedomain.HostedRepoInfo
	Label             Option[configdomain.ForkStackLabel]
	Layers            []Layer
	MainBranch        gitdomain.LocalBranchName
	ProposalBody      Option[gitdomain.ProposalBody]
	ProposalTitle     Option[gitdomain.ProposalTitle]
	SelectedBranch    gitdomain.LocalBranchName
}

func Create(self CreateArgs, backend subshelldomain.RunnerQuerier, finalMessages stringslice.Collector) error {
	if self.ForkRepository.HostnameWithStandardPort() != "github.com" {
		return errors.New("fork-stack proposals currently support github.com repositories only")
	}
	if len(self.Layers) == 0 {
		return nil
	}

	forkSlug := self.ForkRepository.Organization + "/" + self.ForkRepository.Repository
	var fork forkStackRepository
	if err := self.queryJSON(backend, &fork, "repos/"+forkSlug); err != nil {
		return fmt.Errorf("inspect fork repository %s: %w", forkSlug, err)
	}
	if !fork.Fork || fork.Source == nil || fork.Source.FullName == "" {
		return fmt.Errorf("%s is not a GitHub fork; configure hosting.dev-remote to point at the contributor fork", forkSlug)
	}
	upstreamSlug := fork.Source.FullName
	var upstream forkStackRepository
	if err := self.queryJSON(backend, &upstream, "repos/"+upstreamSlug); err != nil {
		return fmt.Errorf("inspect upstream repository %s: %w", upstreamSlug, err)
	}
	if upstream.DefaultBranch == "" {
		return fmt.Errorf("cannot determine the default branch of upstream repository %s", upstreamSlug)
	}
	pullRequestTemplate := ""
	hasLoadedPullRequestTemplate := false
	layers := make([]Layer, 0, len(self.Layers))
	pullRequests := make([]forkStackPullRequest, 0, len(self.Layers))
	for _, layer := range self.Layers {
		if layer.LogicalBase != self.MainBranch {
			if _, err := backend.Query("git", "merge-base", "--is-ancestor", layer.LogicalBase.String(), layer.Branch.String()); err != nil {
				return fmt.Errorf("logical parent %q is not an ancestor of %q; run git town sync --stack and retry", layer.LogicalBase, layer.Branch)
			}
		}
		existing, err := self.findPullRequest(backend, upstreamSlug, forkSlug, layer.Branch)
		if err != nil {
			return err
		}
		if existing != nil {
			if existing.Base.Ref != upstream.DefaultBranch {
				if err := self.patchPullRequest(backend, upstreamSlug, existing.Number, "base", upstream.DefaultBranch); err != nil {
					return fmt.Errorf("update base of pull request #%d: %w", existing.Number, err)
				}
				existing.Base.Ref = upstream.DefaultBranch
			}
			layers = append(layers, layer)
			pullRequests = append(pullRequests, *existing)
			continue
		}
		if !self.BranchesToPropose.Contains(layer.Branch) {
			continue
		}

		title := self.pullRequestTitle(backend, layer)
		if layer.Branch == self.SelectedBranch {
			title = self.ProposalTitle.GetOr(gitdomain.ProposalTitle(title)).String()
		}
		if !hasLoadedPullRequestTemplate {
			pullRequestTemplate, err = self.pullRequestTemplate(backend, upstreamSlug)
			if err != nil {
				return err
			}
			hasLoadedPullRequestTemplate = true
		}
		body := pullRequestTemplate
		if proposalBody, hasProposalBody := self.ProposalBody.Get(); layer.Branch == self.SelectedBranch && hasProposalBody {
			body = proposalBody.String()
		}
		created, err := self.createPullRequest(backend, upstreamSlug, forkSlug, upstream.DefaultBranch, layer.Branch, title, body)
		if err != nil {
			return err
		}
		layers = append(layers, layer)
		pullRequests = append(pullRequests, created)
	}

	for layerIndex, layer := range layers {
		if label, hasLabel := self.Label.Get(); hasLabel {
			if layerIndex == 0 && pullRequestHasLabel(pullRequests[layerIndex], label) {
				if err := self.removePullRequestLabel(backend, upstreamSlug, pullRequests[layerIndex].Number, label); err != nil {
					return fmt.Errorf("remove label from pull request #%d: %w", pullRequests[layerIndex].Number, err)
				}
			} else if layerIndex > 0 {
				if err := self.addPullRequestLabel(backend, upstreamSlug, pullRequests[layerIndex].Number, label); err != nil {
					return fmt.Errorf("label pull request #%d: %w", pullRequests[layerIndex].Number, err)
				}
			}
		}
		commits, err := commitsInLayer(backend, layer)
		if err != nil {
			return err
		}
		block := forkStackManagedBlock(layerIndex, layers, pullRequests, commits)
		body := replaceForkStackBlock(pullRequests[layerIndex].Body, block)
		if err := self.patchPullRequest(backend, upstreamSlug, pullRequests[layerIndex].Number, "body", body); err != nil {
			return fmt.Errorf("update body of pull request #%d: %w", pullRequests[layerIndex].Number, err)
		}
	}

	finalMessages.Addf("Created or updated fork-compatible logical stack with %d pull requests in %s; merge bottom-first", len(pullRequests), upstreamSlug)
	for _, pullRequest := range pullRequests {
		finalMessages.Add(pullRequest.HTMLURL)
	}
	return nil
}

func (self CreateArgs) addPullRequestLabel(backend subshelldomain.RunnerQuerier, upstreamSlug string, number int, label configdomain.ForkStackLabel) error {
	_, err := backend.Query("gh", "api", "--hostname", self.ForkRepository.HostnameWithStandardPort(),
		"repos/"+upstreamSlug+fmt.Sprintf("/issues/%d/labels", number), "--method", "POST", "-f", "labels[]="+label.String())
	return err
}

func (self CreateArgs) createPullRequest(backend subshelldomain.RunnerQuerier, upstreamSlug, forkSlug, base string, branch gitdomain.LocalBranchName, title, body string) (forkStackPullRequest, error) {
	var result forkStackPullRequest
	qualifiedHead := self.ForkRepository.Organization + ":" + branch.String()
	err := self.queryJSON(backend, &result, "repos/"+upstreamSlug+"/pulls",
		"--method", "POST",
		"-f", "base="+base,
		"-f", "body="+body,
		"-f", "head="+qualifiedHead,
		"-f", "head_repo="+self.ForkRepository.Repository,
		"-f", "title="+title)
	if err != nil {
		return result, fmt.Errorf("create pull request for %s in %s: %w", qualifiedHead, upstreamSlug, err)
	}
	if result.Head.Repo == nil {
		result.Head.Repo = &struct {
			FullName string `json:"full_name"` //nolint:tagliatelle
		}{FullName: forkSlug}
	}
	result.Head.Ref = branch.String()
	result.Base.Ref = base
	result.Body = body
	result.Title = title
	return result, nil
}

func (self CreateArgs) findPullRequest(backend subshelldomain.RunnerQuerier, upstreamSlug, forkSlug string, branch gitdomain.LocalBranchName) (*forkStackPullRequest, error) {
	qualifiedHead := self.ForkRepository.Organization + ":" + branch.String()
	endpoint := "repos/" + upstreamSlug + "/pulls?state=open&per_page=100&head=" + url.QueryEscape(qualifiedHead)
	var pullRequests []forkStackPullRequest
	if err := self.queryJSON(backend, &pullRequests, endpoint); err != nil {
		return nil, fmt.Errorf("find pull request for %s: %w", qualifiedHead, err)
	}
	for pullRequestIndex := range pullRequests {
		pullRequest := &pullRequests[pullRequestIndex]
		if pullRequest.Head.Repo != nil && strings.EqualFold(pullRequest.Head.Repo.FullName, forkSlug) && pullRequest.Head.Ref == branch.String() {
			return pullRequest, nil
		}
	}
	return nil, nil
}

func commitsInLayer(backend subshelldomain.Querier, layer Layer) (gitdomain.Commits, error) {
	commitRange := layer.LogicalBase.String() + ".." + layer.Branch.String()
	output, err := backend.Query("git", "log", "--reverse", "--format=%H%x00%s", commitRange)
	if err != nil {
		return nil, fmt.Errorf("list commits in stack layer %s: %w", layer.Branch, err)
	}
	lines := stringslice.NonEmptyLines(output)
	result := make(gitdomain.Commits, 0, len(lines))
	for _, line := range lines {
		shaText, subject, hasSeparator := strings.Cut(line, "\x00")
		if !hasSeparator {
			return nil, fmt.Errorf("parse commit in stack layer %s: malformed git log output", layer.Branch)
		}
		sha, err := gitdomain.NewSHA(stringss.Trimmed(shaText))
		if err != nil {
			return nil, fmt.Errorf("parse commit in stack layer %s: %w", layer.Branch, err)
		}
		result = append(result, gitdomain.Commit{Message: gitdomain.CommitMessage(strings.TrimSpace(subject)), SHA: sha})
	}
	return result, nil
}

func forkStackManagedBlock(current int, layers []Layer, pullRequests []forkStackPullRequest, commits gitdomain.Commits) string {
	var result strings.Builder
	result.WriteString(forkStackBlockStart + "\n### Stack\n\n")
	for layerIndex, pullRequest := range pullRequests {
		line := fmt.Sprintf("- [#%d](%s) %s", pullRequest.Number, pullRequest.HTMLURL, pullRequest.Title)
		if layerIndex == current {
			line = "- **" + strings.TrimPrefix(line, "- ") + " ← current**"
		}
		result.WriteString(line + "\n")
	}
	logicalBase := layers[current].LogicalBase.String()
	result.WriteString("\n<!-- git-town-fork-stack:logical-base=" + logicalBase + " -->\n")
	result.WriteString(forkStackReviewSection(pullRequests[current].HTMLURL, commits, current == 0))
	result.WriteString("\n" + forkStackBlockEnd)
	return result.String()
}

func forkStackReviewSection(pullRequestURL string, commits gitdomain.Commits, isRoot bool) string {
	if isRoot {
		return ""
	}
	var result strings.Builder
	if len(commits) > 0 {
		result.WriteString("\n#### Commits to review\n\n")
		for _, commit := range commits {
			commitURL := pullRequestURL + "/commits/" + commit.SHA.String()
			result.WriteString(fmt.Sprintf("- [`%s`](%s) %s\n", commit.SHA.String()[:7], commitURL, commit.Message.Parts().Title))
		}
	}
	result.WriteString("\n> This PR includes earlier stacked changes because GitHub cannot use a branch from a fork as its base.")
	return result.String()
}

func forkStackTitle(branch gitdomain.LocalBranchName) string {
	return strings.NewReplacer("-", " ", "_", " ").Replace(branch.String())
}

func (self CreateArgs) patchPullRequest(backend subshelldomain.RunnerQuerier, upstreamSlug string, number int, field, value string) error {
	_, err := backend.Query("gh", "api", "--hostname", self.ForkRepository.HostnameWithStandardPort(),
		"repos/"+upstreamSlug+fmt.Sprintf("/pulls/%d", number), "--method", "PATCH", "-f", field+"="+value)
	return err
}

func (self CreateArgs) pullRequestTitle(backend subshelldomain.RunnerQuerier, layer Layer) string {
	output, err := backend.Query("git", "log", "--format=%s", layer.LogicalBase.String()+".."+layer.Branch.String())
	if err == nil {
		subjects := strings.FieldsFunc(strings.TrimSpace(output), func(character rune) bool { return character == '\n' || character == '\r' })
		if len(subjects) == 1 {
			return subjects[0]
		}
	}
	return forkStackTitle(layer.Branch)
}

func (self CreateArgs) pullRequestTemplate(backend subshelldomain.RunnerQuerier, upstreamSlug string) (string, error) {
	for _, directory := range []string{".github", "", "docs"} {
		endpoint := "repos/" + upstreamSlug + "/contents"
		if directory != "" {
			endpoint += "/" + directory
		}
		var contents []repositoryContent
		if err := self.queryJSON(backend, &contents, endpoint); err != nil {
			continue
		}
		for _, content := range contents {
			if content.Type != "file" || !strings.EqualFold(content.Name, "pull_request_template.md") {
				continue
			}
			template, err := backend.Query("gh", "api", "--hostname", self.ForkRepository.HostnameWithStandardPort(),
				"repos/"+upstreamSlug+"/contents/"+content.Path,
				"-H", "Accept: application/vnd.github.raw+json")
			if err != nil {
				return "", fmt.Errorf("read pull request template: %w", err)
			}
			return template, nil
		}
	}
	return "", nil
}

func (self CreateArgs) queryJSON(backend subshelldomain.RunnerQuerier, destination any, endpoint string, additionalArgs ...string) error {
	commandArgs := make([]string, 0, 4+len(additionalArgs))
	commandArgs = append(commandArgs, "api", "--hostname", self.ForkRepository.HostnameWithStandardPort(), endpoint)
	commandArgs = append(commandArgs, additionalArgs...)
	output, err := backend.Query("gh", commandArgs...)
	if err != nil {
		return err
	}
	return json.Unmarshal([]byte(output), destination)
}

func replaceForkStackBlock(body, block string) string {
	before, after, hasStart := strings.Cut(body, forkStackBlockStart)
	if hasStart {
		_, suffix, hasEnd := strings.Cut(after, forkStackBlockEnd)
		if hasEnd {
			before = strings.TrimSpace(before)
			suffix = strings.TrimSpace(suffix)
			body = strings.TrimSpace(strings.Join([]string{before, suffix}, "\n\n"))
		}
	}
	body = strings.TrimSpace(body)
	if body == "" {
		return block
	}
	return body + "\n\n" + block
}
