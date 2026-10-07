package runtime

import (
	"bytes"
	"debug/elf"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"codex-build/internal/assets"
	"codex-build/internal/core"
)

type BuildInstallOptions struct {
	endpointOptions
	WorkingRoot, ScratchRoot, RuntimeRoot, RecordRoot string
	CodexHome                                         string
	Tag                                               string
	source, sourceCommit                              string
	BuildVersion                                      string
	BuildSequence                                     uint64
	InstallRequested                                  bool
	ToolPaths                                         [2]string
	Deadline                                          time.Duration
	OnLive                                            func()
	Cancellation                                      <-chan os.Signal
}

type BuildInstallResult struct {
	SchemaVersion                                                                  int
	RecordType, Terminal, ProductEffect, FailureDomain, PackagePath, CurrentTarget string
	Tag, BuildVersion                                                              string
	BuildSequence                                                                  uint64
}

type buildPackageMetadata struct {
	SchemaVersion                            int
	RecordType, ReleaseVersion, SourceCommit string
	Version                                  string `json:"version"`
	Target                                   string `json:"target"`
	Entrypoint                               string `json:"entrypoint"`
	BuildSequence                            uint64
	Tools                                    buildToolsPreparation
}

type inspectionToolRelease struct {
	Name, Version string
	Source        string
}

type buildToolsPreparation struct {
	Ccusage, Shuck, Tokei inspectionToolRelease
}

var packageFiles = [][2]string{
	{"codex", "bin/codex"}, {"codex-code-mode-host", "bin/codex-code-mode-host"},
	{"codex-voice-host", "codex-resources/voice/bin/codex-voice-host"},
	{"ccusage", "codex-path/ccusage"}, {"shuck", "codex-path/shuck"},
	{"tokei", "codex-path/tokei"}, {"rg", "codex-path/rg"}, {"bwrap", "codex-resources/bwrap"},
}

