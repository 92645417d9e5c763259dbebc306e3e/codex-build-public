package runtime

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"codex-build/internal/core"
)

type RepositoryMaintenanceOptions struct {
	repositoryWorktreeOptions
	UpdateCargoLock bool
	Cancellation    <-chan os.Signal
	OnLive          func()
}

type RepositoryMaintenanceResult struct {
	SchemaVersion    int
	RecordType       string
	Release          string
	Source           string
	AttemptID        string
	MaintainedPaths  []string
	Terminal         string
	FailureDomain    string
	ProductEffect    string
	Stdout           string
	Stderr           string
	ExecutionStarted bool
	TimedOut         bool
	Canceled         bool
	LogsTruncated    bool
}

type repositoryMaintenanceExecution struct {
	stdout, stderr                         string
	started, timedOut, canceled, truncated bool
	err                                    error
}

func repositoryMaintenanceResult(options RepositoryMaintenanceOptions) RepositoryMaintenanceResult {
	return RepositoryMaintenanceResult{
		SchemaVersion: core.SchemaVersion, RecordType: "RepositoryMaintenanceResult",
		Release: options.Release, Source: options.Source, AttemptID: options.AttemptID,
		MaintainedPaths: []string{}, Terminal: "Blocked", FailureDomain: failureDomainInput, ProductEffect: "None",
	}
}

func RunRepositoryMaintenance(options RepositoryMaintenanceOptions) (result RepositoryMaintenanceResult, resultErr error) {
	result = repositoryMaintenanceResult(options)
	if options.UpdateCargoLock {
		lock, err := cargoLock(options.ProjectScratchRoot)
		if err != nil {
			result.FailureDomain = failureDomainHelper
			return result, err
		}
		defer releaseCargoLock(lock)
		cargoHome := cacheProjection(options.ProjectWorkingRoot).CargoHome
		if err := noFollowPath("maintenance Cargo home", cargoHome); err != nil {
			return result, err
		}
		info, err := os.Lstat(cargoHome)
		if err != nil || !info.IsDir() {
			result.FailureDomain = failureDomainHelper
			return result, errors.Join(err, errors.New("maintenance Cargo cache is absent"))
		}
		for _, name := range []string{"config", "config.toml", "credentials", "credentials.toml"} {
			if _, err := os.Lstat(filepath.Join(cargoHome, name)); !os.IsNotExist(err) {
				return result, errors.Join(err, errors.New("maintenance Cargo home contains an override"))
			}
		}
	}
	projection, err := repositoryMaintenanceSourceProjection(options.repositoryWorktreeOptions)
	if err != nil {
		result.FailureDomain = runnerFailureDomain(err)
		return result, errors.Join(err, errors.New("repository maintenance source differs"))
	}
	toolchain, err := resolveToolchainProjection(options.ProjectWorkingRoot, options.Source, options.RuntimeTemporaryRoot)
	if err != nil {
		result.FailureDomain = failureDomainHelper
		return result, err
	}
	gitRoot, err := repositoryGitRoot(options.Source)
	if err != nil {
		result.FailureDomain = failureDomainHelper
		return result, err
	}
	beforePaths, beforeBytes, err := repositoryMaintenancePreimage(options, projection.SourceBranch)
	if err != nil {
		return result, err
	}
	head, err := repositoryGitOutput(options.Source, "rev-parse", "HEAD")
	if err != nil {
		result.FailureDomain = failureDomainHelper
		return result, err
	}
	attempt := filepath.Join(options.RuntimeTemporaryRoot, "repository-maintenance", options.AttemptID)
	if _, statErr := os.Lstat(attempt); !os.IsNotExist(statErr) {
		result.FailureDomain = failureDomainHelper
		return result, errors.Join(statErr, errors.New("repository maintenance attempt exists"))
	}
	for _, path := range []string{attempt, filepath.Join(attempt, "home"), filepath.Join(attempt, "tmp")} {
		if err := ensureDirectoryNoFollow(path, 0o700); err != nil {
			result.FailureDomain = failureDomainHelper
			return result, errors.Join(err, removeRepositoryEffectAttempt(attempt))
		}
	}
	defer func() {
		if cleanupErr := removeRepositoryEffectAttempt(attempt); cleanupErr != nil {
			result.Terminal, result.FailureDomain = "Fail", failureDomainHelper
			resultErr = errors.Join(resultErr, cleanupErr)
		}
	}()
	started := time.Now()
	if !options.UpdateCargoLock {
		acquisition := runRepositoryMaintenanceCommand(repositoryMaintenanceSandboxArguments(options, toolchain, attempt, gitRoot, true), options.Deadline, options.Cancellation, options.OnLive)
		result.ExecutionStarted = acquisition.started
		result.TimedOut, result.Canceled, result.LogsTruncated = acquisition.timedOut, acquisition.canceled, acquisition.truncated
		if acquisition.err != nil || acquisition.timedOut || acquisition.canceled || acquisition.truncated {
			result.Terminal, result.FailureDomain = "Fail", failureDomainHelper
			result.Stdout, result.Stderr = acquisition.stdout, acquisition.stderr
			return result, errors.Join(acquisition.err, errors.New("repository maintenance acquisition failed"))
		}
	}
	remaining := options.Deadline - time.Since(started)
	if remaining <= 0 {
		result.Terminal, result.FailureDomain = "Fail", failureDomainHelper
		return result, errors.New("repository maintenance deadline exhausted before recipe")
	}
	execution := runRepositoryMaintenanceCommand(repositoryMaintenanceSandboxArguments(options, toolchain, attempt, gitRoot, false), remaining, options.Cancellation, options.OnLive)
	result.ExecutionStarted = result.ExecutionStarted || execution.started
	result.TimedOut, result.Canceled, result.LogsTruncated = execution.timedOut, execution.canceled, execution.truncated
	paths, postErr := repositoryMaintenancePostimage(options, projection.SourceBranch, beforePaths, beforeBytes)
	afterHead, headErr := repositoryGitOutput(options.Source, "rev-parse", "HEAD")
	if headErr != nil || !bytes.Equal(head, afterHead) {
		postErr = errors.Join(postErr, headErr, errors.New("repository maintenance changed HEAD"))
	}
	result.MaintainedPaths = paths
	if len(paths) != 0 {
		result.ProductEffect = "BazelLockPostimage"
		if options.UpdateCargoLock {
			result.ProductEffect = "CargoLockPostimage"
		}
	}
	if execution.err == nil && postErr == nil && !execution.timedOut && !execution.canceled && !result.LogsTruncated {
		result.Terminal, result.FailureDomain = "Pass", failureDomainNone
		return result, nil
	}
	result.Terminal, result.FailureDomain = "Fail", failureDomainProduct
	result.Stdout, result.Stderr = execution.stdout, execution.stderr
	if !execution.started || execution.timedOut || execution.canceled || result.LogsTruncated || postErr != nil {
		result.FailureDomain = failureDomainHelper
	}
	return result, errors.Join(execution.err, postErr, errors.New("repository maintenance recipe failed"))
}

