package cmd //nolint:testpackage

import (
	"testing"

	"github.com/git-town/git-town/v24/internal/config/configdomain"
	"github.com/git-town/git-town/v24/internal/git/gitdomain"
	"github.com/shoenig/test/must"
)

func TestBranchesToProposeFromStack(t *testing.T) {
	t.Parallel()
	lineage := configdomain.NewLineage().
		Set("root", "main").
		Set("current", "root").
		Set("feature-child", "current").
		Set("prototype-child", "current").
		Set("feature-below-prototype", "prototype-child").
		Set("sibling", "root")
	branches := gitdomain.LocalBranchNames{"root", "current", "feature-child", "prototype-child", "feature-below-prototype", "sibling"}
	branchesAndTypes := configdomain.BranchesAndTypes{
		"current":                 configdomain.BranchTypePrototypeBranch,
		"feature-below-prototype": configdomain.BranchTypeFeatureBranch,
		"feature-child":           configdomain.BranchTypeFeatureBranch,
		"prototype-child":         configdomain.BranchTypePrototypeBranch,
		"root":                    configdomain.BranchTypeFeatureBranch,
		"sibling":                 configdomain.BranchTypeFeatureBranch,
	}

	have := branchesToProposeFromStack(branches, "current", branchesAndTypes, lineage)

	must.Eq(t, gitdomain.LocalBranchNames{"root", "current", "feature-child", "sibling"}, have)
}

func TestPrototypeAncestor(t *testing.T) {
	t.Parallel()
	lineage := configdomain.NewLineage().
		Set("prototype", "main").
		Set("current", "prototype")
	branchesAndTypes := configdomain.BranchesAndTypes{
		"current":   configdomain.BranchTypeFeatureBranch,
		"prototype": configdomain.BranchTypePrototypeBranch,
	}

	have, hasPrototypeAncestor := prototypeAncestor("current", branchesAndTypes, lineage).Get()

	must.True(t, hasPrototypeAncestor)
	must.EqOp(t, gitdomain.LocalBranchName("prototype"), have)
}

func TestStackBranchesForPropose(t *testing.T) {
	t.Parallel()
	lineage := configdomain.NewLineage().
		Set("root", "main").
		Set("left", "root").
		Set("left-leaf", "left").
		Set("right", "root").
		Set("right-leaf", "right")

	have := stackBranchesForPropose(lineage, "left-leaf", gitdomain.LocalBranchNames{"main"}, configdomain.OrderAsc)

	must.Eq(t, gitdomain.LocalBranchNames{"root", "left", "left-leaf", "right", "right-leaf"}, have)
}
