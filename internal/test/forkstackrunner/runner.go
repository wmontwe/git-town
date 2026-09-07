// Package forkstackrunner provides a command runner for fork-stack tests.
package forkstackrunner

import "github.com/git-town/git-town/v24/internal/gohacks/stringss"

// Runner is a configurable command runner for fork-stack opcode tests.
type Runner struct {
	QueryFunc func(executable string, args ...string) (string, error)
}

func (self Runner) Query(executable string, args ...string) (string, error) {
	return self.QueryFunc(executable, args...)
}

func (self Runner) QueryTrim(executable string, args ...string) (stringss.Trimmed, error) {
	output, err := self.Query(executable, args...)
	return stringss.Trimmed(output), err
}

func (self Runner) QueryZ(executable string, args ...string) (stringss.ZeroDelineated, error) {
	output, err := self.Query(executable, args...)
	return stringss.ZeroDelineated(output), err
}

func (self Runner) Run(executable string, args ...string) error {
	_, err := self.Query(executable, args...)
	return err
}

func (self Runner) RunWithEnv(_ []string, executable string, args ...string) error {
	return self.Run(executable, args...)
}
