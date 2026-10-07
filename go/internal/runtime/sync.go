package runtime

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"codex-build/internal/assets"
	"codex-build/internal/core"
)

type publicSyncOptions struct {
	endpointOptions
	WorkingRoot string
	RuntimeRoot string
	Version     string
	PatchTag    string
}

type publicSyncResult struct {
	Terminal      string
	FailureDomain string
	ProductEffect string `json:",omitempty"`
	Version       string
	PatchTag      string
	Repository    string
	PublicRoot    string
	URL           string
}

func runSync(arguments []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("sync", flag.ContinueOnError)
	flags.SetOutput(stderr)
	options := publicSyncOptions{}
	bindPublicRoots(flags, &options.endpointOptions, &options.WorkingRoot, nil, &options.RuntimeRoot, nil)
	flags.StringVar(&options.Version, "version", "", "entire public commit message (release version)")
	flags.StringVar(&options.PatchTag, "patch-tag", "", "one committed canonical exported patch set")
	if parseError := parsePublicFlags(flags, arguments); errors.Is(parseError, flag.ErrHelp) {
		return 0
	} else if parseError != nil || flags.NArg() != 0 {
		return 2
	}
	result := publicSyncResult{Terminal: "Fail", FailureDomain: failureDomainHelper, Version: options.Version, PatchTag: options.PatchTag}
	if admissionError := admitPublicRoute(&options.endpointOptions, "SyncSupport"); admissionError != nil {
		return writeResult(result, result.Terminal, admissionError, stdout, stderr)
	}
	cancellation, releaseController, signalError := controllerSignals()
	if signalError != nil {
		return writeResult(result, result.Terminal, signalError, stdout, stderr)
	}
	defer releaseController()
	newWaitNotice(stderr, "sync")
	result, runError := runPublicSync(options, result, cancellation)
	return writeResult(result, result.Terminal, runError, stdout, stderr)
}

func runPublicSync(options publicSyncOptions, result publicSyncResult, cancellation <-chan os.Signal) (completed publicSyncResult, runError error) {
	window := newDeadline(core.StageDeadline(), cancellation)
	if !core.ValidReleaseTag("rust-v"+options.Version) || !core.ValidReleaseTag(options.PatchTag) {
		return result, errors.New("sync requires --version RELEASE_VERSION and --patch-tag rust-vRELEASE_TAG")
	}
	repositories := os.Getenv("REPOS_ROOT")
	if repositories == "" {
		return result, errors.New("sync requires REPOS_ROOT")
	}
	privateRoot := filepath.Join(options.WorkingRoot, "build", "patches", "private")
	result.PublicRoot = filepath.Join(repositories, core.PublicSyncRepositoryName)
	for _, root := range []string{options.SkillRoot, options.WorkingRoot, options.RuntimeRoot, repositories, privateRoot, result.PublicRoot, filepath.Join(privateRoot, options.PatchTag)} {
		if pathError := noFollowPath("sync root", root); pathError != nil {
			return result, pathError
		}
	}
	for _, source := range []string{options.SkillRoot, options.WorkingRoot, options.RuntimeRoot, privateRoot} {
		if pathsOverlap(source, result.PublicRoot) {
			return result, errors.New("public checkout must be separate from source and runtime roots")
		}
	}
	environment := make([]string, 0, len(os.Environ())+3)
	for _, value := range os.Environ() {
		if !strings.HasPrefix(value, "GIT_") {
			environment = append(environment, value)
		}
	}
	environment = append(environment, "PATH=/usr/bin:/bin", "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL=/dev/null")
	login, queryError := publicSyncQuery(window, environment, "api", "user", "--jq", ".login")
	if queryError != nil {
		return result, queryError
	}
	owner := strings.TrimSpace(string(login))
	if !validName(owner, "-") {
		return result, errors.New("authenticated GitHub login differs")
	}
	result.Repository = owner + "/" + core.PublicSyncRepositoryName
	data, queryError := publicSyncQuery(window, environment, "repo", "view", result.Repository, "--json", "nameWithOwner,isPrivate,defaultBranchRef,url")
	if queryError != nil {
		return result, queryError
	}
	var repository struct {
		NameWithOwner    string `json:"nameWithOwner"`
		IsPrivate        bool   `json:"isPrivate"`
		URL              string `json:"url"`
		DefaultBranchRef struct {
			Name string `json:"name"`
		} `json:"defaultBranchRef"`
	}
	if decodeError := json.Unmarshal(data, &repository); decodeError != nil {
		return result, decodeError
	}
	if repository.NameWithOwner != result.Repository || repository.IsPrivate || (repository.DefaultBranchRef.Name != "" && repository.DefaultBranchRef.Name != "main") {
		return result, errors.New("sync target must be the authenticated user's public repository with main or no default branch")
	}
	result.URL = repository.URL
	parent := filepath.Join(options.RuntimeRoot, "sync")
	if directoryError := ensureDirectoryNoFollow(parent, 0o700); directoryError != nil {
		return result, directoryError
	}
	attempt, directoryError := os.MkdirTemp(parent, "attempt-")
	if directoryError != nil {
		return result, directoryError
	}
	defer func() {
		cleanupError := removeOwnedPathWithin(attempt, newDeadline(core.GracePeriod(), nil))
		if cleanupError != nil {
			completed.Terminal, completed.FailureDomain = "Fail", failureDomainHelper
			runError = errors.Join(runError, cleanupError)
		}
	}()
	script := filepath.Join(attempt, "sync.zsh")
	if writeError := writeRuntimeAsset(script, assets.PublicSync(), 0o700); writeError != nil {
		return result, writeError
	}
	arguments := []string{options.SkillRoot, privateRoot, options.PatchTag, result.PublicRoot, result.Repository, options.Version, repository.DefaultBranchRef.Name, attempt}
	_, diagnostic, nativeError := runNativeProcess(attempt, script, "sync", arguments, environment, core.PublicSyncLogLimit, "", window.remaining(), cancellation, nil)
	if nativeError != nil {
		return result, fmt.Errorf("public snapshot sync: %w: %s", nativeError, diagnostic)
	}
	result.Terminal, result.FailureDomain, result.ProductEffect = "Pass", "", "PublicSnapshotPublished"
	return result, nil
}

func publicSyncQuery(window deadline, environment []string, arguments ...string) ([]byte, error) {
	command := exec.Command("/usr/bin/gh", arguments...)
	command.Env = environment
	output, diagnostic := &diagnosticWriter{}, &diagnosticWriter{}
	command.Stdout, command.Stderr = output, diagnostic
	if queryError := window.run(command); queryError != nil {
		return nil, fmt.Errorf("sync GitHub discovery: %w: %s", queryError, diagnostic.String())
	}
	if output.truncated {
		return nil, errors.New("sync GitHub discovery exceeded its output bound")
	}
	return output.data, nil
}
