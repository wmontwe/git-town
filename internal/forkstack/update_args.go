package forkstack

import (
	"github.com/git-town/git-town/v24/internal/config/configdomain"
	"github.com/git-town/git-town/v24/internal/forge/forgedomain"
	. "github.com/git-town/git-town/v24/pkg/prelude"
)

// UpdateArgs contains the inputs for updating fork-stack review links.
type UpdateArgs struct {
	ForkRepository forgedomain.HostedRepoInfo
	Label          Option[configdomain.ForkStackLabel]
	Layers         []Layer
}
