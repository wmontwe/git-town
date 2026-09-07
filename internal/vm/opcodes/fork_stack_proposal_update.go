package opcodes

import (
	"github.com/git-town/git-town/v24/internal/config/configdomain"
	"github.com/git-town/git-town/v24/internal/forge/forgedomain"
	"github.com/git-town/git-town/v24/internal/forkstack"
	"github.com/git-town/git-town/v24/internal/vm/shared"
	. "github.com/git-town/git-town/v24/pkg/prelude"
)

// ForkStackProposalUpdate refreshes commit links in existing fork-stack pull requests.
type ForkStackProposalUpdate struct {
	ForkRepository forgedomain.HostedRepoInfo
	Label          Option[configdomain.ForkStackLabel]
	Layers         []forkstack.Layer
}

func (self *ForkStackProposalUpdate) Run(args shared.RunArgs) error {
	return forkstack.Update(forkstack.UpdateArgs{
		ForkRepository: self.ForkRepository,
		Label:          self.Label,
		Layers:         self.Layers,
	}, args.Backend)
}
