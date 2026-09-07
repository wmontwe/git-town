package configdomain

import (
	"github.com/git-town/git-town/v24/internal/gohacks/stringss"
	. "github.com/git-town/git-town/v24/pkg/prelude"
)

// ForkStackLabel is the GitHub label applied to non-root fork-stack pull requests.
type ForkStackLabel stringss.Trimmed

func NewForkStackLabel(value stringss.Trimmed) Option[ForkStackLabel] {
	if value == "" {
		return None[ForkStackLabel]()
	}
	return Some(ForkStackLabel(value))
}

func (self ForkStackLabel) String() string {
	return string(self)
}
