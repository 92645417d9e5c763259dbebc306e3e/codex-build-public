package runtime

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"codex-build/internal/core"
)

type releaseWorktrees struct {
	Main       string
	Upstream   string
	ForkRoot   string
	Forks      map[string]string
	Registered map[string]string
}

func newReleaseWorktrees(scratch string) releaseWorktrees {
	return releaseWorktrees{
		Main:       filepath.Join(scratch, "main-worktree", "source"),
		Upstream:   filepath.Join(scratch, "upstream-baseline", "source"),
		ForkRoot:   filepath.Join(scratch, "forked-worktrees"),
		Forks:      map[string]string{},
		Registered: map[string]string{},
	}
}

func readReleaseWorktrees(source, scratch, tag string, window deadline) (releaseWorktrees, error) {
	return readReleaseWorktreeSet(source, scratch, tag, nil, window)
}

func readReleaseBaselineWorktrees(source, scratch, tag, predecessor string, window deadline) (releaseWorktrees, error) {
	return readReleaseWorktreeSet(source, scratch, tag, []string{predecessor}, window)
}

func readReleaseWorktreeSet(source, scratch, tag string, additionalTags []string, window deadline) (releaseWorktrees, error) {
	roles := newReleaseWorktrees(scratch)
	if len(additionalTags) > 1 {
		return roles, errors.New("release worktree boundary has multiple alternate tags")
	}
	allowed := map[string]bool{}
	for _, allowedTag := range append([]string{tag}, additionalTags...) {
		if !core.ValidReleaseTag(allowedTag) || allowed[allowedTag] {
			return roles, errors.New("release worktree tag boundary differs")
		}
		allowed[allowedTag] = true
	}
	if noFollowPath("release scratch", scratch) != nil {
		return roles, errors.New("release worktree inputs differ")
	}
	command := exec.Command("/usr/bin/git", "-C", source, "worktree", "list", "--porcelain", "-z")
	command.Env = repositoryGitEnvironment()
	var output bytes.Buffer
	var diagnostic diagnosticWriter
	command.Stdout, command.Stderr = &output, &diagnostic
	if err := window.run(command); err != nil {
		return roles, preflightHelperError{errors.Join(err, errors.New("release worktree inventory failed: "+boundedRunnerDiagnostic(diagnostic.String())))}
	}
	observedTag := ""
	for _, record := range strings.Split(output.String(), "\x00\x00") {
		path, branch := "", ""
		for _, field := range strings.Split(record, "\x00") {
			if value, present := strings.CutPrefix(field, "worktree "); present {
				path = value
			}
			if value, present := strings.CutPrefix(field, "branch refs/heads/"); present {
				branch = value
			}
		}
		branchTag, expected := "", ""
		switch {
		case strings.HasSuffix(branch, "-upstream-baseline"):
			branchTag = strings.TrimSuffix(branch, "-upstream-baseline")
			expected = roles.Upstream
		case strings.HasSuffix(branch, "-main-worktree"):
			branchTag = strings.TrimSuffix(branch, "-main-worktree")
			expected = roles.Main
		default:
			separator := strings.LastIndex(branch, "-main-worktree-forked-")
			if separator >= 0 {
				branchTag = branch[:separator]
				ordinal := branch[separator+len("-main-worktree-forked-"):]
				if validForkOrdinal(ordinal) {
					expected = filepath.Join(roles.ForkRoot, ordinal)
				}
			}
		}
		if !core.ValidReleaseTag(branchTag) {
			continue
		}
		info, err := os.Lstat(path)
		if !allowed[branchTag] || observedTag != "" && observedTag != branchTag || expected == "" || path != expected || err != nil || path == scratch || !containedBy(scratch, path) || noFollowPath("registered release worktree", path) != nil || !info.IsDir() || roles.Registered[branch] != "" {
			return roles, errors.Join(err, errors.New("registered global release worktree differs: "+path))
		}
		observedTag = branchTag
		roles.Registered[branch] = path
		switch branch {
		case branchTag + "-main-worktree":
			roles.Main = path
		case branchTag + "-upstream-baseline":
			roles.Upstream = path
		default:
			ordinal := strings.TrimPrefix(branch, branchTag+"-main-worktree-forked-")
			roles.Forks[ordinal] = path
		}
	}
	for _, path := range []string{roles.Main, roles.Upstream, roles.ForkRoot} {
		if err := noFollowPath("release worktree role", path); err != nil {
			return roles, err
		}
	}
	return roles, nil
}

func (roles releaseWorktrees) productPath(tag, source string) bool {
	for branch, path := range roles.Registered {
		if path == source && validRustSuiteProductBranch(tag, branch) {
			return true
		}
	}
	return false
}

func (roles releaseWorktrees) parent(tag, ordinal string) string {
	if ordinal == "" {
		return roles.Registered[tag+"-main-worktree"]
	}
	return roles.Forks[ordinal]
}