func RunBuildInstall(options BuildInstallOptions) (result BuildInstallResult, resultErr error) {
	result = BuildInstallResult{SchemaVersion: core.SchemaVersion, RecordType: "BuildInstallLocalResult", Tag: options.Tag, Terminal: "Blocked", ProductEffect: "None", FailureDomain: failureDomainInput, BuildSequence: options.BuildSequence, BuildVersion: options.BuildVersion}
	if err := validateBuildInstall(options); err != nil {
		return result, err
	}
	result.FailureDomain = failureDomainHelper
	cargo, err := cargoLock(options.ScratchRoot)
	if err != nil {
		return result, err
	}
	defer releaseCargoLock(cargo)
	result.FailureDomain = failureDomainInput
	options.source = newReleaseWorktrees(options.ScratchRoot).Main
	if _, err := fixedMainProjection(options.ScratchRoot, options.RuntimeRoot, options.Tag, options.source); err != nil {
		return result, err
	}
	head, err := repositoryGitOutput(options.source, "rev-parse", "HEAD")
	options.sourceCommit = strings.TrimSpace(string(head))
	if err != nil || !validGitCommit(options.sourceCommit) {
		return result, errors.Join(err, errors.New("BuildInstall Main commit is absent"))
	}
	dist := filepath.Join(options.WorkingRoot, "build", "dist")
	result.FailureDomain = failureDomainHelper
	preparation, preparedAssets, err := prepareBuildInspectionTools(options)
	if err != nil {
		return result, err
	}
	result.FailureDomain = failureDomainInput
	if completed, completedErr := completedBuildPostimage(dist, options, preparation, &result); completedErr != nil {
		return result, completedErr
	} else if completed {
		result.Terminal, result.FailureDomain = "Pass", failureDomainNone
		return result, nil
	}
	if err := validateBuildSequence(dist, options.Tag, options.BuildSequence); err != nil {
		return result, err
	}
	tools := make([]runtimeAsset, 0, 5)
	tools = append(tools, runtimeAsset{name: "build-install.zsh", data: assets.BuildInstall(), mode: 0o700})
	tools = append(tools, preparedAssets...)
	for index, name := range []string{"rg", "bwrap"} {
		data, readErr := readRegularNoFollow(options.ToolPaths[index])
		if readErr != nil {
			return result, errors.Join(readErr, fmt.Errorf("BuildInstall %s input differs", name))
		}
		tools = append(tools, runtimeAsset{name: name, data: data, mode: 0o700})
	}
	result.Terminal, result.FailureDomain = "Fail", failureDomainHelper
	outputRoot := projectionPublishedOutputRoot(options.WorkingRoot, "build-install-"+options.BuildVersion)
	if err := removeOwnedPaths(outputRoot); err != nil {
		return result, err
	}
	projection, err := runProjection(projectionRequest{
		SkillRoot: options.SkillRoot, ProjectWorkingRoot: options.WorkingRoot, ProjectScratchRoot: options.ScratchRoot, RuntimeTemporaryRoot: options.RuntimeRoot, CodexHome: options.CodexHome,
		RecordRoot: options.RecordRoot, Source: options.source, SourceCommit: options.sourceCommit, Release: options.Tag, WorkingDirectory: "codex-rs", Arguments: []string{"/runner/build-install.zsh", options.BuildVersion},
		OutputRoot: outputRoot, ResultFiles: []string{"codex", "codex-code-mode-host", "codex-voice-host", "ccusage", "shuck", "tokei", "rg", "bwrap", "build.stdout.log", "build.stderr.log"}, Consumer: runnerConsumer{assets: tools, environment: []string{"CARGO_NET_OFFLINE=true", "STABLE_GIT_COMMIT=" + options.sourceCommit, "CODEX_SANDBOX_NETWORK_DISABLED=1", "CCUSAGE_VERSION=" + preparation.Ccusage.Version}, networkDisabled: true},
		LockHeld: true, Deadline: options.Deadline, Cancellation: options.Cancellation, OnLive: options.OnLive, AttemptIdentity: "build-install-" + options.BuildVersion,
	})
	defer func() {
		for _, file := range packageFiles {
			bindBuildInstallCleanup(&result, &resultErr, filepath.Join(outputRoot, file[0]))
		}
	}()
	if err != nil || !projection.ExecutionStarted || !projection.Reaped || !projection.AttemptCleaned || projection.ProcessState != "StartedReaped" || projection.AttemptCleanup != "ExactAttemptRootAbsent" || !projection.SourceUnchanged || projection.FailureClass != nil {
		return result, errors.Join(err, errors.New("BuildInstall build failed"))
	}
	if projection.ExitCode != 0 {
		result.Terminal, result.FailureDomain = "Fail", failureDomainProduct
		return result, errors.New("BuildInstall product build failed")
	}
	packageName := options.BuildVersion
	stage, final := filepath.Join(dist, "."+packageName+".stage"), filepath.Join(dist, packageName)
	if err := ensureDirectoryNoFollow(dist, 0o700); err != nil {
		return result, err
	}
	if _, err := os.Lstat(stage); !os.IsNotExist(err) {
		return result, errors.New("BuildInstall stage exists")
	}
	if _, err := os.Lstat(final); !os.IsNotExist(err) {
		return result, errors.New("BuildInstall package exists")
	}
	if err := os.Mkdir(stage, 0o700); err != nil {
		return result, err
	}
	defer func() { bindBuildInstallCleanup(&result, &resultErr, stage) }()
	for _, file := range packageFiles {
		name, relative := file[0], file[1]
		if err := ensureDirectoryNoFollow(filepath.Dir(filepath.Join(stage, relative)), 0o700); err != nil {
			return result, err
		}
		file := projection.Files[name]
		data, readErr := readRegularNoFollow(file.Path)
		if readErr != nil {
			return result, errors.Join(readErr, errors.New("BuildInstall projection file differs"))
		}
		if err := writeRuntimeAsset(filepath.Join(stage, relative), data, 0o755); err != nil {
			return result, err
		}
	}
	toolchain, err := resolveToolchainProjection(options.WorkingRoot)
	if err != nil {
		return result, err
	}
	if err := copyVoiceRuntime(filepath.Join(toolchain.Path, "voice", "runtime"), filepath.Join(stage, "codex-resources", "voice")); err != nil {
		return result, err
	}
	metadata := buildPackageMetadataFor(options, preparation)
	metadataBytes, err := core.CanonicalJSON(metadata)
	if err != nil {
		return result, err
	}
	if err := writeRuntimeAsset(filepath.Join(stage, "codex-package.json"), metadataBytes, 0o644); err != nil {
		return result, err
	}
	if err := validatePackagePostimage(stage, filepath.Join(toolchain.Path, "voice", "runtime"), options.CodexHome, metadata); err != nil {
		return result, err
	}
	if err := os.Rename(stage, final); err != nil {
		return result, err
	}
	result.PackagePath, result.ProductEffect = final, "PackagePublished"
	if err := syncDirectory(dist); err != nil {
		return result, err
	}
	if options.InstallRequested {
		if err := publishBuildSelection(dist, options, &result); err != nil {
			return result, err
		}
	}
	result.Terminal = "Pass"
	result.FailureDomain = failureDomainNone
	return result, nil
}

