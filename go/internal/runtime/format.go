package runtime

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"syscall"
	"time"

	"codex-build/internal/core"
)

type repositoryWorktreeOptions struct {
	endpointOptions
	ProjectWorkingRoot   string
	ProjectScratchRoot   string
	RuntimeTemporaryRoot string
	Source               string
	Release              string
	AttemptID            string
	Deadline             time.Duration
}

type RepositoryFormatOptions struct {
	repositoryWorktreeOptions
	Cancellation <-chan os.Signal
	OnLive       func()
}

type RepositoryFormatResult struct {
	SchemaVersion    int
	RecordType       string
	Release          string
	Source           string
	AttemptID        string
	FormattedPaths   []string
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

func repositoryFormatResult(options RepositoryFormatOptions) RepositoryFormatResult {
	return RepositoryFormatResult{
		SchemaVersion: core.SchemaVersion, RecordType: "RepositoryFormatResult",
		Release: options.Release, Source: options.Source, AttemptID: options.AttemptID,
		FormattedPaths: []string{}, Terminal: "Blocked", FailureDomain: failureDomainInput, ProductEffect: "None",
	}
}

func validateRepositoryFormatOptions(options RepositoryFormatOptions) error {
	return validateRepositoryWorktreeOptions(options.repositoryWorktreeOptions)
}

func validateRepositoryWorktreeOptions(options repositoryWorktreeOptions) error {
	if !validName(options.AttemptID, "-._") || !strings.HasPrefix(options.Release, "rust-v") || !validName(options.Release, "-._") {
		return errors.New("repository worktree identity differs")
	}
	if !validAcceptanceDeadline(options.Deadline) {
		return errors.New("repository worktree deadline differs")
	}
	for name, path := range map[string]string{
		"skill root": options.SkillRoot, "project working root": options.ProjectWorkingRoot,
		"project scratch root": options.ProjectScratchRoot, "runtime temporary root": options.RuntimeTemporaryRoot,
		"source": options.Source,
	} {
		if err := noFollowPath(name, path); err != nil {
			return err
		}
	}
	if !containedBy(options.ProjectScratchRoot, options.Source) || pathsOverlap(options.Source, options.ProjectWorkingRoot) || pathsOverlap(options.Source, options.RuntimeTemporaryRoot) {
		return errors.New("repository worktree roots differ")
	}
	return nil
}

func RunRepositoryFormat(options RepositoryFormatOptions) (result RepositoryFormatResult, resultErr error) {
	result = repositoryFormatResult(options)
	input, err := repositoryFormatPreimage(options)
	if err != nil {
		result.FailureDomain = runnerFailureDomain(err)
		return result, errors.Join(err, errors.New("repository format source differs"))
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
	attempt := filepath.Join(options.RuntimeTemporaryRoot, "repository-format", options.AttemptID)
	if _, statErr := os.Lstat(attempt); !os.IsNotExist(statErr) {
		result.FailureDomain = failureDomainHelper
		return result, errors.Join(statErr, errors.New("repository format attempt exists"))
	}
	for _, path := range []string{
		attempt, filepath.Join(attempt, "home"), filepath.Join(attempt, "tmp"),
		filepath.Join(attempt, "format-cache"), filepath.Join(attempt, "format-cache", "dotslash"), filepath.Join(attempt, "format-cache", "uv"),
		filepath.Join(attempt, "sdk-environment"), filepath.Join(attempt, "scripts-environment"),
	} {
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
	supportOutput, err := prepareRepositoryFormatSupport(options, toolchain, attempt)
	if err != nil {
		result.Terminal, result.FailureDomain, result.Stderr = "Fail", failureDomainHelper, supportOutput
		return result, err
	}
	preparedInput, err := repositoryFormatPreimage(options)
	if err != nil || !reflect.DeepEqual(preparedInput, input) {
		result.Terminal, result.FailureDomain = "Fail", failureDomainHelper
		return result, errors.Join(err, errors.New("repository format support changed the source projection"))
	}
	arguments := repositoryFormatSandboxArguments(options, toolchain, attempt, gitRoot)
	command := exec.Command("/usr/bin/bwrap", arguments...)
	command.Env = []string{}
	command.SysProcAttr = &syscall.SysProcAttr{Setpgid: true, Pdeathsig: syscall.SIGTERM}
	stdoutBuffer, stderrBuffer := &bytes.Buffer{}, &bytes.Buffer{}
	stdoutWriter := &boundedWriter{writer: stdoutBuffer, remaining: 4 << 20}
	stderrWriter := &boundedWriter{writer: stderrBuffer, remaining: 4 << 20}
	command.Stdout, command.Stderr = stdoutWriter, stderrWriter
	remaining := options.Deadline - time.Since(started)
	if remaining <= 0 {
		result.Terminal, result.FailureDomain = "Fail", failureDomainHelper
		return result, errors.New("repository format deadline expired after support preparation")
	}
	if err := command.Start(); err != nil {
		result.FailureDomain = failureDomainHelper
		return result, err
	}
	result.ExecutionStarted = true
	if options.OnLive != nil {
		options.OnLive()
	}
	runErr, timedOut, canceled := waitProcessGroup(command, remaining, core.GracePeriod(), options.Cancellation)
	result.TimedOut, result.Canceled = timedOut, canceled
	result.LogsTruncated = stdoutWriter.truncated || stderrWriter.truncated
	untracked := make([]string, len(input.Untracked))
	for index, file := range input.Untracked {
		untracked[index] = file.Path
	}
	paths, postErr := repositoryOwnedPostimage(options.RuntimeTemporaryRoot, options.Source, input.Projection.SourceBranch, untracked)
	head, headErr := repositoryGitOutput(options.Source, "rev-parse", "HEAD")
	if headErr != nil || string(head) != input.Head {
		postErr = errors.Join(postErr, headErr, errors.New("repository formatter changed HEAD"))
	}
	result.FormattedPaths = paths
	if len(paths) != 0 {
		result.ProductEffect = "FormatterPostimage"
	}
	if runErr == nil && postErr == nil && !result.LogsTruncated {
		result.Terminal, result.FailureDomain = "Pass", failureDomainNone
		return result, nil
	}
	result.Terminal, result.FailureDomain = "Fail", failureDomainProduct
	result.Stdout, result.Stderr = stdoutBuffer.String(), stderrBuffer.String()
	if timedOut || canceled || result.LogsTruncated || postErr != nil {
		result.FailureDomain = failureDomainHelper
	}
	return result, errors.Join(runErr, postErr)
}

type repositoryFormatInput struct {
	Projection sourceProjection
	Head       string
	Diff       string
	Untracked  []repositoryFormatFile
}

type repositoryFormatFile struct {
	Path    string
	Content []byte
	Mode    os.FileMode
}

func repositoryFormatPreimage(options RepositoryFormatOptions) (repositoryFormatInput, error) {
	projection, err := nativeSourceProjectionAt(options.RuntimeTemporaryRoot, options.Source, "custody-maintenance-source-projection")
	if err != nil || !repositoryEffectSource(options.repositoryWorktreeOptions, projection) {
		return repositoryFormatInput{}, errors.Join(err, errors.New("repository format worktree differs"))
	}
	untracked, err := repositoryUntrackedPaths(options.Source)
	if err != nil {
		return repositoryFormatInput{}, err
	}
	if _, err := repositoryOwnedPostimage(options.RuntimeTemporaryRoot, options.Source, projection.SourceBranch, untracked); err != nil {
		return repositoryFormatInput{}, err
	}
	files := make([]repositoryFormatFile, len(untracked))
	for index, path := range untracked {
		fullPath := filepath.Join(options.Source, path)
		content, readErr := readRegularNoFollow(fullPath)
		info, statErr := os.Lstat(fullPath)
		if readErr != nil || statErr != nil || !info.Mode().IsRegular() {
			return repositoryFormatInput{}, errors.Join(readErr, statErr, errors.New("repository formatter new input differs"))
		}
		files[index] = repositoryFormatFile{Path: path, Content: content, Mode: info.Mode()}
	}
	head, err := repositoryGitOutput(options.Source, "rev-parse", "HEAD")
	if err != nil {
		return repositoryFormatInput{}, err
	}
	diff, err := repositoryGitOutput(options.Source, "diff", "--binary", "--full-index", "--no-ext-diff", "--no-textconv", "HEAD", "--")
	return repositoryFormatInput{Projection: projection, Head: string(head), Diff: string(diff), Untracked: files}, err
}

func removeRepositoryEffectAttempt(attempt string) error {
	return removePrivateAttempt(attempt)
}

func prepareRepositoryFormatSupport(options RepositoryFormatOptions, toolchain ToolchainProjection, attempt string) (string, error) {
	cache := filepath.Join(attempt, "format-cache")
	baseEnvironment := []string{
		"HOME=" + filepath.Join(attempt, "home"), "TMPDIR=" + filepath.Join(attempt, "tmp"),
		"LANG=C.UTF-8", "LC_ALL=C.UTF-8", "PATH=" + filepath.Join(toolchain.Path, "bin") + ":/usr/bin",
	}
	type supportCommand struct {
		arguments   []string
		environment []string
	}
	commands := []supportCommand{
		{
			arguments:   []string{filepath.Join(toolchain.Path, "bin", "dotslash"), "--", "fetch", filepath.Join(options.Source, "tools", "buildifier")},
			environment: append(slices.Clone(baseEnvironment), "DOTSLASH_CACHE="+filepath.Join(cache, "dotslash")),
		},
		{
			arguments: []string{filepath.Join(toolchain.Path, "bin", "uv"), "sync", "--frozen", "--project", filepath.Join(options.Source, "sdk", "python"), "--only-group", "format", "--no-install-project", "--python", "/usr/bin/python3", "--no-python-downloads"},
			environment: append(slices.Clone(baseEnvironment),
				"UV_CACHE_DIR="+filepath.Join(cache, "uv"),
				"UV_PROJECT_ENVIRONMENT="+filepath.Join(attempt, "sdk-environment"),
			),
		},
		{
			arguments: []string{filepath.Join(toolchain.Path, "bin", "uv"), "sync", "--frozen", "--project", filepath.Join(options.Source, "scripts"), "--no-install-project", "--python", "/usr/bin/python3", "--no-python-downloads"},
			environment: append(slices.Clone(baseEnvironment),
				"UV_CACHE_DIR="+filepath.Join(cache, "uv"),
				"UV_PROJECT_ENVIRONMENT="+filepath.Join(attempt, "scripts-environment"),
			),
		},
	}
	end := time.Now().Add(options.Deadline)
	output := &bytes.Buffer{}
	writer := &boundedWriter{writer: output, remaining: 4 << 20}
	for _, support := range commands {
		remaining := time.Until(end)
		if remaining <= 0 {
			return output.String(), errors.New("repository format support timed out")
		}
		command := exec.Command(support.arguments[0], support.arguments[1:]...)
		command.Dir, command.Env = options.Source, support.environment
		command.SysProcAttr = &syscall.SysProcAttr{Setpgid: true, Pdeathsig: syscall.SIGTERM}
		command.Stdout, command.Stderr = writer, writer
		if err := command.Start(); err != nil {
			return output.String(), err
		}
		runErr, timedOut, canceled := waitProcessGroup(command, remaining, core.GracePeriod(), options.Cancellation)
		if runErr != nil || timedOut || canceled || writer.truncated {
			return output.String(), errors.Join(runErr, errors.New("repository format support preparation failed"))
		}
	}
	return output.String(), nil
}

func repositoryEffectSource(options repositoryWorktreeOptions, projection sourceProjection) bool {
	if !validRustSuiteProductBranch(options.Release, projection.SourceBranch) {
		return false
	}
	roles, err := readReleaseWorktrees(options.Source, options.ProjectScratchRoot, options.Release, newDeadline(options.Deadline, nil))
	return err == nil && roles.Registered[projection.SourceBranch] == options.Source
}

func repositoryGitRoot(source string) (string, error) {
	common, err := repositoryGitPath(source, "--git-common-dir")
	if err != nil {
		return "", err
	}
	directory, err := repositoryGitPath(source, "--git-dir")
	if err != nil || !containedBy(common, directory) {
		return "", errors.Join(err, errors.New("repository Git directory differs"))
	}
	return common, nil
}

func repositoryGitPath(source, argument string) (string, error) {
	command := exec.Command("/usr/bin/git", "-C", source, "rev-parse", "--path-format=absolute", argument)
	command.Env = repositoryGitEnvironment()
	output, err := command.Output()
	path := strings.TrimSpace(string(output))
	if err != nil || noFollowPath("repository Git path", path) != nil {
		return "", errors.Join(err, errors.New("repository Git path differs"))
	}
	return path, nil
}

func repositoryFormatSandboxArguments(options RepositoryFormatOptions, toolchain ToolchainProjection, attempt, gitRoot string) []string {
	arguments := []string{
		"--die-with-parent", "--new-session", "--unshare-all", "--proc", "/proc", "--dev", "/dev",
		"--ro-bind", "/usr", "/usr", "--symlink", "usr/bin", "/bin", "--symlink", "usr/lib", "/lib", "--symlink", "usr/lib64", "/lib64",
		"--dir", "/etc", "--ro-bind", "/etc/ld.so.cache", "/etc/ld.so.cache", "--ro-bind", "/etc/ssl", "/etc/ssl",
		"--bind", options.Source, "/workspace", "--ro-bind", filepath.Join(options.Source, ".git"), "/workspace/.git",
		"--ro-bind", toolchain.Path, "/toolchain", "--bind", filepath.Join(attempt, "home"), "/home", "--bind", filepath.Join(attempt, "tmp"), "/tmp",
		"--bind", filepath.Join(attempt, "format-cache"), "/format-cache",
		"--bind", filepath.Join(attempt, "sdk-environment"), "/workspace/sdk/python/.venv",
		"--bind", filepath.Join(attempt, "scripts-environment"), "/workspace/scripts/.venv",
	}
	arguments = append(arguments, repositorySandboxDirectories(filepath.Dir(gitRoot))...)
	arguments = append(arguments, "--ro-bind", gitRoot, gitRoot)
	for _, value := range []string{
		"HOME=/home", "TMPDIR=/tmp", "LANG=C.UTF-8", "LC_ALL=C.UTF-8", "TZ=UTC",
		"PATH=/toolchain/bin:/toolchain/rust/bin:/usr/bin", "CARGO_HOME=/home/cargo", "RUSTC=/toolchain/rust/bin/rustc",
		"DOTSLASH_CACHE=/format-cache/dotslash", "UV_CACHE_DIR=/format-cache/uv", "UV_OFFLINE=1", "UV_PYTHON_DOWNLOADS=never", "UV_NO_ENV_FILE=1",
		"GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL=/dev/null",
	} {
		name, content, _ := strings.Cut(value, "=")
		arguments = append(arguments, "--setenv", name, content)
	}
	arguments = append(arguments, "--chdir", "/workspace", "--", "/toolchain/bin/just")
	recipes := []string{"fmt", "fmt-check"}
	return append(arguments, recipes...)
}

func repositorySandboxDirectories(path string) []string {
	var paths []string
	for current := filepath.Clean(path); current != "/"; current = filepath.Dir(current) {
		paths = append(paths, current)
	}
	slices.Reverse(paths)
	arguments := make([]string, 0, len(paths)*2)
	for _, current := range paths {
		arguments = append(arguments, "--dir", current)
	}
	return arguments
}

func repositoryUntrackedPaths(source string) ([]string, error) {
	data, err := repositoryGitOutput(source, "ls-files", "--others", "--exclude-standard", "-z")
	if err != nil {
		return nil, err
	}
	paths, err := repositoryFormatPaths(data)
	slices.Sort(paths)
	return paths, err
}

func repositoryOwnedPostimage(runtimeRoot, source, branch string, admittedUntracked []string) ([]string, error) {
	projection, err := nativeSourceProjectionAt(runtimeRoot, source, "custody-maintenance-source-projection")
	if err != nil || projection.SourceBranch != branch {
		return nil, errors.Join(err, errors.New("repository worktree branch differs"))
	}
	untracked, err := repositoryUntrackedPaths(source)
	if err != nil || !slices.Equal(untracked, admittedUntracked) {
		return nil, errors.Join(err, errors.New("repository effect changed the admitted untracked paths"))
	}
	deleted, err := repositoryGitOutput(source, "diff", "--no-renames", "--name-only", "--diff-filter=D", "-z", "--")
	if err != nil || len(deleted) != 0 {
		return nil, errors.Join(err, errors.New("repository effect deleted paths"))
	}
	staged, err := repositoryGitOutput(source, "diff", "--cached", "--ita-visible-in-index", "--name-only", "-z", "--")
	if err != nil || len(staged) != 0 {
		return nil, errors.Join(err, errors.New("repository effect staged paths"))
	}
	output, err := repositoryGitOutput(source, "diff", "--no-renames", "--name-only", "-z", "--")
	if err != nil {
		return nil, err
	}
	paths, err := repositoryFormatPaths(output)
	if err != nil {
		return nil, err
	}
	paths = append(paths, untracked...)
	slices.Sort(paths)
	for index, path := range paths {
		if path == "" || !filepath.IsLocal(path) || filepath.Clean(path) != path || index > 0 && paths[index-1] == path {
			return nil, errors.New("repository effect path differs")
		}
		info, statErr := os.Lstat(filepath.Join(source, path))
		if statErr != nil || !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 {
			return nil, errors.Join(statErr, errors.New("repository effect postimage differs"))
		}
	}
	return paths, nil
}

func repositoryFormatPaths(data []byte) ([]string, error) {
	if len(data) == 0 {
		return []string{}, nil
	}
	if data[len(data)-1] != 0 {
		return nil, errors.New("repository formatter path output differs")
	}
	fields := bytes.Split(data[:len(data)-1], []byte{0})
	paths := make([]string, len(fields))
	for index, field := range fields {
		paths[index] = string(field)
	}
	return paths, nil
}

func repositoryGitOutput(source string, arguments ...string) ([]byte, error) {
	command := exec.Command("/usr/bin/git", append([]string{"-C", source}, arguments...)...)
	command.Env = repositoryGitEnvironment()
	return command.Output()
}

func repositoryGitEnvironment() []string {
	return []string{"HOME=/nonexistent", "PATH=/usr/bin:/bin", "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL=/dev/null"}
}
