package forkstack

import (
	"errors"
	"fmt"
	"net/url"
	"strings"

	"github.com/git-town/git-town/v24/internal/config/configdomain"
	"github.com/git-town/git-town/v24/internal/git/gitdomain"
	"github.com/git-town/git-town/v24/internal/subshell/subshelldomain"
)

// Update refreshes metadata in existing fork-stack pull requests.
func Update(args UpdateArgs, backend subshelldomain.RunnerQuerier) error {
	if args.ForkRepository.HostnameWithStandardPort() != "github.com" {
		return errors.New("fork-stack proposals currently support github.com repositories only")
	}
	if len(args.Layers) == 0 {
		return nil
	}

	createArgs := CreateArgs{ForkRepository: args.ForkRepository} //exhaustruct:ignore
	forkSlug := args.ForkRepository.Organization + "/" + args.ForkRepository.Repository
	var fork forkStackRepository
	if err := createArgs.queryJSON(backend, &fork, "repos/"+forkSlug); err != nil {
		return fmt.Errorf("inspect fork repository %s: %w", forkSlug, err)
	}
	if !fork.Fork || fork.Source == nil || fork.Source.FullName == "" {
		return fmt.Errorf("%s is not a GitHub fork; configure hosting.dev-remote to point at the contributor fork", forkSlug)
	}
	upstreamSlug := fork.Source.FullName

	layersWithPullRequests := make([]Layer, 0, len(args.Layers))
	pullRequests := make([]forkStackPullRequest, 0, len(args.Layers))
	for _, layer := range args.Layers {
		pullRequest, err := createArgs.findPullRequest(backend, upstreamSlug, forkSlug, layer.Branch)
		if err != nil {
			return err
		}
		if pullRequest != nil {
			layersWithPullRequests = append(layersWithPullRequests, layer)
			pullRequests = append(pullRequests, *pullRequest)
		}
	}

	layersByBranch := make(map[gitdomain.LocalBranchName]Layer, len(args.Layers))
	for _, layer := range args.Layers {
		layersByBranch[layer.Branch] = layer
	}
	branchesWithPullRequests := make(map[gitdomain.LocalBranchName]struct{}, len(layersWithPullRequests))
	for _, layer := range layersWithPullRequests {
		branchesWithPullRequests[layer.Branch] = struct{}{}
	}
	for pullRequestIndex, pullRequest := range pullRequests {
		layer := layersWithPullRequests[pullRequestIndex]
		isRoot := !hasOpenAncestor(layer, layersByBranch, branchesWithPullRequests)
		if label, hasLabel := args.Label.Get(); hasLabel {
			if isRoot && pullRequestHasLabel(pullRequest, label) {
				if err := createArgs.removePullRequestLabel(backend, upstreamSlug, pullRequest.Number, label); err != nil {
					return fmt.Errorf("remove label from pull request #%d: %w", pullRequest.Number, err)
				}
			} else if !isRoot {
				if err := createArgs.addPullRequestLabel(backend, upstreamSlug, pullRequest.Number, label); err != nil {
					return fmt.Errorf("label pull request #%d: %w", pullRequest.Number, err)
				}
			}
		}
		commits := gitdomain.Commits{}
		if !isRoot {
			var err error
			commits, err = commitsInLayer(backend, layer)
			if err != nil {
				return err
			}
		}
		body, hasManagedBlock := replaceForkStackReviewSection(
			pullRequest.Body,
			forkStackReviewSection(pullRequest.HTMLURL, commits, isRoot),
		)
		if !hasManagedBlock || body == pullRequest.Body {
			continue
		}
		if err := createArgs.patchPullRequest(backend, upstreamSlug, pullRequest.Number, "body", body); err != nil {
			return fmt.Errorf("update body of pull request #%d: %w", pullRequest.Number, err)
		}
	}
	return nil
}

func hasOpenAncestor(layer Layer, layersByBranch map[gitdomain.LocalBranchName]Layer, branchesWithPullRequests map[gitdomain.LocalBranchName]struct{}) bool {
	ancestor := layer.LogicalBase
	for {
		if _, hasPullRequest := branchesWithPullRequests[ancestor]; hasPullRequest {
			return true
		}
		ancestorLayer, hasAncestorLayer := layersByBranch[ancestor]
		if !hasAncestorLayer {
			return false
		}
		ancestor = ancestorLayer.LogicalBase
	}
}

func pullRequestHasLabel(pullRequest forkStackPullRequest, label configdomain.ForkStackLabel) bool {
	for _, pullRequestLabel := range pullRequest.Labels {
		if strings.EqualFold(pullRequestLabel.Name, label.String()) {
			return true
		}
	}
	return false
}

func (self CreateArgs) removePullRequestLabel(backend subshelldomain.RunnerQuerier, upstreamSlug string, number int, label configdomain.ForkStackLabel) error {
	_, err := backend.Query("gh", "api", "--hostname", self.ForkRepository.HostnameWithStandardPort(),
		"repos/"+upstreamSlug+fmt.Sprintf("/issues/%d/labels/", number)+url.PathEscape(label.String()), "--method", "DELETE")
	return err
}

func replaceForkStackReviewSection(body, reviewSection string) (string, bool) {
	blockStart := strings.Index(body, forkStackBlockStart)
	if blockStart < 0 {
		return body, false
	}
	blockEndOffset := strings.Index(body[blockStart:], forkStackBlockEnd)
	if blockEndOffset < 0 {
		return body, false
	}
	blockEnd := blockStart + blockEndOffset
	logicalBaseOffset := strings.Index(body[blockStart:blockEnd], "<!-- git-town-fork-stack:logical-base=")
	if logicalBaseOffset < 0 {
		return body, false
	}
	logicalBaseStart := blockStart + logicalBaseOffset
	logicalBaseEndOffset := strings.Index(body[logicalBaseStart:blockEnd], "-->")
	if logicalBaseEndOffset < 0 {
		return body, false
	}
	logicalBaseEnd := logicalBaseStart + logicalBaseEndOffset + len("-->")
	return body[:logicalBaseEnd] + reviewSection + "\n" + body[blockEnd:], true
}