func bindBuildInstallCleanup(result *BuildInstallResult, resultErr *error, path string) {
	if err := removeOwnedPaths(path); err != nil {
		result.Terminal, result.FailureDomain = "Fail", failureDomainHelper
		*resultErr = errors.Join(*resultErr, err)
	}
}

func prepareBuildInspectionTools(options BuildInstallOptions) (preparation buildToolsPreparation, prepared []runtimeAsset, resultErr error) {
	id, err := randomID()
	if err != nil {
		return preparation, nil, err
	}
	attempt := filepath.Join(options.RuntimeRoot, "build-tools", id)
	for _, path := range []string{attempt, filepath.Join(attempt, "home")} {
		if err := ensureDirectoryNoFollow(path, 0o700); err != nil {
			return preparation, nil, errors.Join(err, removeOwnedPaths(attempt))
		}
	}
	defer func() { resultErr = errors.Join(resultErr, removeOwnedPaths(attempt)) }()
	script := filepath.Join(attempt, "build-tools.py")
	if err := writeRuntimeAsset(script, assets.BuildTools(), 0o700); err != nil {
		return preparation, nil, err
	}
	toolchain, err := resolveToolchainProjection(options.WorkingRoot)
	if err != nil {
		return preparation, nil, err
	}
	cache := cacheProjection(options.WorkingRoot)
	if err := ensureDirectoryNoFollow(cache.CargoHome, 0o700); err != nil {
		return preparation, nil, err
	}
	output := filepath.Join(attempt, "output")
	metadataPath := filepath.Join(output, "metadata.json")
	_, stderr, err := runNativeProcess(attempt, script, "prepare", []string{filepath.Join(toolchain.Path, "rust", "bin", "cargo"), cache.CargoHome, output, filepath.Join(options.ScratchRoot, "tool-sources"), options.source}, nativeEnvironment(attempt), 32<<20, metadataPath, core.StageDeadline(), options.Cancellation, options.OnLive)
	if err != nil {
		if message := strings.TrimSpace(stderr); message != "" {
			err = errors.Join(err, errors.New(message))
		}
		return preparation, nil, err
	}
	if _, err := readCanonicalValue(metadataPath, &preparation); err != nil || !validInspectionToolRelease(preparation.Shuck, "shuck") || !validInspectionToolRelease(preparation.Tokei, "tokei") || !validInspectionToolRelease(preparation.Ccusage, "ccusage") {
		return buildToolsPreparation{}, nil, errors.Join(err, errors.New("BuildInstall inspection tool preparation differs"))
	}
	for _, name := range []string{"shuck-source.tar.gz", "tokei-source.tar.gz", "ccusage-source.tar.gz", "ccusage-pricing.json"} {
		data, err := readRegularNoFollow(filepath.Join(output, name))
		if err != nil {
			return buildToolsPreparation{}, nil, err
		}
		prepared = append(prepared, runtimeAsset{name: name, data: data, mode: 0o600})
	}
	return preparation, prepared, nil
}

