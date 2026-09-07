package forkstack

import "github.com/git-town/git-town/v24/internal/git/gitdomain"

// Layer describes one fork-stack branch and its logical parent.
type Layer struct {
	Branch      gitdomain.LocalBranchName
	LogicalBase gitdomain.LocalBranchName
}
