package runtime

import (
	"encoding/json"
	"errors"
	"path/filepath"
	"strings"

	"codex-build/internal/assets"
	"codex-build/internal/core"
)

type NativeToolchainOptions struct {
	endpointOptions
	WorkingRoot string
	ScratchRoot string
	RuntimeRoot string
	Source      string
	Release     string
	Auxiliary   []string
	OnLive      func()
}

type NativeToolchainResult struct {
	SchemaVersion                       int
	RecordType, Terminal, FailureDomain string
	ProductEffect, ToolchainRoot        string
}

type nativeToolchainResult struct {
	Result     string
	Output     string
	Diagnostic string
}

func prepareNativeToolchain(options NativeToolchainOptions) (NativeToolchainResult, error) {
	result := NativeToolchainResult{SchemaVersion: core.SchemaVersion, RecordType: "NativeToolchainResult", Terminal: "Blocked", FailureDomain: failureDomainInput, ProductEffect: "None"}
	for _, path := range []string{options.WorkingRoot, options.ScratchRoot, options.RuntimeRoot, options.Source} {
		if noFollowPath("native toolchain root", path) != nil {
			return result, errors.New("native toolchain root differs")
		}
	}
	window := newDeadline(core.StageDeadline(), nil)
	roles, err := readReleaseWorktrees(options.Source, options.ScratchRoot, options.Release, window)
	if err != nil {
		return result, err
	}
	registered := false
	for _, source := range roles.Registered {
		registered = registered || source == options.Source
	}
	if !registered || len(options.Auxiliary) != 6 {
		return result, errors.New("native toolchain source or auxiliary selection differs")
	}
	for _, version := range options.Auxiliary {
		if !validToolRelease(version) {
			return result, errors.New("native toolchain auxiliary version is not a semantic release")
		}
	}
	lock, err := cargoLock(options.ScratchRoot)
	if err != nil {
		result.FailureDomain = failureDomainHelper
		return result, err
	}
	defer releaseCargoLock(lock)
	attempt, err := prepareNativeToolchainAttempt(options.RuntimeRoot)
	if err != nil {
		result.FailureDomain = failureDomainHelper
		return result, err
	}
	defer removeOwnedPaths(attempt)
	resultPath := filepath.Join(attempt, "result.json")
	script := filepath.Join(attempt, "native-toolchain.py")
	arguments := append([]string{options.WorkingRoot, options.ScratchRoot, attempt, resultPath, options.Source}, options.Auxiliary...)
	data, stderr, runErr := runNativeProcess(attempt, script, "prepare", arguments, nativeEnvironment(attempt), 32<<20, resultPath, core.StageDeadline(), nil, options.OnLive)
	if diagnostic := strings.TrimSpace(stderr); diagnostic != "" {
		runErr = errors.Join(runErr, errors.New(diagnostic))
	}
	var native nativeToolchainResult
	if runErr == nil {
		runErr = json.Unmarshal(data, &native)
	}
	if runErr != nil || native.Result != "Prepared" || native.Diagnostic != "" {
		result.Terminal, result.FailureDomain = "Fail", failureDomainHelper
		var diagnosticErr error
		if native.Diagnostic != "" {
			diagnosticErr = errors.New(native.Diagnostic)
		}
		return result, errors.Join(runErr, diagnosticErr, errors.New("native toolchain build failed"))
	}
	projection, err := resolveToolchainProjection(options.WorkingRoot, options.Source, options.RuntimeRoot)
	if err != nil {
		result.Terminal, result.FailureDomain = "Fail", failureDomainHelper
		return result, errors.Join(err, errors.New("published native toolchain admission failed"))
	}
	if projection.Path != native.Output {
		result.Terminal, result.FailureDomain = "Fail", failureDomainHelper
		return result, errors.New("published native toolchain output differs")
	}
	result.Terminal, result.FailureDomain, result.ProductEffect = "Pass", failureDomainNone, "ToolchainPublished"
	result.ToolchainRoot = projection.Root
	return result, nil
}

func prepareNativeToolchainAttempt(runtimeRoot string) (string, error) {
	id, err := randomID()
	if err != nil {
		return "", err
	}
	root := filepath.Join(runtimeRoot, "native-toolchain", id)
	if err := ensureDirectoryNoFollow(root, 0o700); err != nil {
		return "", err
	}
	script := filepath.Join(root, "native-toolchain.py")
	if err := writeRuntimeAsset(script, assets.NativeToolchain(), 0o700); err != nil {
		return "", errors.Join(err, removeOwnedPaths(root))
	}
	return root, nil
}