func validInspectionToolRelease(release inspectionToolRelease, name string) bool {
	version := toolVersion{}
	count, err := fmt.Sscanf(release.Version, "%d.%d.%d", &version.Major, &version.Minor, &version.Patch)
	return release.Name == name && count == 3 && err == nil && version.String() == release.Version && len(release.Source) == 40
}

func buildPackageMetadataFor(options BuildInstallOptions, preparation buildToolsPreparation) buildPackageMetadata {
	return buildPackageMetadata{
		SchemaVersion: core.SchemaVersion, RecordType: "BuildPackageMetadata", ReleaseVersion: options.Tag,
		Version: options.BuildVersion, BuildSequence: options.BuildSequence, SourceCommit: options.sourceCommit, Tools: preparation,
		Target: nativeTarget, Entrypoint: "bin/codex",
	}
}

func validateBuildInstall(options BuildInstallOptions) error {
	for _, path := range []string{options.SkillRoot, options.WorkingRoot, options.ScratchRoot, options.RuntimeRoot, options.RecordRoot, options.CodexHome, filepath.Join(options.WorkingRoot, "build", "dist")} {
		if noFollowPath("build-install path", path) != nil {
			return errors.New("invalid BuildInstall path")
		}
	}
	if options.CodexHome == "/" || pathsOverlap(options.CodexHome, filepath.Join(options.WorkingRoot, "build", "dist")) {
		return errors.New("invalid BuildInstall CODEX_HOME")
	}
	expectedVersion := strings.TrimPrefix(options.Tag, "rust-v")
	if options.BuildSequence != 0 {
		expectedVersion = fmt.Sprintf("%s.%d", expectedVersion, options.BuildSequence)
	}
	if !core.ValidReleaseTag(options.Tag) || options.BuildVersion != expectedVersion || !validName(options.BuildVersion, ".-") || !validAcceptanceDeadline(options.Deadline) {
		return errors.New("invalid BuildInstall input")
	}
	for index := range options.ToolPaths {
		if noFollowPath("build-install tool", options.ToolPaths[index]) != nil {
			return errors.New("invalid BuildInstall tooling")
		}
	}
	return nil
}

func completedBuildPostimage(dist string, options BuildInstallOptions, preparation buildToolsPreparation, result *BuildInstallResult) (bool, error) {
	final := filepath.Join(dist, options.BuildVersion)
	info, err := os.Lstat(final)
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return false, errors.Join(err, errors.New("BuildInstall package postimage differs"))
	}
	metadata := buildPackageMetadataFor(options, preparation)
	toolchain, toolchainErr := resolveToolchainProjection(options.WorkingRoot)
	if toolchainErr != nil {
		return false, toolchainErr
	}
	if err := validatePackagePostimage(final, filepath.Join(toolchain.Path, "voice", "runtime"), options.CodexHome, metadata); err != nil {
		return false, err
	}
	result.PackagePath, result.ProductEffect = final, "PackagePublished"
	if !options.InstallRequested {
		return true, nil
	}
	result.Terminal, result.FailureDomain = "Fail", failureDomainHelper
	if err := publishBuildSelection(dist, options, result); err != nil {
		return false, err
	}
	return true, nil
}