func runRepositoryMaintenanceCommand(arguments []string, deadline time.Duration, cancellation <-chan os.Signal, onLive func()) repositoryMaintenanceExecution {
	command := exec.Command("/usr/bin/bwrap", arguments...)
	command.Env = []string{}
	command.SysProcAttr = &syscall.SysProcAttr{Setpgid: true, Pdeathsig: syscall.SIGTERM}
	stdoutBuffer, stderrBuffer := &bytes.Buffer{}, &bytes.Buffer{}
	stdoutWriter := &boundedWriter{writer: stdoutBuffer, remaining: 4 << 20}
	stderrWriter := &boundedWriter{writer: stderrBuffer, remaining: 4 << 20}
	command.Stdout, command.Stderr = stdoutWriter, stderrWriter
	execution := repositoryMaintenanceExecution{}
	if execution.err = command.Start(); execution.err == nil {
		execution.started = true
		if onLive != nil {
			onLive()
		}
		execution.err, execution.timedOut, execution.canceled = waitProcessGroup(command, deadline, core.GracePeriod(), cancellation)
	}
	execution.stdout, execution.stderr = stdoutBuffer.String(), stderrBuffer.String()
	execution.truncated = stdoutWriter.truncated || stderrWriter.truncated
	return execution
}

func repositoryMaintenanceSourceProjection(options repositoryWorktreeOptions) (sourceProjection, error) {
	if !strings.HasPrefix(options.Release, "rust-v") || !validName(options.Release, "-._") {
		return sourceProjection{}, errors.New("invalid projection release")
	}
	projection, err := nativeSourceProjectionAt(options.RuntimeTemporaryRoot, options.Source, "custody-maintenance-source-projection")
	if err != nil || !repositoryEffectSource(options, projection) {
		return sourceProjection{}, errors.Join(err, errors.New("repository maintenance source role rejected"))
	}
	return projection, nil
}

func repositoryMaintenancePreimage(options RepositoryMaintenanceOptions, branch string) ([]string, map[string][]byte, error) {
	if options.UpdateCargoLock {
		if _, err := readRegularNoFollow(filepath.Join(options.Source, "codex-rs", "Cargo.lock")); err != nil {
			return nil, nil, errors.Join(err, errors.New("workspace Cargo lock preimage is absent"))
		}
	} else {
		bazelVersion, err := readRegularNoFollow(filepath.Join(options.Source, ".bazelversion"))
		if err != nil || !bytes.HasSuffix(bazelVersion, []byte("\n")) || !validToolRelease(strings.TrimSuffix(string(bazelVersion), "\n")) {
			return nil, nil, errors.Join(err, errors.New("repository Bazel selection differs"))
		}
	}
	paths, err := repositoryOwnedPostimage(options.RuntimeTemporaryRoot, options.Source, branch, nil)
	if err != nil {
		return nil, nil, err
	}
	hasDependencyChange := false
	content := make(map[string][]byte, len(paths))
	for _, path := range paths {
		if path == "MODULE.bazel.lock" {
			return nil, nil, errors.New("Bazel lock postimage already exists")
		}
		base := filepath.Base(path)
		if options.UpdateCargoLock && (base != "Cargo.toml" || !strings.HasPrefix(path, "codex-rs/")) {
			return nil, nil, errors.New("Cargo lock maintenance requires only workspace manifest postimages")
		}
		if base != "Cargo.toml" && base != "Cargo.lock" {
			return nil, nil, errors.New("repository maintenance preimage contains a nondependency path")
		}
		hasDependencyChange = true
		content[path], err = os.ReadFile(filepath.Join(options.Source, path))
		if err != nil {
			return nil, nil, err
		}
	}
	if !hasDependencyChange {
		return nil, nil, errors.New("repository maintenance needs a dependency owner postimage")
	}
	return paths, content, nil
}

