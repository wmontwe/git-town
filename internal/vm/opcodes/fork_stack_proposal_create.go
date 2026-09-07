package opcodes

import (
	"github.com/git-town/git-town/v24/internal/config/configdomain"
	"github.com/git-town/git-town/v24/internal/forge/forgedomain"
	"github.com/git-town/git-town/v24/internal/forkstack"
	"github.com/git-town/git-town/v24/internal/git/gitdomain"
	"github.com/git-town/git-town/v24/internal/vm/shared"
	. "github.com/git-town/git-town/v24/pkg/prelude"
)

// ForkStackProposalCreate creates cumulative upstream GitHub pull requests for a stack.
type ForkStackProposalCreate struct {
	BranchesToPropose gitdomain.LocalBranchNames
	ForkRepository    forgedomain.HostedRepoInfo
	Label             Option[configdomain.ForkStackLabel]
	Layers            []forkstack.Layer
	MainBranch        gitdomain.LocalBranchName
	ProposalBody      Option[gitdomain.ProposalBody]
	ProposalTitle     Option[gitdomain.ProposalTitle]
	SelectedBranch    gitdomain.LocalBranchName
}

func (self *ForkStackProposalCreate) Run(args shared.RunArgs) error {
	return forkstack.Create(forkstack.CreateArgs{
		BranchesToPropose: self.BranchesToPropose,
		ForkRepository:    self.ForkRepository,
		Label:             self.Label,
		Layers:            self.Layers,
		MainBranch:        self.MainBranch,
		ProposalBody:      self.ProposalBody,
		ProposalTitle:     self.ProposalTitle,
		SelectedBranch:    self.SelectedBranch,
	}, args.Backend, args.FinalMessages)
}