func validatePackagePostimage(root, voiceRuntime, codexHome string, metadata buildPackageMetadata) error {
	manifest, readError := readRegularNoFollow(filepath.Join(root, "codex-package.json"))
	expectedManifest, encodingError := core.CanonicalJSON(metadata)
	if readError != nil || encodingError != nil || !bytes.Equal(manifest, expectedManifest) {
		return errors.Join(readError, encodingError, errors.New("BuildInstall package manifest differs"))
	}
	expected := map[string]string{"codex-package.json": ""}
	for _, file := range packageFiles {
		expected[file[1]] = ""
	}
	err := walkPackageTree(voiceRuntime, func(relative, path string, info os.FileInfo) error {
		name := filepath.Join("codex-resources", "voice", relative)
		if !info.IsDir() {
			if _, exists := expected[name]; exists {
				return errors.New("native voice runtime collides with package")
			}
			expected[name] = path
		}
		return nil
	})
	if err != nil {
		return err
	}
	err = walkPackageTree(root, func(name, path string, info os.FileInfo) error {
		if info.IsDir() {
			if name == "." || strings.HasPrefix(name, "codex-resources/voice/") {
				return nil
			}
			for file := range expected {
				if strings.HasPrefix(file, name+"/") {
					return nil
				}
			}
			return errors.New("unexpected package directory")
		}
		source, exists := expected[name]
		if !exists {
			return errors.New("package file differs")
		}
		mode := os.FileMode(0o755)
		if name == "codex-package.json" {
			mode = 0o644
		}
		if source != "" {
			sourceInfo, err := os.Lstat(source)
			if err != nil || !sourceInfo.Mode().IsRegular() {
				return errors.Join(err, errors.New("native voice runtime file differs"))
			}
			mode = 0o644
			if sourceInfo.Mode().Perm()&0o111 != 0 {
				mode = 0o755
			}
			before, beforeErr := readRegularNoFollow(source)
			after, afterErr := readRegularNoFollow(path)
			if beforeErr != nil || afterErr != nil || !bytes.Equal(before, after) {
				return errors.Join(beforeErr, afterErr, errors.New("package runtime bytes differ"))
			}
		}
		if info.Mode().Perm() != mode {
			return errors.New("package file mode differs")
		}
		delete(expected, name)
		return nil
	})
	if err != nil || len(expected) != 0 {
		return errors.Join(err, errors.New("package inventory differs"))
	}
	for _, file := range packageFiles {
		if file[0] != "rg" && file[0] != "bwrap" {
			if err := validateGNUBinary(filepath.Join(root, file[1])); err != nil {
				return err
			}
		}
	}
	for _, tool := range []inspectionToolRelease{metadata.Tools.Ccusage, metadata.Tools.Shuck, metadata.Tools.Tokei} {
		command, err := packageCommand(root, codexHome, filepath.Join("codex-path", tool.Name), "--version")
		if err != nil {
			return err
		}
		version, err := readToolVersion(command, tool.Name, newDeadline(core.StageDeadline(), nil))
		if err != nil || version.String() != tool.Version {
			return errors.Join(err, errors.New("BuildInstall inspection tool version differs"))
		}
	}
	for _, check := range [][3]string{
		{"bin/codex", "--version", "codex-cli " + metadata.Version},
		{"codex-resources/voice/bin/codex-voice-host", "--build-commit", metadata.SourceCommit},
	} {
		command, err := packageCommand(root, codexHome, check[0], check[1])
		if err != nil {
			return err
		}
		var stdout, stderr diagnosticWriter
		command.Stdout, command.Stderr = &stdout, &stderr
		err = newDeadline(core.GracePeriod(), nil).run(command)
		if err != nil || stdout.truncated || strings.TrimSpace(string(stdout.data)) != check[2] {
			return errors.Join(err, fmt.Errorf("package %s: expected %q; stdout=%q stderr=%q", check[0], check[2], stdout.data, stderr.data))
		}
	}
	return nil
}

func packageCommand(root, codexHome, executable, argument string) (*exec.Cmd, error) {
	if err := validateRunnerHome(codexHome); err != nil {
		return nil, err
	}
	arguments, err := runnerSystemArguments(true)
	if err != nil {
		return nil, err
	}
	arguments = append(arguments,
		"--ro-bind", root, "/result/package",
		"--tmpfs", "/home/codex", "--tmpfs", codexHome,
		"--tmpfs", "/harness", "--tmpfs", "/tmp", "--tmpfs", "/run",
		"--chdir", "/tmp", "--clearenv",
		"--setenv", "HOME", "/home/codex", "--setenv", "CODEX_HOME", codexHome,
		"--setenv", "CODEX_HARNESS", "/harness", "--setenv", "TMPDIR", "/tmp",
		"--setenv", "PATH", "/usr/bin", "--setenv", "LANG", "C.UTF-8",
		filepath.Join("/result/package", executable), argument)
	command := exec.Command("/usr/bin/bwrap", arguments...)
	command.Env = []string{"PATH=/usr/bin"}
	return command, nil
}