func repositoryMaintenancePostimage(options RepositoryMaintenanceOptions, branch string, beforePaths []string, beforeBytes map[string][]byte) ([]string, error) {
	afterPaths, err := repositoryOwnedPostimage(options.RuntimeTemporaryRoot, options.Source, branch, nil)
	if err != nil {
		return nil, err
	}
	after := make(map[string]bool, len(afterPaths))
	for _, path := range afterPaths {
		after[path] = true
	}
	for _, path := range beforePaths {
		if !after[path] {
			return nil, errors.New("repository maintenance changed dependency owner paths")
		}
		content, readErr := os.ReadFile(filepath.Join(options.Source, path))
		if readErr != nil || !bytes.Equal(content, beforeBytes[path]) {
			return nil, errors.Join(readErr, errors.New("repository maintenance changed dependency owner bytes"))
		}
		delete(after, path)
	}
	if len(after) == 0 {
		return []string{}, nil
	}
	output := "MODULE.bazel.lock"
	if options.UpdateCargoLock {
		output = "codex-rs/Cargo.lock"
	}
	if len(after) != 1 || !after[output] {
		return nil, errors.New("repository maintenance postimage differs")
	}
	return []string{output}, nil
}

func repositoryMaintenanceSandboxArguments(options RepositoryMaintenanceOptions, toolchain ToolchainProjection, attempt, gitRoot string, acquisition bool) []string {
	arguments := []string{
		"--die-with-parent", "--new-session", "--unshare-all", "--proc", "/proc", "--dev", "/dev",
		"--ro-bind", "/usr", "/usr", "--symlink", "usr/bin", "/bin", "--symlink", "usr/lib", "/lib", "--symlink", "usr/lib64", "/lib64",
		"--dir", "/etc", "--ro-bind", "/etc/ld.so.cache", "/etc/ld.so.cache", "--ro-bind", "/etc/ssl", "/etc/ssl",
		"--ro-bind", "/etc/resolv.conf", "/etc/resolv.conf", "--ro-bind", "/etc/hosts", "/etc/hosts", "--ro-bind", "/etc/nsswitch.conf", "/etc/nsswitch.conf",
	}
	if acquisition {
		arguments = append(arguments, "--share-net", "--ro-bind", options.Source, "/workspace")
	} else {
		arguments = append(arguments, "--bind", options.Source, "/workspace", "--ro-bind", filepath.Join(options.Source, ".git"), "/workspace/.git")
	}
	arguments = append(arguments, "--ro-bind", toolchain.Path, "/toolchain", "--bind", filepath.Join(attempt, "home"), "/home", "--bind", filepath.Join(attempt, "tmp"), "/tmp")
	arguments = append(arguments, repositorySandboxDirectories(filepath.Dir(gitRoot))...)
	arguments = append(arguments, "--ro-bind", gitRoot, gitRoot)
	if options.UpdateCargoLock {
		arguments = append(arguments, "--bind", cacheProjection(options.ProjectWorkingRoot).CargoHome, "/cargo-home",
			"--setenv", "CARGO_HOME", "/cargo-home", "--setenv", "CARGO_NET_OFFLINE", "true",
			"--setenv", "CARGO_TERM_COLOR", "never", "--setenv", "RUSTC", "/toolchain/rust/bin/rustc")
	}
	for _, value := range []string{
		"HOME=/home", "USER=codex", "TMPDIR=/tmp", "LANG=C.UTF-8", "LC_ALL=C.UTF-8", "TZ=UTC",
		"PATH=/toolchain/bin:/toolchain/rust/bin:/usr/bin", "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL=/dev/null",
	} {
		name, content, _ := strings.Cut(value, "=")
		arguments = append(arguments, "--setenv", name, content)
	}
	if options.UpdateCargoLock {
		arguments = append(arguments, "--chdir", "/workspace/codex-rs", "--")
		return append(arguments, "/toolchain/rust/bin/cargo", "update", "--workspace", "--offline")
	}
	arguments = append(arguments, "--chdir", "/workspace", "--")
	if acquisition {
		return append(arguments, "/toolchain/bin/bazel", "--batch", "mod", "deps", "--lockfile_mode=off")
	}
	return append(arguments, "/toolchain/bin/just", "bazel-lock-update")
}