func validateGNUBinary(path string) error {
	binary, err := elf.Open(path)
	if err != nil {
		return err
	}
	defer binary.Close()
	if binary.Class == elf.ELFCLASS64 && binary.Machine == elf.EM_X86_64 {
		for _, program := range binary.Progs {
			if program.Type == elf.PT_INTERP {
				loader, err := io.ReadAll(io.LimitReader(program.Open(), 256))
				if err == nil && filepath.Base(strings.TrimSuffix(string(loader), "\x00")) == "ld-linux-x86-64.so.2" {
					return nil
				}
			}
		}
	}
	return fmt.Errorf("package binary is not x86_64 GNU: %s", path)
}

func walkPackageTree(root string, visit func(string, string, os.FileInfo) error) error {
	return filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && !info.Mode().IsRegular() {
			return errors.New("package tree contains a non-regular entry")
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		return visit(relative, path, info)
	})
}

func copyVoiceRuntime(source, target string) error {
	return walkPackageTree(source, func(relative, path string, info os.FileInfo) error {
		if relative == "." {
			return nil
		}
		destination := filepath.Join(target, relative)
		if info.IsDir() {
			return ensureDirectoryNoFollow(destination, 0o700)
		}
		data, err := readRegularNoFollow(path)
		if err != nil {
			return err
		}
		mode := os.FileMode(0o644)
		if info.Mode().Perm()&0o111 != 0 {
			mode = 0o755
		}
		return writeRuntimeAsset(destination, data, mode)
	})
}

func validateBuildSequence(dist, release string, sequence uint64) error {
	entries, err := os.ReadDir(dist)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	prior := sequence == 0
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		var metadata buildPackageMetadata
		data, readErr := readJSONValue(filepath.Join(dist, entry.Name(), "codex-package.json"), &metadata)
		if readErr == nil && strings.Contains(string(data), `"BuildSequence":`) && metadata.SchemaVersion == core.SchemaVersion && metadata.RecordType == "BuildPackageMetadata" && metadata.ReleaseVersion == release {
			if metadata.BuildSequence >= sequence {
				prior = false
				break
			}
			prior = prior || metadata.BuildSequence+1 == sequence
		}
	}
	if !prior {
		return errors.New("sequence differs")
	}
	return nil
}

func publishBuildSelection(dist string, options BuildInstallOptions, result *BuildInstallResult) (resultError error) {
	identifier, creationError := randomID()
	if creationError != nil {
		return creationError
	}
	attempt := filepath.Join(options.RuntimeRoot, "build-select", identifier)
	if creationError := ensureDirectoryNoFollow(filepath.Dir(attempt), 0o700); creationError != nil {
		return creationError
	}
	if creationError := os.Mkdir(attempt, 0o700); creationError != nil {
		return creationError
	}
	defer func() { bindBuildInstallCleanup(result, &resultError, attempt) }()
	script := filepath.Join(attempt, "build-select.zsh")
	if writeError := writeRuntimeAsset(script, assets.BuildSelect(), 0o700); writeError != nil {
		return writeError
	}
	_, diagnostic, runError := runNativeProcess(attempt, script, "install", []string{dist, options.BuildVersion}, nativeEnvironment(attempt), 1<<20, "", options.Deadline, options.Cancellation, options.OnLive)
	currentTarget, selectionError := os.Readlink(filepath.Join(dist, "current"))
	if selectionError == nil && currentTarget == options.BuildVersion {
		result.CurrentTarget, result.ProductEffect = currentTarget, "CurrentLinkChanged"
	} else {
		runError = errors.Join(runError, selectionError, errors.New("build-install current selection differs"))
	}
	if message := strings.TrimSpace(diagnostic); runError != nil && message != "" {
		runError = errors.Join(runError, errors.New(message))
	}
	return runError
}
