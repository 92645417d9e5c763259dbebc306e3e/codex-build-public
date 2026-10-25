package runtime

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"codex-build/internal/assets"
	"codex-build/internal/core"
)

type ReleaseBaselineOptions struct {
	endpointOptions
	WorkingRoot, ScratchRoot, RuntimeRoot, RecordRoot, Tag string
	Prepare                                                bool
	RequiredTests                                          []string
	Cancellation                                           <-chan os.Signal
	OnLive                                                 func()
	OnPhase                                                func(string)
	worktrees                                              releaseWorktrees
	taskPath, predecessorTag                               string
	taskBytes, indexBytes, nextIndex                       []byte
}

type BaselineCoverageOptions struct {
	ReleaseBaselineOptions
	Unit, FlatPatch  string
	Align            bool
	AffectedPackages []string
	Proposal         rustSuiteBaselineProposal
}

type nativeReleaseBaselineResult struct {
	Result, PredecessorTag, Output string
	Bootstrap, Ready               bool
}

type ReleaseRouteResult struct {
	SchemaVersion                    int
	RecordType, Route, Tag, Terminal string
	FailureDomain, ProductEffect     string
}

func releaseRouteResult(route, tag string) ReleaseRouteResult {
	return ReleaseRouteResult{SchemaVersion: core.SchemaVersion, RecordType: "ReleaseRouteResult", Route: route, Tag: tag, Terminal: "Blocked", FailureDomain: failureDomainInput, ProductEffect: "None"}
}

func (result ReleaseRouteResult) terminal(terminal, domain, effect string) ReleaseRouteResult {
	result.Terminal, result.FailureDomain, result.ProductEffect = terminal, domain, effect
	return result
}

func rustSuiteRouteFailure(result ReleaseRouteResult, suite RustSuiteResult, err error) (ReleaseRouteResult, error) {
	domain, terminal := suite.FailureDomain, "Fail"
	if domain == "" {
		domain = failureDomainHelper
	}
	if suite.Terminal == "Stopped" {
		terminal = "Stopped"
	}
	return result.terminal(terminal, domain, "None"), err
}

func (options ReleaseBaselineOptions) rustSuiteOptions() RustSuiteOptions {
	return RustSuiteOptions{endpointOptions: options.endpointOptions, ProjectWorkingRoot: options.WorkingRoot, ProjectScratchRoot: options.ScratchRoot, RuntimeTemporaryRoot: options.RuntimeRoot, RecordRoot: options.RecordRoot, Source: options.worktrees.Upstream, Scope: "baseline", Tag: options.Tag, BaselineSelectionPath: filepath.Join(options.ScratchRoot, "tasks", "rust-suite-baselines.json"), Cancellation: options.Cancellation, OnLive: options.OnLive, OnPhase: options.OnPhase, releaseBaseline: true, cargoLockHeld: true, requiredTests: options.RequiredTests}
}

func RunReleaseBaseline(options ReleaseBaselineOptions) (ReleaseRouteResult, error) {
	started := time.Now()
	blocked := releaseRouteResult("Baseline", options.Tag)
	if err := validateReleaseOptions(options, true); err != nil || !validRustSuiteRequiredTests(options.RequiredTests) || options.Prepare && len(options.RequiredTests) != 0 {
		return blocked, errors.Join(err, errors.New("invalid baseline preparation or required tests"))
	}
	cargo, err := cargoLock(options.ScratchRoot)
	if err != nil {
		return blocked.terminal("Fail", failureDomainHelper, "None"), err
	}
	defer releaseCargoLock(cargo)
	options, err = releaseBaselineInputs(options, deadline{end: started.Add(core.StageDeadline()), signals: options.Cancellation})
	if err != nil {
		return blocked, err
	}
	suite := options.rustSuiteOptions()
	suite.started, suite.end = started, started.Add(core.StageDeadline())
	calibrationPlan := RustSuitePlan{}
	suite.calibrationPlan = &calibrationPlan
	if _, _, err = readRustSuiteBaselineFile(suite.BaselineSelectionPath, options.Tag); err != nil {
		return blocked, err
	}
	var start nativeReleaseBaselineResult
	if err = ensureCargoProjection(cacheProjection(options.WorkingRoot)); err == nil {
		start, err = runReleaseBaselineNative(options, "release-baseline-start", suite.deadline())
		if err == nil && (start.Result != "Started" || start.PredecessorTag != options.predecessorTag) {
			err = errors.New("release baseline start differs")
		}
	}
	if err != nil {
		return blocked.terminal("Fail", failureDomainHelper, "None"), err
	}
	if options.Prepare {
		return blocked.terminal("Pass", failureDomainNone, "BaselineSourcePrepared"), nil
	}
	if !start.Ready {
		fields, taskErr := taskGraphFields(options.taskBytes, "CurrentWorktree")
		currentDiff, currentErr := releaseBugfixDiff(fields["CurrentWorktree"], options.RuntimeRoot, options.Tag, suite.deadline())
		baselineDiff, baselineErr := releaseBugfixDiff(suite.Source, options.RuntimeRoot, options.Tag, suite.deadline())
		if taskErr != nil || currentErr != nil || baselineErr != nil || currentDiff != baselineDiff {
			return blocked, errors.Join(taskErr, currentErr, baselineErr, errors.New("Current and Baseline bugfix prefixes differ before measurement"))
		}
	}
	inherited, _, inheritedErr := readRustSuiteBaselineFile(suite.BaselineSelectionPath, options.predecessorTag)
	if inheritedErr != nil {
		return blocked, inheritedErr
	}
	if inherited.Established != nil {
		proposal := inherited.Established.Selection.rustSuiteBaselineProposal
		proposal.Tag = options.Tag
		proposal.BugfixDiff, err = releaseBugfixDiff(suite.Source, options.RuntimeRoot, options.Tag, suite.deadline())
		if err != nil {
			return blocked, err
		}
		suite.baselinePreimage = &inherited.Established.Selection
		suite.baselineProposal = &proposal
	} else if !start.Bootstrap && !start.Ready {
		return blocked, errors.New("inherited established baseline differs")
	}
	var runErr error
	if start.Ready {
		runErr = validateReleaseCleanupBaseline(suite)
	} else if start.Bootstrap {
		runErr = establishZeroRustSuiteBaseline(suite)
	} else {
		result, suiteErr := RunRustSuite(suite)
		if suiteErr != nil || result.Terminal != "Pass" {
			return rustSuiteRouteFailure(blocked, result, suiteErr)
		}
	}
	if runErr != nil {
		return blocked.terminal("Fail", failureDomainHelper, "None"), runErr
	}
	native, err := runReleaseBaselineNative(options, "release-baseline-cleanup", suite.deadline())
	if err != nil || native.Result != "Cleaned" || native.PredecessorTag != options.predecessorTag {
		return blocked.terminal("Fail", failureDomainHelper, "BaselineEstablished"), errors.Join(err, errors.New("release baseline cleanup differs"))
	}
	if err := publishRustSuiteBaselineTiming(suite); err != nil {
		return blocked.terminal("Fail", failureDomainHelper, "BaselineEstablishedAndCleanupComplete"), err
	}
	return blocked.terminal("Pass", failureDomainNone, "BaselineEstablishedAndCleanupComplete"), nil
}

func releaseBaselineInputs(options ReleaseBaselineOptions, window deadline) (ReleaseBaselineOptions, error) {
	task, err := activeTaskPath(options.ScratchRoot, options.Tag, "")
	if err != nil {
		return options, err
	}
	options.taskPath = task
	options.taskBytes, err = readRegularNoFollow(task)
	if err != nil {
		return options, err
	}
	prefixes, err := taskSourcePrefixes(options.taskBytes, options.Tag)
	if err != nil {
		return options, err
	}
	options.predecessorTag = prefixes[0]
	options.indexBytes, err = readRegularNoFollow(filepath.Join(options.ScratchRoot, "tasks", "index.json"))
	if err != nil {
		return options, err
	}
	options.worktrees, err = readReleaseBaselineWorktrees(options.WorkingRoot, options.ScratchRoot, options.Tag, options.predecessorTag, window)
	if err != nil {
		return options, err
	}
	fields, err := taskGraphFields(options.taskBytes, "CurrentWorktree")
	if err != nil || fields["CurrentWorktree"] != options.worktrees.Main && !options.worktrees.productPath(options.Tag, fields["CurrentWorktree"]) {
		return options, errors.Join(err, errors.New("baseline task must name its actual registered Current or canonical global Main path"))
	}
	return options, nil
}

func RunReleaseCleanup(options ReleaseBaselineOptions) (ReleaseRouteResult, error) {
	result := releaseRouteResult("Cleanup", options.Tag)
	if err := validateReleaseOptions(options, false); err != nil {
		return result, err
	}
	if pathsOverlap(options.RuntimeRoot, options.WorkingRoot) {
		return result, errors.New("cleanup runtime overlaps working root")
	}
	for _, root := range []string{options.RuntimeRoot, options.RecordRoot, projectionTemporaryRoot(options.WorkingRoot)} {
		for _, retained := range []string{options.SkillRoot, os.Getenv("PROJECT_PERSISTENT_ROOT")} {
			if retained != "" && pathsOverlap(root, retained) {
				return result, errors.New("cleanup overlaps retained storage")
			}
		}
	}
	cargo, err := cargoLock(options.ScratchRoot)
	if err != nil {
		return result.terminal("Fail", failureDomainHelper, "None"), err
	}
	defer releaseCargoLock(cargo)
	window := newDeadline(core.CleanupDeadline(), options.Cancellation)
	if options.Tag == "" {
		if err := removeCompletedReleaseOutputs(options, window); err != nil {
			return result.terminal("Fail", failureDomainHelper, "None"), err
		}
		return result.terminal("Pass", failureDomainNone, "CleanupComplete"), nil
	}
	var index outcomeTaskIndex
	options.indexBytes, err = readCanonicalValue(filepath.Join(options.ScratchRoot, "tasks", "index.json"), &index)
	if err != nil {
		return result, err
	}
	if err := validateReleaseTaskIndex(options.ScratchRoot, index, options.Tag); err != nil {
		return result, err
	}
	options.worktrees, err = readReleaseWorktrees(options.WorkingRoot, options.ScratchRoot, options.Tag, window)
	if err != nil || options.worktrees.Registered[options.Tag+"-main-worktree"] == "" {
		return result, errors.Join(err, errors.New("completed release Main is absent"))
	}
	rows, err := verifyReleasePublication(options)
	if err != nil {
		return result, err
	}
	options.taskPath = filepath.Join(options.ScratchRoot, "tasks", releaseTaskName(options.Tag))
	taskBytes, _, present, taskErr := readOptionalRegular(options.taskPath)
	if taskErr != nil {
		return result, taskErr
	}
	options.taskBytes = taskBytes
	if present {
		terminal, err := completedTaskTerminal(options.taskBytes, options.worktrees.Main, options.Tag, options.worktrees)
		prefixes, prefixErr := taskSourcePrefixes(options.taskBytes, options.Tag)
		if err != nil || prefixErr != nil || patchReplayRowSelector(rows[len(rows)-1]) != terminal {
			return result, errors.Join(err, prefixErr, errors.New("cleanup complete task frontier differs"))
		}
		options.predecessorTag = prefixes[0]
	}
	next, err := nextReleaseTaskIndex(index, options.Tag)
	if err == nil {
		options.nextIndex, err = core.CanonicalJSON(next)
	}
	if err != nil {
		return result, err
	}
	native, err := runReleaseBaselineNative(options, "release-cleanup", window)
	if err == nil && native.Result == "Input" {
		return result, errors.New(native.Output)
	}
	if err != nil || native.Result != "Cleaned" || native.PredecessorTag != options.predecessorTag {
		effect := "None"
		currentIndex, indexErr := readRegularNoFollow(filepath.Join(options.ScratchRoot, "tasks", "index.json"))
		currentTask, _, taskPresent, taskErr := readOptionalRegular(options.taskPath)
		if indexErr != nil || taskErr != nil {
			effect = "TaskRetirementUncertain"
		} else if !bytes.Equal(currentIndex, options.indexBytes) || taskPresent != present || !bytes.Equal(currentTask, options.taskBytes) {
			effect = "TaskRetirementChanged"
		}
		return result.terminal("Fail", failureDomainHelper, effect), errors.Join(err, indexErr, taskErr, errors.New("release cleanup differs"))
	}
	if err := removeCompletedReleaseOutputs(options, window); err != nil {
		return result.terminal("Fail", failureDomainHelper, "TaskRetirementChanged"), errors.Join(err, errors.New("release output cleanup differs"))
	}
	return result.terminal("Pass", failureDomainNone, "CleanupComplete"), nil
}

func removeCompletedReleaseOutputs(options ReleaseBaselineOptions, window deadline) error {
	root, script, err := prepareNativeAttempt(options.RuntimeRoot, "release-baseline")
	if err != nil {
		return err
	}
	defer removeOwnedPaths(root)
	arguments := []string{options.WorkingRoot, options.ScratchRoot, options.RecordRoot, options.RuntimeRoot}
	_, diagnostic, err := runNativeProcess(root, script, "release-cleanup", arguments, nativeEnvironment(root), 1<<20, "", window.remaining(), options.Cancellation, nil, nil)
	if message := strings.TrimSpace(diagnostic); err != nil && message != "" {
		err = errors.Join(err, errors.New(message))
	}
	return err
}

func validateReleaseCleanupBaseline(suite RustSuiteOptions) error {
	root := rustSuiteRecordRoot(suite)
	value, _, readErr := readRustSuiteBaselineFile(suite.BaselineSelectionPath, suite.Tag)
	if readErr != nil || value.Established == nil {
		return errors.Join(readErr, errors.New("cleanup requires established baseline facts"))
	}
	selection := value.Established.Selection
	bugfixDiff, sourceErr := releaseBugfixDiff(suite.Source, suite.RuntimeTemporaryRoot, suite.Tag, suite.deadline())
	if sourceErr != nil || selection.BugfixDiff != bugfixDiff {
		return errors.Join(sourceErr, errors.New("baseline source changed; remeasure affected coverage before cleanup"))
	}
	if len(selection.CoveredPackages)+len(selection.CompileOnlyPackages)+len(selection.HelperBinaries) == 0 {
		expected, expectedErr := zeroRustSuiteBaseline(suite)
		if expectedErr != nil || !rustSuiteSameValue(value, expected) {
			return errors.Join(expectedErr, errors.New("cleanup zero-coverage baseline differs"))
		}
		return nil
	}
	if _, err := os.Lstat(root); os.IsNotExist(err) {
		return nil
	}
	if _, err := os.Lstat(filepath.Join(root, "result.json")); err != nil {
		return errors.Join(err, errors.New("cleanup baseline terminal differs"))
	}
	retained, present, err := readRustSuiteResult(suite, true)
	if err == nil && (!present || retained.Terminal != "Pass" || retained.FailureDomain != failureDomainNone) {
		err = errors.New("cleanup baseline terminal differs")
	}
	return err
}

func RunBaselineCoverage(options BaselineCoverageOptions) (ReleaseRouteResult, error) {
	started := time.Now()
	result := releaseRouteResult("RollCoverage", options.Tag)
	missingSelection := options.Proposal.CompileOnlyPackages == nil || options.Proposal.CoveredPackages == nil || options.Proposal.HelperBinaries == nil
	onePatch := options.Unit != "" && options.FlatPatch == "" || options.Unit == "" && options.FlatPatch != ""
	name := options.Unit + options.FlatPatch
	if err := validateReleaseOptions(options.ReleaseBaselineOptions, true); err != nil || options.Align == onePatch || !options.Align && !validName(name, "-._") || missingSelection || !validRustSuitePackages(options.AffectedPackages) || !validRustSuiteRequiredTests(options.RequiredTests) || len(options.AffectedPackages) != 0 && (options.Align || options.FlatPatch == "") {
		return result, errors.Join(err, errors.New("invalid baseline coverage options"))
	}
	var inputErr error
	options.ReleaseBaselineOptions, inputErr = releaseBaselineInputs(options.ReleaseBaselineOptions, deadline{end: started.Add(core.StageDeadline()), signals: options.Cancellation})
	if inputErr != nil {
		return result, inputErr
	}
	suite := options.rustSuiteOptions()
	suite.baselineAffected = options.AffectedPackages
	suite.started, suite.end = started, started.Add(core.StageDeadline())
	calibrationPlan := RustSuitePlan{}
	suite.calibrationPlan = &calibrationPlan
	lock, err := cargoLock(options.ScratchRoot)
	if err != nil {
		return result.terminal("Fail", failureDomainHelper, "None"), err
	}
	defer releaseCargoLock(lock)
	if err := ensureCargoProjection(cacheProjection(options.WorkingRoot)); err != nil {
		return result.terminal("Fail", failureDomainHelper, "None"), err
	}
	toolchainErr := suite.phase("Toolchain")
	toolchain := ToolchainProjection{}
	if toolchainErr == nil {
		toolchain, toolchainErr = resolveToolchain(options.WorkingRoot, suite.Source, options.RuntimeRoot, suite.deadline())
	}
	value, preimage, baselineErr := readRustSuiteBaselineFile(suite.BaselineSelectionPath, suite.Tag)
	if toolchainErr != nil || baselineErr != nil || value.Established == nil || value.Established.Selection.CargoVersion != toolchain.CargoVersion || !validRustSuiteBaselineSelection(value.Established.Selection, options.Tag) {
		return result, errors.Join(toolchainErr, baselineErr, errors.New("current established baseline differs"))
	}
	proposal := options.Proposal
	proposal.Tag, proposal.ToolchainRoot, proposal.CargoVersion, proposal.ToolchainTarget = options.Tag, toolchain.Root, toolchain.CargoVersion, toolchain.Target
	proposal.ExecutionProfile = value.Established.Selection.ExecutionProfile
	proposal.BugfixDiff, err = releaseBugfixDiff(suite.Source, options.RuntimeRoot, options.Tag, suite.deadline())
	if err != nil {
		return result, err
	}
	if proposal.BugfixDiff != value.Established.Selection.BugfixDiff {
		if !options.Align && options.FlatPatch == "" {
			return result, errors.New("changed baseline source requires its owning bugfix Flat Patch")
		}
		fields, taskErr := taskGraphFields(options.taskBytes, "CurrentWorktree")
		currentDiff, currentErr := releaseBugfixDiff(fields["CurrentWorktree"], options.RuntimeRoot, options.Tag, suite.deadline())
		if taskErr != nil || currentErr != nil || currentDiff != proposal.BugfixDiff {
			return result, errors.Join(taskErr, currentErr, errors.New("Current and Baseline bugfix prefixes differ"))
		}
	}
	if !validRustSuiteBaselineProposal(proposal, options.Tag) {
		return result, errors.New("baseline coverage proposal differs")
	}
	suite.baselinePreimage = &value.Established.Selection
	suite.baselineProposal = &proposal
	suite.baselineReanchor = options.Align && len(proposal.CompileOnlyPackages)+len(proposal.HelperBinaries)+len(proposal.CoveredPackages) != 0
	packages, err := discoverRustSuitePackages(suite, toolchain)
	execution, aligned, executionErr := rustSuiteBaselineExecution(value.Established.Selection, proposal, packages, options.Align, options.AffectedPackages)
	if err != nil || executionErr != nil {
		if errors.Is(err, errNativeTimeout) || errors.Is(err, errOperationCanceled) || errors.Is(executionErr, errNativeTimeout) || errors.Is(executionErr, errOperationCanceled) {
			return result.terminal("Stopped", failureDomainHelper, "None"), errors.Join(err, executionErr)
		}
		return result, errors.Join(err, executionErr)
	}
	if zeroRustSuiteProposal(execution) && len(options.RequiredTests) != 0 {
		return result, errors.New("required regression tests need nonempty baseline execution")
	}
	sameSelection := rustSuiteSameValue(value.Established.Selection.rustSuiteBaselineProposal, proposal)
	if sameSelection && !suite.baselineReanchor {
		if zeroRustSuiteProposal(proposal) {
			return result.terminal("Pass", failureDomainNone, "BaselineCoverageUnchanged"), nil
		}
		prior := RustSuitePlan{}
		if _, readErr := readCanonicalValue(filepath.Join(rustSuiteRecordRoot(suite), "plan.json"), &prior); readErr == nil {
			suite.baselineReanchor = prior.BaselineReanchor
		}
		if retained, present, err := readRustSuiteResult(suite, true); err != nil || present && retained.Terminal != "Pass" {
			return rustSuiteRouteFailure(result, retained, err)
		}
		return result.terminal("Pass", failureDomainNone, "BaselineCoverageUnchanged"), nil
	}
	if zeroRustSuiteProposal(execution) {
		selection := value.Established.Selection
		selection.rustSuiteBaselineProposal = aligned
		selection.ExpectedFailures = map[string][]string{}
		selection.ExpectedTimeouts = map[string][]string{}
		encodeErr := publishRustSuiteBaselineWithin(suite, preimage, rustSuiteBaselineFile{Established: &rustSuiteBaselineGeneration{Selection: selection}}, "ResultPublication")
		if encodeErr != nil {
			suiteResult, _ := rustSuiteHelperResult(suite, encodeErr)
			return rustSuiteRouteFailure(result, suiteResult, encodeErr)
		}
		return result.terminal("Pass", failureDomainNone, "BaselineCoverageAligned"), nil
	}
	rustResult, runErr := RunRustSuite(suite)
	if runErr != nil || rustResult.Terminal != "Pass" {
		return rustSuiteRouteFailure(result, rustResult, runErr)
	}
	effect := "BaselineCoverageExpanded"
	if proposal.BugfixDiff != value.Established.Selection.BugfixDiff {
		effect = "BaselineBugfixCoverageRemeasured"
	}
	if options.Align {
		effect = "BaselineCoverageAligned"
	}
	if err := publishRustSuiteBaselineTiming(suite); err != nil {
		return result.terminal("Fail", failureDomainHelper, effect), err
	}
	return result.terminal("Pass", failureDomainNone, effect), nil
}

func releaseBugfixDiff(source, runtimeRoot, tag string, window deadline) (string, error) {
	data, err := runNativeWithin(runtimeRoot, "release-baseline-source", window.remaining(), window.signals, source, tag)
	var diff string
	if err == nil {
		err = json.Unmarshal(data, &diff)
	}
	return diff, err
}

func validateReleaseOptions(options ReleaseBaselineOptions, requireTag bool) error {
	for _, path := range []string{options.WorkingRoot, options.ScratchRoot, options.RuntimeRoot, options.RecordRoot, projectionTemporaryRoot(options.WorkingRoot)} {
		if noFollowPath("release route root", path) != nil {
			return errors.New("invalid release route roots")
		}
	}
	if options.admission == nil || (requireTag || options.Tag != "") && (!strings.HasPrefix(options.Tag, "rust-v") || !validName(options.Tag, "-._")) || options.RecordRoot != filepath.Join(options.ScratchRoot, "records") || pathsOverlap(options.RuntimeRoot, options.ScratchRoot) || pathsOverlap(options.WorkingRoot, options.ScratchRoot) {
		return errors.New("invalid release route options")
	}
	return nil
}

func runReleaseBaselineNative(options ReleaseBaselineOptions, action string, window deadline) (nativeReleaseBaselineResult, error) {
	root, script, err := prepareNativeAttempt(options.RuntimeRoot, "release-baseline")
	if err != nil {
		return nativeReleaseBaselineResult{}, err
	}
	defer removeOwnedPaths(root)
	output := filepath.Join(root, "result")
	arguments := []string{options.WorkingRoot, options.ScratchRoot, options.RecordRoot, options.Tag, options.taskPath, options.predecessorTag, options.worktrees.Main, options.worktrees.Upstream, string(options.taskBytes), string(options.indexBytes), string(options.nextIndex), output}
	data, stderr, runErr := runNativeProcess(root, script, action, arguments, nativeEnvironment(root), 8<<20, output, window.remaining(), options.Cancellation, options.OnLive, nil)
	result := nativeReleaseBaselineResult{}
	if diagnostic := strings.TrimSpace(stderr); runErr != nil && diagnostic != "" {
		runErr = errors.Join(runErr, errors.New(diagnostic))
	}
	if runErr == nil {
		runErr = json.Unmarshal(data, &result)
	}
	return result, runErr
}

type PatchReplayOptions struct {
	Resume bool
	endpointOptions
	RuntimeRoot, Mode, Source, Target, Tag string
	SkipLineLimit                          bool
	worktrees                              releaseWorktrees
	FinalizeSnapshots                      bool
	StartUnit, EndUnit                     string
	StartFlatPatch, EndFlatPatch           string
	StartTransform, EndTransform           string
	rustfmt                                string
	Lane, Unit, ChildPatch, FlatPatch      string
	Form, Segment                          string
	Contract                               PatchContract
	WorkingRoot, ScratchRoot, RecordRoot   string
	PredecessorTag                         string
	ExpectedTerminal                       string
	schemaSourceCommit                     string
	ChildPatchSequence                     string
	ExportOpen                             string
	ForkOpen                               string
	ForkRetainedParents                    string
	ParentFork                             string
	ActiveTask                             string
	taskPreimage                           []byte
	coverageMode                           string
	OnLive                                 func()
	OnPhase                                func(string)
	onTerminal                             func(nativePatchReplayResult) error
}

type PatchContract struct {
	Purpose, Provides, Boundary, Verification string
	Requires                                  []string
	CrossGroup                                string `json:",omitempty"`
}

type PatchReplayRow struct {
	Kind, Lane, Form, Segment string
	FlatPatch                 string         `json:",omitempty"`
	Unit                      string         `json:",omitempty"`
	ChildPatches              []string       `json:",omitempty"`
	Transform                 string         `json:",omitempty"`
	TransformPatch            string         `json:",omitempty"`
	Contract                  *PatchContract `json:",omitempty"`
}

type PatchReplayResult struct {
	SchemaVersion                             int
	RecordType, Mode, Terminal, FailureDomain string
	ProductEffect, Kind, Lane                 string
	FlatPatch, Unit, ChildPatch               string `json:",omitempty"`
	Transform, TransformPatch                 string `json:",omitempty"`
	FirstLaneChild, LastLaneChild             string
	Output                                    string
	ArtifactPath                              string `json:",omitempty"`
	ActualLines, MaximumLines                 int    `json:",omitempty"`
	AppliedRows                               []PatchReplayRow
	Rows                                      []PatchReplayRow
	Diagnostics                               []RustSuiteDiagnostic `json:",omitempty"`
}

type nativePatchReplayResult struct {
	Result, Kind, Lane, FlatPatch, Unit, ChildPatch string
	Transform, TransformPatch                       string
	FirstLaneChild, LastLaneChild, Output           string
	Diagnostic                                      string `json:",omitempty"`
	ArtifactPath                                    string `json:",omitempty"`
	ActualLines, MaximumLines                       int    `json:",omitempty"`
	AppliedRows                                     []PatchReplayRow
	Rows                                            []PatchReplayRow
	Diagnostics                                     []RustSuiteDiagnostic `json:",omitempty"`
}

func patchSelector(kind, identity string) string {
	if kind == "" || identity == "" {
		return ""
	}
	return kind + ":" + identity
}

func patchReplayRowSelector(row PatchReplayRow) string {
	name := ""
	switch row.Kind {
	case "FlatPatch":
		name = row.FlatPatch
	case "Unit":
		name = row.Unit
	case "Transform":
		name = row.Transform
	}
	if !validUnit(row.Lane + "/" + name) {
		return ""
	}
	return patchSelector(row.Kind, row.Lane+"/"+name)
}

func generatedTransformSelector(selector string) bool {
	kind, identity, found := strings.Cut(selector, ":")
	lane, name, separated := strings.Cut(identity, "/")
	return found && separated && kind == "Transform" && listed("regenerate-snapshots regenerate-app-server-schema", name) && listed("bugfix normalization backend mixed frontend cross draft", lane)
}

func onePatchSelector(unit, flatPatch, transform string) (string, error) {
	values := []struct{ kind, value string }{{"Unit", unit}, {"FlatPatch", flatPatch}, {"Transform", transform}}
	selector := ""
	for _, value := range values {
		if value.value == "" {
			continue
		}
		if selector != "" || !validUnit(value.value) {
			return "", errors.New("exactly one canonical Unit, Flat Patch, or Transform is required")
		}
		selector = patchSelector(value.kind, value.value)
	}
	if selector == "" {
		return "", errors.New("exactly one canonical Unit, Flat Patch, or Transform is required")
	}
	return selector, nil
}

func taskPatchSelector(data []byte, terminal bool) (string, error) {
	names := []string{"OpenUnit", "OpenFlatPatch", "OpenTransform"}
	if terminal {
		names = []string{"CurrentTerminalUnit", "CurrentTerminalFlatPatch", "CurrentTerminalTransform"}
	}
	fields, err := taskGraphFields(data, names...)
	if err != nil {
		return "", err
	}
	if !terminal && fields[names[0]] == "" && fields[names[1]] == "" && fields[names[2]] == "" {
		return "", nil
	}
	return onePatchSelector(fields[names[0]], fields[names[1]], fields[names[2]])
}

func (options PatchReplayOptions) startSelector() (string, error) {
	return onePatchSelector(options.StartUnit, options.StartFlatPatch, options.StartTransform)
}

func (options PatchReplayOptions) endSelector() (string, error) {
	if options.EndUnit == "" && options.EndFlatPatch == "" && options.EndTransform == "" {
		return "", nil
	}
	return onePatchSelector(options.EndUnit, options.EndFlatPatch, options.EndTransform)
}

func (options PatchReplayOptions) mutationIdentity() (string, string, string, error) {
	count := 0
	kind, name, patch := "", "", ""
	if options.FlatPatch != "" {
		count++
		kind, name, patch = "FlatPatch", options.FlatPatch, options.FlatPatch
	}
	if options.Unit != "" || options.ChildPatch != "" {
		count++
		kind, name, patch = "Unit", options.Unit, options.ChildPatch
	}
	identity := options.Lane + "/" + name
	if count != 1 || !validUnit(identity) || !validName(patch, "-") {
		return "", "", "", errors.New("exactly one Flat Patch or Unit Child Patch is required")
	}
	return kind, identity, patch, nil
}

func patchReplayResultSelector(result PatchReplayResult) string {
	name := ""
	switch result.Kind {
	case "FlatPatch":
		name = result.FlatPatch
	case "Unit":
		name = result.Unit
	case "Transform":
		name = result.Transform
	}
	if !validUnit(result.Lane + "/" + name) {
		return ""
	}
	return patchSelector(result.Kind, result.Lane+"/"+name)
}

func validPatchReplayLineFinding(options PatchReplayOptions, result PatchReplayResult) bool {
	valid := result.ArtifactPath != "" && result.MaximumLines == 1500 && (result.Output == "EmptyGitDiff" && result.ActualLines == 0 || result.Output == "OversizeGitDiff" && result.ActualLines > result.MaximumLines)
	if options.Mode == "export" {
		return valid
	}
	if !listed("commit revise", options.Mode) {
		return false
	}
	kind, identity, patch, err := options.mutationIdentity()
	validChild := kind != "Unit" && result.ChildPatch == "" || kind == "Unit" && result.ChildPatch != "" && (options.Mode == "commit" && result.ChildPatch == patch || options.Mode == "revise" && strings.Contains(", "+options.ChildPatchSequence+", ", ", "+result.ChildPatch+", "))
	return valid && err == nil && patchReplayResultSelector(result) == patchSelector(kind, identity) && validChild
}

func executePatchReplay(options PatchReplayOptions) (PatchReplayResult, error) {
	result := PatchReplayResult{
		SchemaVersion: core.SchemaVersion, RecordType: "PatchReplayResult", Mode: options.Mode,
		Terminal: "Blocked", FailureDomain: failureDomainInput, ProductEffect: "None",
		AppliedRows: []PatchReplayRow{}, Rows: []PatchReplayRow{},
	}
	if options.admission == nil {
		return result, errors.New("patch replay public admission is absent")
	}
	if err := validatePatchReplayOptions(options); err != nil {
		return result, err
	}
	if listed("apply commit revise", options.Mode) {
		options.rustfmt = filepath.Join(options.WorkingRoot, filepath.FromSlash(nativeToolchainRoot), "rust", "bin", "rustfmt")
		if err := noFollowPath("patch replay native rustfmt", options.rustfmt); err != nil {
			return result, err
		}
		info, err := os.Lstat(options.rustfmt)
		if err != nil || !info.Mode().IsRegular() || info.Mode().Perm()&0o111 == 0 {
			return result, errors.Join(err, errors.New("patch replay native rustfmt must be an existing executable regular file: "+options.rustfmt))
		}
	}
	task, err := activeTaskPath(options.ScratchRoot, options.Tag, options.PredecessorTag)
	if err != nil {
		return result, err
	}
	if task != options.ActiveTask {
		return result, errors.New("patch replay --active-task does not name the current indexed task")
	}
	taskBytes, err := readRegularNoFollow(task)
	if err != nil {
		return result, err
	}
	taskFields, err := taskGraphFields(taskBytes, "CurrentWorktree")
	if err != nil || taskFields["CurrentWorktree"] == "" {
		return result, errors.Join(err, errors.New("patch replay task CurrentWorktree is absent"))
	}
	options.worktrees, err = readReleaseWorktrees(taskFields["CurrentWorktree"], options.ScratchRoot, options.Tag, newDeadline(core.StageDeadline(), nil))
	if err != nil {
		return result, err
	}
	if options.Mode == "fork" {
		options.ForkOpen, options.ExpectedTerminal, options.ForkRetainedParents, err = validateForkTask(options, taskBytes)
		if err != nil {
			return result, err
		}
	}
	prefixes, err := taskSourcePrefixes(taskBytes, options.Tag)
	if err != nil {
		return result, err
	}
	sourcePrefix := ""
	if options.WorkingRoot != "" {
		for _, prefix := range prefixes {
			if options.Source == filepath.Join(exportRepositoryRoot(options.WorkingRoot), prefix) {
				sourcePrefix = prefix
			}
		}
	}
	exportedSource := sourcePrefix != ""
	if options.Mode == "list" && !exportedSource {
		options.coverageMode = "prefix"
	}
	if options.Mode == "apply" {
		if !options.worktrees.productPath(options.Tag, options.Target) {
			return result, errors.New("replay target is not this release's registered Current")
		}
		if exportedSource {
			end, _ := options.endSelector()
			if end != "" {
				return result, errors.New("exported-prefix Apply requires one start selector and forbids every --end-* selector")
			}
		}
		start, selectorErr := options.startSelector()
		options.ChildPatchSequence, err = applyTaskChildPatchSequence(taskBytes, start, options.Source, options.Target, exportedSource)
		err = errors.Join(selectorErr, err)
		if err != nil {
			return result, err
		}
		if !exportedSource && options.Source != "" {
			if _, err := taskPatchSelector(taskBytes, true); err != nil {
				return result, err
			}
			options.taskPreimage = taskBytes
		}
	}
	if options.Mode == "commit" {
		kind, identity, _, identityErr := options.mutationIdentity()
		options.ChildPatchSequence, err = taskPatchStructure(taskBytes, patchSelector(kind, identity), options.Target)
		err = errors.Join(identityErr, err)
		if err != nil {
			return result, err
		}
	}
	if options.Mode == "revise" {
		kind, identity, _, identityErr := options.mutationIdentity()
		options.ChildPatchSequence, options.ExpectedTerminal, err = patchTaskBoundary(taskBytes, patchSelector(kind, identity), options.Target)
		err = errors.Join(identityErr, err)
		if err != nil {
			return result, err
		}
	}
	if options.Mode == "config-schema" {
		open, fieldErr := taskPatchSelector(taskBytes, false)
		if fieldErr != nil || options.Target != taskFields["CurrentWorktree"] || !options.worktrees.productPath(options.Tag, options.Target) {
			return result, errors.Join(fieldErr, errors.New("config schema requires the registered current worktree and open Flat Patch or Unit"))
		}
		options.ChildPatchSequence, options.ExpectedTerminal, err = patchTaskBoundary(taskBytes, open, options.Target)
		if err != nil {
			return result, err
		}
	}
	if options.Mode == "finalize" {
		open, openErr := taskPatchSelector(taskBytes, false)
		if openErr != nil || options.Target != taskFields["CurrentWorktree"] || !options.worktrees.productPath(options.Tag, options.Target) {
			return result, errors.Join(openErr, errors.New("Finalize requires registered Current"))
		}
		options.FinalizeSnapshots = open == ""
		if options.Resume && !options.FinalizeSnapshots {
			return result, errors.New("resume selects the current full execution, not schema-only generation")
		}
		if options.FinalizeSnapshots {
			options.ExpectedTerminal, err = closedTaskTerminal(taskBytes, options.Target, options.Tag, options.worktrees)
			if err != nil {
				return result, err
			}
		} else if !strings.HasPrefix(open, "Unit:") && !strings.HasPrefix(open, "FlatPatch:") {
			return result, errors.New("focused schema Finalize requires an open semantic Patch")
		}
	}
	if options.Mode == "prepare" && (options.Target != taskFields["CurrentWorktree"] || !options.worktrees.productPath(options.Tag, options.Target)) {
		return result, errors.New("Prepare requires the registered Current worktree")
	}
	if options.Mode == "prepare" {
		options.ExpectedTerminal, err = taskPatchSelector(taskBytes, true)
		if err != nil {
			return result, err
		}
		if !generatedTransformSelector(options.ExpectedTerminal) {
			options.ExpectedTerminal = ""
		}
	}
	if exportedSource {
		options.PredecessorTag = sourcePrefix
	} else if options.Mode == "list" {
		fields, taskErr := taskGraphFields(taskBytes, "CurrentWorktree", "RetainedParents")
		allowed := fields["CurrentWorktree"] != "" && options.Source == fields["CurrentWorktree"]
		for _, parent := range strings.Split(fields["RetainedParents"], ", ") {
			allowed = allowed || parent != "" && options.Source == parent
		}
		if taskErr != nil || fields["CurrentWorktree"] == "" || !allowed {
			return result, errors.Join(taskErr, errors.New("live List source must be task CurrentWorktree or a retained parent; predecessor export requires project and record roots"))
		}
	}
	if options.Mode == "export" {
		options.Source = options.worktrees.Main
		if taskFields["CurrentWorktree"] == options.Source {
			options.ExportOpen, err = taskPatchSelector(taskBytes, false)
			if err != nil {
				return result, err
			}
		}
	}

	if options.Mode == "prepare" || options.Mode == "finalize" && options.FinalizeSnapshots {
		options.onTerminal = func(native nativePatchReplayResult) error {
			if native.Output == "Committed" {
				result.ProductEffect = "TargetCommit"
			}
			if len(native.Rows) == 0 {
				return nil
			}
			before, fieldErr := taskPatchSelector(taskBytes, true)
			after := patchReplayRowSelector(native.Rows[len(native.Rows)-1])
			if fieldErr != nil || before == after {
				return fieldErr
			}
			postimage, postErr := taskTerminalPostimage(taskBytes, before, after)
			current, indexErr := activeTaskPath(options.ScratchRoot, options.Tag, options.PredecessorTag)
			if postErr != nil || indexErr != nil || current != options.ActiveTask {
				return errors.Join(postErr, indexErr, errors.New("generated task terminal publication failed"))
			}
			if err := publishExactPostimage(options.ActiveTask, taskBytes, postimage); err != nil {
				return err
			}
			taskBytes = postimage
			return nil
		}
	}
	native, stderr, err := runPatchReplayNative(options)
	if native.Result == "ApplyFailed" {
		err = errors.Join(err, errors.New(native.Output))
	}
	currentTask, taskErr := readRegularNoFollow(options.ActiveTask)
	if taskErr != nil {
		err = errors.Join(err, taskErr, errors.New("patch replay active task is unreadable after execution"))
	} else if len(options.taskPreimage) == 0 && !bytes.Equal(currentTask, taskBytes) {
		err = errors.Join(err, errors.New("patch replay active task changed"))
	}
	if err == nil && native.Result == "Finalized" && listed("Current Committed", native.Output) {
		runner := patchReplayRunnerOptions(options, nil)
		err = removeOwnedPaths(runnerTerminalPath(runner), runnerOutputRoot(runner), filepath.Join(options.RecordRoot, "process", runner.AttemptID))
	}
	result.Kind, result.Lane = native.Kind, native.Lane
	result.FlatPatch, result.Unit, result.ChildPatch = native.FlatPatch, native.Unit, native.ChildPatch
	result.Transform, result.TransformPatch = native.Transform, native.TransformPatch
	result.FirstLaneChild, result.LastLaneChild = native.FirstLaneChild, native.LastLaneChild
	result.Output, result.AppliedRows, result.Rows = native.Output, native.AppliedRows, native.Rows
	result.ArtifactPath, result.ActualLines, result.MaximumLines = native.ArtifactPath, native.ActualLines, native.MaximumLines
	result.Diagnostics = native.Diagnostics
	if err != nil {
		result.Terminal, result.FailureDomain = "Fail", failureDomainHelper
		if options.Mode == "apply" && len(native.AppliedRows) != 0 {
			result.ProductEffect = "TargetPrefix"
		}
		if native.Result == "Finalized" && native.Output == "Committed" {
			result.ProductEffect = "TargetCommit"
		}
		if native.Result == "ConfigSchemaGenerated" && native.Output == "Changed" {
			result.ProductEffect = "ConfigSchemaPostimage"
		}
		if native.Result == "Prepared" && native.Output == "Removed" {
			result.ProductEffect = "GeneratedTailRemoved"
		}
		if listed("Exported PublicationFailed", native.Result) && result.Output == filepath.Join(exportRepositoryRoot(options.WorkingRoot), options.Tag) {
			result.ProductEffect = "PublishedPrefix"
		}
		return result, errors.Join(err, errors.New(strings.TrimSpace(stderr)))
	}
	switch native.Result {
	case "ConfigSchemaGenerated":
		if options.Mode != "config-schema" || patchReplayResultSelector(result) != options.ExpectedTerminal || !listed("Current Changed", result.Output) || result.ArtifactPath != "codex-rs/core/config.schema.json" {
			return result, errors.New("config schema generation result differs")
		}
		result.Terminal, result.FailureDomain = "Pass", failureDomainNone
		if result.Output == "Changed" {
			result.ProductEffect = "ConfigSchemaPostimage"
		}
	case "Forked":
		ordinal := filepath.Base(result.Output)
		if filepath.Dir(result.Output) != options.worktrees.ForkRoot || !validForkOrdinal(ordinal) {
			return result, errors.New("patch replay fork result differs")
		}
		result.Terminal, result.FailureDomain, result.ProductEffect = "Pass", failureDomainNone, "WorktreeCreated"
	case "Listed":
		if len(result.Rows) == 0 {
			return result, errors.New("patch replay row list is empty")
		}
		result.Terminal, result.FailureDomain = "Pass", failureDomainNone
	case "Prepared":
		if options.Mode != "prepare" || !listed("Current Removed", result.Output) || result.Output == "Removed" && len(result.Rows) == 0 {
			return result, errors.New("source preparation result differs")
		}
		result.Terminal, result.FailureDomain = "Pass", failureDomainNone
		if result.Output == "Removed" {
			result.ProductEffect = "GeneratedTailRemoved"
		}
	case "Committed":
		kind, identity, patch, identityErr := options.mutationIdentity()
		if identityErr != nil || patchReplayResultSelector(result) != patchSelector(kind, identity) || kind == "Unit" && result.ChildPatch != patch {
			return result, errors.New("patch replay commit result differs")
		}
		result.Terminal, result.FailureDomain, result.ProductEffect = "Pass", failureDomainNone, "TargetCommit"
	case "Revised":
		kind, identity, patch, identityErr := options.mutationIdentity()
		resultPatch := result.ChildPatch
		if kind == "FlatPatch" {
			resultPatch = options.FlatPatch
		}
		if identityErr != nil || patchReplayResultSelector(result) != patchSelector(kind, identity) || resultPatch != patch {
			return result, errors.New("patch replay revise result differs")
		}
		result.Terminal, result.FailureDomain, result.ProductEffect = "Pass", failureDomainNone, "TargetPrefix"
	case "Applied":
		start, startErr := options.startSelector()
		last, lastErr := options.endSelector()
		if last == "" {
			last = start
		}
		emptyGenerated := len(result.AppliedRows) == 0 && generatedTransformSelector(start) && generatedTransformSelector(last)
		if startErr != nil || lastErr != nil || result.FirstLaneChild != start || result.LastLaneChild != last || len(result.AppliedRows) == 0 && !emptyGenerated {
			return result, errors.New("patch replay apply result differs")
		}
		result.Terminal, result.FailureDomain = "Pass", failureDomainNone
		if !emptyGenerated {
			result.ProductEffect = "TargetPrefix"
		}
	case "Finalized":
		transform := "regenerate-app-server-schema"
		if options.FinalizeSnapshots {
			transform = "regenerate-snapshots"
		}
		expected := patchSelector("Transform", options.Lane+"/"+transform)
		if patchReplayResultSelector(result) != expected || !listed("Current Committed", result.Output) {
			return result, errors.New("patch replay finalize result differs")
		}
		result.Terminal, result.FailureDomain = "Pass", failureDomainNone
		if result.Output == "Committed" {
			result.ProductEffect = "TargetCommit"
		}
	case "Input":
		result.Terminal, result.FailureDomain = "Blocked", failureDomainInput
	case "Conflict":
		if patchReplayResultSelector(result) == "" {
			return result, errors.New("patch replay conflict result differs")
		}
		if len(result.AppliedRows) != 0 {
			result.ProductEffect = "TargetPrefix"
		}
		if strings.HasPrefix(result.Output, "RepositoryFormat") && strings.TrimSpace(stderr) != "" {
			diagnostic := &diagnosticWriter{}
			_, _ = diagnostic.Write([]byte(result.Output + "\n" + strings.TrimSpace(stderr)))
			result.Output = diagnostic.String()
		}
		if strings.HasPrefix(result.Output, "RepositoryFormatHelper:") {
			result.Terminal, result.FailureDomain = "Fail", failureDomainHelper
			break
		}
		sizeFinding := listed("EmptyGitDiff OversizeGitDiff", result.Output)
		if sizeFinding && !validPatchReplayLineFinding(options, result) {
			return result, errors.New("GitDiff finding differs")
		}
		if options.Mode == "export" && !sizeFinding {
			return result, errors.New("export finding differs")
		}
		result.Terminal, result.FailureDomain = "Required", failureDomainProduct
	case "Exported", "PublicationFailed":
		expectedOutput := filepath.Join(exportRepositoryRoot(options.WorkingRoot), options.Tag)
		if options.Mode != "export" || result.Output != expectedOutput || len(result.Rows) == 0 {
			return result, errors.New("patch replay export result differs")
		}
		result.ProductEffect = "PublishedPrefix"
		if native.Result == "PublicationFailed" {
			result.Terminal, result.FailureDomain = "Fail", failureDomainHelper
			return result, errors.New("export publication cleanup failed: " + native.Diagnostic)
		}
		result.Terminal, result.FailureDomain = "Pass", failureDomainNone
	default:
		return result, errors.New("patch replay native result differs")
	}

	return result, err
}

func validUnit(value string) bool {
	parts := strings.Split(value, "/")
	return len(parts) == 2 && listed("bugfix normalization backend mixed frontend cross draft", parts[0]) && validName(parts[1], "-")
}

func validForkOrdinal(value string) bool {
	return len(value) == 3 && value != "000" && strings.Trim(value, "0123456789") == ""
}

func currentWorktreeTask(data []byte, source, tag string, roles releaseWorktrees) (map[string]string, error) {
	fields, err := taskGraphFields(data, "CurrentWorktree", "RetainedParents")
	if err != nil {
		return nil, err
	}
	terminal, terminalErr := taskPatchSelector(data, true)
	open, openErr := taskPatchSelector(data, false)
	if openErr != nil || terminalErr != nil || fields["CurrentWorktree"] != source || !roles.productPath(tag, source) {
		return nil, errors.Join(openErr, terminalErr, errors.New("current registered worktree task frontier differs"))
	}
	fields["OpenPatch"], fields["TerminalPatch"] = open, terminal
	if source == roles.Main {
		if fields["RetainedParents"] != "" {
			return nil, errors.New("Main task requires completed retained-parent absorption")
		}
		return fields, nil
	}
	if fields["RetainedParents"] == "" {
		return nil, errors.New("current repair worktree has no retained parent")
	}
	known := map[string]bool{}
	for _, parent := range strings.Split(fields["RetainedParents"], ", ") {
		if !roles.productPath(tag, parent) || parent == source || known[parent] {
			return nil, errors.New("current worktree retained parent differs")
		}
		known[parent] = true
	}
	if !known[roles.Main] {
		return nil, errors.New("fork task requires its retained Main")
	}
	return fields, nil
}

func closedTaskTerminal(data []byte, source, tag string, roles releaseWorktrees) (string, error) {
	fields, err := currentWorktreeTask(data, source, tag, roles)
	if err != nil || fields["OpenPatch"] != "" {
		return "", errors.Join(err, errors.New("closed task frontier differs"))
	}
	return fields["TerminalPatch"], nil
}

func taskTerminalPostimage(data []byte, before, after string) ([]byte, error) {
	oldKind, oldName, oldOK := strings.Cut(before, ":")
	newKind, newName, newOK := strings.Cut(after, ":")
	if !oldOK || !newOK || before == after {
		return nil, errors.New("generated task terminal identity differs")
	}
	pattern := regexp.MustCompile("(?m)^([+`]-- )CurrentTerminal" + regexp.QuoteMeta(oldKind+": "+oldName) + "$")
	if len(pattern.FindAllIndex(data, -1)) != 1 {
		return nil, errors.New("generated task terminal preimage differs")
	}
	return pattern.ReplaceAll(data, []byte("${1}CurrentTerminal"+newKind+": "+newName)), nil
}

func validateReleaseForkInventory(roles releaseWorktrees, allowed map[string]bool) error {
	for _, path := range roles.Forks {
		if !allowed[path] {
			return errors.New("unmentioned or superseded release repair worktree: " + path)
		}
	}
	entries, err := os.ReadDir(roles.ForkRoot)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	for _, entry := range entries {
		path := filepath.Join(roles.ForkRoot, entry.Name())
		if !validForkOrdinal(entry.Name()) || roles.Forks[entry.Name()] != path || !allowed[path] {
			return errors.New("unmentioned release repair directory: " + path)
		}
	}
	return nil
}

const retainedParentsTaskRequirement = "RetainedParents: omit the whole field when Main has no retained parents; otherwise require unique existing absolute parent worktree paths joined by comma-space, never a blank or placeholder value"

func taskGraphFields(data []byte, names ...string) (map[string]string, error) {
	required := map[string]bool{}
	for _, name := range names {
		required[name] = true
	}
	fields, seen := map[string]string{}, map[string]bool{}
	for _, line := range strings.Split(string(data), "\n") {
		body, found := strings.CutPrefix(line, "+-- ")
		if !found {
			body, found = strings.CutPrefix(line, "`-- ")
			if !found {
				continue
			}
		}
		name, rawValue, found := strings.Cut(body, ":")
		if !required[name] {
			continue
		}
		value, spaced := strings.CutPrefix(rawValue, " ")
		if !found || !spaced || value == "" || strings.TrimSpace(value) != value || seen[name] {
			if name == "RetainedParents" {
				return nil, errors.New(retainedParentsTaskRequirement)
			}
			return nil, errors.New("backward convergence task fields differ")
		}
		fields[name], seen[name] = value, true
	}
	return fields, nil
}

func applyTaskChildPatchSequence(data []byte, selector, source, target string, exported bool) (string, error) {
	sequence, structureErr := taskPatchStructure(data, selector, target)
	if source == "" {
		return sequence, structureErr
	}
	fields, err := taskGraphFields(data, "RetainedParents")
	sourceRetained := exported
	for _, parent := range strings.Split(fields["RetainedParents"], ", ") {
		sourceRetained = sourceRetained || parent == source
	}
	if structureErr != nil || err != nil || !sourceRetained {
		return "", errors.Join(structureErr, err, errors.New("patch replay task patch structure differs"))
	}
	return sequence, nil
}

func taskPatchStructure(data []byte, selector, target string) (string, error) {
	fields, err := taskGraphFields(data, "CurrentWorktree", "ChildPatchSequence")
	open, openErr := taskPatchSelector(data, false)
	sequence := fields["ChildPatchSequence"]
	patches, seen := strings.Split(sequence, ", "), map[string]bool{}
	valid := err == nil && openErr == nil && fields["CurrentWorktree"] == target && open == selector
	if strings.HasPrefix(selector, "Unit:") {
		valid = valid && len(patches) >= 2
		for _, patch := range patches {
			valid = valid && validName(patch, "-") && !seen[patch]
			seen[patch] = true
		}
	} else {
		valid = valid && sequence == ""
	}
	if !valid {
		return "", errors.Join(err, openErr, errors.New("patch replay task patch structure differs"))
	}
	return sequence, nil

}

func patchTaskBoundary(data []byte, selector, target string) (string, string, error) {
	sequence, structureErr := taskPatchStructure(data, selector, target)
	terminal, terminalErr := taskPatchSelector(data, true)
	if structureErr != nil || terminalErr != nil || terminal != selector {
		return "", "", errors.Join(structureErr, terminalErr, errors.New("patch replay in-place correction requires the terminal Flat Patch or Unit; earlier correction requires Fork"))
	}
	return sequence, terminal, nil
}

func validateForkTask(options PatchReplayOptions, data []byte) (string, string, string, error) {
	fields, err := taskGraphFields(data, "CurrentWorktree", "RetainedParents")
	if err != nil {
		return "", "", "", err
	}
	openPatch, openErr := taskPatchSelector(data, false)
	terminalPatch, terminalErr := taskPatchSelector(data, true)
	roles := options.worktrees
	current := roles.parent(options.Tag, options.ParentFork)
	retained := fields["RetainedParents"]
	if current == "" || options.ParentFork != "" && retained == "" {
		return "", "", "", errors.New(retainedParentsTaskRequirement)
	}
	valid := openErr == nil && openPatch != "" && terminalErr == nil && fields["CurrentWorktree"] == current
	allowed := map[string]bool{current: true}
	if retained != "" {
		for _, parent := range strings.Split(retained, ", ") {
			if !roles.productPath(options.Tag, parent) || parent == current || allowed[parent] {
				return "", "", "", errors.New(retainedParentsTaskRequirement + "; invalid parent: " + parent)
			}
			allowed[parent] = true
		}
	}
	inventoryErr := validateReleaseForkInventory(roles, allowed)
	listedOptions := options
	listedOptions.Mode, listedOptions.Source, listedOptions.WorkingRoot, listedOptions.RecordRoot, listedOptions.PredecessorTag = "list", current, "", "", ""
	listedResult, _, listErr := runPatchReplayNative(listedOptions)
	last := len(listedResult.Rows) - 1
	if !valid || inventoryErr != nil || listErr != nil || listedResult.Result != "Listed" || last < 0 || patchReplayRowSelector(listedResult.Rows[last]) != terminalPatch {
		return "", "", "", errors.Join(openErr, terminalErr, inventoryErr, listErr, errors.New("patch replay fork task differs"))
	}
	return openPatch, terminalPatch, retained, nil
}

func completedTaskTerminal(data []byte, source, tag string, roles releaseWorktrees) (string, error) {
	fields, err := currentWorktreeTask(data, source, tag, roles)
	if err != nil || source != roles.Main || fields["RetainedParents"] != "" || fields["OpenPatch"] != "" {
		return "", errors.Join(err, errors.New("currently requested Main task frontier differs"))
	}
	return fields["TerminalPatch"], nil
}

func validatePatchReplayOptions(options PatchReplayOptions) error {
	valid := listed("fork list prepare apply revise finalize config-schema commit export", options.Mode) && strings.HasPrefix(options.Tag, "rust-v") && validName(options.Tag, "-._") && noFollowPath("patch replay runtime", options.RuntimeRoot) == nil && noFollowPath("patch replay scratch", options.ScratchRoot) == nil && noFollowPath("patch replay active task", options.ActiveTask) == nil
	valid = valid && (options.Source == "" || noFollowPath("patch replay source", options.Source) == nil) && (options.Target == "" || noFollowPath("patch replay target", options.Target) == nil)
	valid = valid && (!options.SkipLineLimit || listed("commit export", options.Mode))
	switch options.Mode {
	case "fork":
		valid = valid && (options.ParentFork == "" || validForkOrdinal(options.ParentFork))
	case "prepare":
		valid = valid && options.Target != "" && containedBy(options.ScratchRoot, options.Target)
	case "list":
		valid = valid && options.Source != "" && (options.WorkingRoot == "" && options.RecordRoot == "" || noFollowPath("patch replay export root", options.WorkingRoot) == nil && noFollowPath("patch replay record root", options.RecordRoot) == nil && containedBy(options.ScratchRoot, options.RecordRoot) && !pathsOverlap(options.RuntimeRoot, options.RecordRoot))
		if options.StartUnit != "" || options.StartFlatPatch != "" {
			_, selectionErr := options.startSelector()
			valid = valid && selectionErr == nil
		}
	case "apply":
		start, startErr := options.startSelector()
		end, endErr := options.endSelector()
		if startErr != nil {
			return errors.New("Apply requires one canonical --start-unit, --start-flat-patch, or --start-transform Lane/Patch")
		}
		if endErr != nil {
			return errors.New("Apply permits at most one canonical --end-* Lane/Patch")
		}
		valid = valid && options.Target != ""
		if start == "Transform:normalization/normalize-terminal-chrome-for-powerline" {
			valid = valid && options.Source == "" && end == ""
		} else {
			valid = valid && options.Source != options.Target && options.Source != ""
		}
	case "commit", "revise":
		_, _, _, identityErr := options.mutationIdentity()
		segments := "none snapshot"
		if listed("backend mixed cross draft", options.Lane) {
			segments = "none schema snapshot schema-snapshot"
		}
		if !listed(segments, options.Segment) {
			return fmt.Errorf("Lane=%.256s Field=Segment Expected=%s Actual=%.256s", options.Lane, segments, options.Segment)
		}
		valid = valid && identityErr == nil && options.Target != "" && options.Form == "GitDiff"
	case "finalize", "config-schema":
		valid = valid && options.Target != "" && (options.Mode == "config-schema" || listed("bugfix normalization backend mixed frontend cross draft", options.Lane))
	case "export":
		valid = valid && options.Source == ""
	}
	if listed("apply commit revise finalize config-schema export", options.Mode) {
		for _, root := range []string{options.WorkingRoot, options.RecordRoot} {
			valid = valid && noFollowPath("patch replay "+options.Mode+" root", root) == nil
		}
		valid = valid && containedBy(options.ScratchRoot, options.RecordRoot) && !pathsOverlap(options.RuntimeRoot, options.RecordRoot)
	}
	if !valid {
		return errors.New("invalid patch replay options")
	}
	return nil
}

func runPatchReplayNative(options PatchReplayOptions) (nativePatchReplayResult, string, error) {
	cancellation, releaseController, err := controllerSignals()
	if err != nil {
		return nativePatchReplayResult{}, "", err
	}
	defer releaseController()
	window := newDeadline(core.StageDeadline(), cancellation)
	native, stderr, err := runPatchReplayProcess(options, "", window)
	if err == nil && options.Mode == "config-schema" && native.Result == "ConfigSchemaProjectionRequired" {
		result := PatchReplayResult{Kind: native.Kind, Lane: native.Lane, FlatPatch: native.FlatPatch, Unit: native.Unit, Transform: native.Transform}
		if patchReplayResultSelector(result) != options.ExpectedTerminal || !regexp.MustCompile(`^(?:[0-9a-f]{40}|[0-9a-f]{64})$`).MatchString(native.Output) {
			return nativePatchReplayResult{}, stderr, errors.New("config schema projection request differs")
		}
		return runConfigSchemaProjection(options, native.Output, window)
	}
	if err != nil {
		return native, stderr, err
	}
	if native.Result == "ProjectionRequired" {
		result := PatchReplayResult{Kind: native.Kind, Lane: native.Lane, Transform: native.Transform}
		if patchReplayResultSelector(result) != patchSelector("Transform", options.Lane+"/regenerate-app-server-schema") || native.Output != "" {
			return nativePatchReplayResult{}, stderr, errors.New("patch replay projection request differs")
		}
		native, stderr, err = runPatchReplayProjection(options, cancellation)
		if err != nil {
			return native, stderr, err
		}
	}
	if native.Result == "SnapshotProjectionRequired" {
		result := PatchReplayResult{Kind: native.Kind, Lane: native.Lane, Transform: native.Transform}
		if !options.FinalizeSnapshots || patchReplayResultSelector(result) != patchSelector("Transform", options.Lane+"/regenerate-snapshots") || !listed("Current Committed", native.Output) {
			return native, stderr, errors.New("snapshot projection request differs")
		}
		return runPatchReplaySnapshotProjection(options, cancellation)
	}
	return native, stderr, nil
}

func runPatchReplayProcess(options PatchReplayOptions, postimages string, window deadline) (nativePatchReplayResult, string, error) {
	if err := window.check(); err != nil {
		return nativePatchReplayResult{}, "", err
	}
	root, script, err := prepareNativeAttempt(options.RuntimeRoot, "patch-replay")
	if err != nil {
		return nativePatchReplayResult{}, "", err
	}
	defer removeOwnedPaths(root)
	if listed("apply commit revise", options.Mode) {
		if err := writeRuntimeAsset(filepath.Join(root, "normalize-tui-terminal-chrome.py"), assets.TerminalChromeTransform(), 0o600); err != nil {
			return nativePatchReplayResult{}, "", err
		}
	}
	action, arguments := "patch-replay-"+options.Mode, []string{}
	environment := nativeEnvironment(root)
	contract := ""
	if options.Contract.Purpose != "" || options.Contract.Provides != "" || options.Contract.Boundary != "" || options.Contract.Verification != "" || len(options.Contract.Requires) != 0 || options.Contract.CrossGroup != "" {
		data, encodeErr := json.Marshal(options.Contract)
		if encodeErr != nil {
			return nativePatchReplayResult{}, "", encodeErr
		}
		contract = string(data)
	}
	switch options.Mode {
	case "fork":
		arguments = []string{options.ScratchRoot, options.Tag, options.ParentFork, options.ForkOpen, options.ExpectedTerminal, options.ForkRetainedParents, options.worktrees.Main, options.worktrees.parent(options.Tag, options.ParentFork), options.worktrees.ForkRoot}
	case "list":
		selector := ""
		if options.StartUnit != "" || options.StartFlatPatch != "" {
			selector, _ = options.startSelector()
		}
		arguments = []string{options.Source, options.Tag, options.PredecessorTag, selector, options.WorkingRoot, options.ActiveTask, options.coverageMode}
	case "apply":
		environment = repositoryFormatProxyEnvironment(environment)
		start, _ := options.startSelector()
		end, _ := options.endSelector()
		arguments = []string{options.Source, options.Target, options.Tag, start, end, options.rustfmt, root, options.PredecessorTag, options.ChildPatchSequence}
		arguments = append(arguments, filepath.Join(options.SkillRoot, "bin"), options.WorkingRoot, options.ScratchRoot, options.RuntimeRoot, core.StageDeadline().String())
		arguments = append(arguments, options.ActiveTask, string(options.taskPreimage))
	case "revise":
		kind, identity, patch, _ := options.mutationIdentity()
		_, name, _ := strings.Cut(identity, "/")
		arguments = []string{options.Target, options.Tag, options.Lane, kind, name, patch, options.Form, options.Segment, options.rustfmt, root, options.ChildPatchSequence, options.ExpectedTerminal, contract, options.WorkingRoot, options.ActiveTask}
	case "prepare":
		arguments = []string{options.Target, options.Tag, options.ExpectedTerminal}
	case "finalize":
		arguments = []string{options.Target, options.Tag, options.Lane, strconv.FormatBool(options.FinalizeSnapshots), options.ExpectedTerminal, postimages, options.schemaSourceCommit}
	case "config-schema":
		arguments = []string{options.Target, options.Tag, options.ExpectedTerminal, options.ChildPatchSequence, postimages}
	case "commit":
		kind, identity, patch, _ := options.mutationIdentity()
		_, name, _ := strings.Cut(identity, "/")
		arguments = []string{options.Target, options.Tag, options.Lane, kind, name, patch, options.Form, options.Segment, options.rustfmt, options.ChildPatchSequence, strconv.FormatBool(options.SkipLineLimit), contract, options.WorkingRoot, options.ActiveTask}
	case "export":
		arguments = []string{options.Source, options.WorkingRoot, options.ScratchRoot, exportRepositoryRoot(options.WorkingRoot), options.Tag, options.ExportOpen, strconv.FormatBool(options.SkipLineLimit), options.ActiveTask}
	}
	output := filepath.Join(root, "result")
	arguments = append(arguments, output)
	data, stderr, runErr := runNativeProcess(root, script, action, arguments, environment, 8<<20, output, window.remaining(), window.signals, options.OnLive, options.OnPhase)
	native := nativePatchReplayResult{}
	if decodeErr := json.Unmarshal(data, &native); runErr == nil || decodeErr == nil {
		runErr = decodeErr
	}
	if runErr == nil && options.onTerminal != nil {
		runErr = options.onTerminal(native)
	}
	return native, stderr, runErr
}

func patchReplayRunnerOptions(options PatchReplayOptions, cancellation <-chan os.Signal) RunnerExecOptions {
	return RunnerExecOptions{
		endpointOptions:    options.endpointOptions,
		ProjectWorkingRoot: options.WorkingRoot, ProjectScratchRoot: options.ScratchRoot,
		RuntimeTemporaryRoot: options.RuntimeRoot, RecordRoot: options.RecordRoot, Source: options.Target, Release: options.Tag,
		WorkingDirectory: "codex-rs", Arguments: []string{"/usr/bin/python3", "-s", "/runner/regenerate-app-server-schema.py"},
		AttemptID: "patch-replay-" + options.Tag + "-" + options.Lane + "-" + filepath.Base(options.Target), Deadline: core.StageDeadline(), Cancellation: cancellation, OnLive: options.OnLive,
		consumer:     runnerConsumer{assets: []runtimeAsset{{name: "regenerate-app-server-schema.py", data: assets.AppServerSchemaTransform(), mode: 0o600}}},
		resumeReaped: true,
	}
}

func patchReplaySnapshotRunnerOptions(options PatchReplayOptions, cancellation <-chan os.Signal) (RunnerExecOptions, RustSuitePlan, error) {
	source, err := runnerSourceProjection(options.Target, options.WorkingRoot, options.ScratchRoot, options.RuntimeRoot, options.Tag)
	if err != nil || !options.worktrees.productPath(options.Tag, options.Target) || !validRustSuiteProductBranch(options.Tag, source.SourceBranch) {
		return RunnerExecOptions{}, RustSuitePlan{}, errors.Join(err, errors.New("snapshot Finalize source role differs"))
	}
	data, err := repositoryGitOutput(options.Target, "rev-parse", "HEAD")
	sourceCommit := strings.TrimSuffix(string(data), "\n")
	if err != nil || !validGitCommit(sourceCommit) {
		return RunnerExecOptions{}, RustSuitePlan{}, errors.Join(err, errors.New("snapshot Finalize source commit differs"))
	}
	suite := RustSuiteOptions{
		endpointOptions: options.endpointOptions, ProjectWorkingRoot: options.WorkingRoot, ProjectScratchRoot: options.ScratchRoot,
		RuntimeTemporaryRoot: options.RuntimeRoot, RecordRoot: options.RecordRoot, Source: options.Target, Scope: "full", Tag: options.Tag,
		BaselineSelectionPath: filepath.Join(options.ScratchRoot, "tasks", "rust-suite-baselines.json"), Cancellation: cancellation, OnLive: options.OnLive, OnPhase: options.OnPhase, started: time.Now(),
	}
	lock, err := cargoLock(options.ScratchRoot)
	if err != nil {
		return RunnerExecOptions{}, RustSuitePlan{}, err
	}
	defer releaseCargoLock(lock)
	if err := suite.startDeadline(); err != nil {
		return RunnerExecOptions{}, RustSuitePlan{}, err
	}
	benchmark := suite.end.Sub(suite.started)
	suite.end = suite.end.Add(benchmark/2 + benchmark/4 + core.CleanupDeadline())
	toolchain, err := resolveToolchain(options.WorkingRoot, options.Target, options.RuntimeRoot, suite.deadline())
	if err != nil {
		return RunnerExecOptions{}, RustSuitePlan{}, err
	}
	plan, err := establishedRustSuitePlan(suite, toolchain, "ExplicitProductWorktree")
	if err != nil {
		return RunnerExecOptions{}, RustSuitePlan{}, err
	}
	runner := RunnerExecOptions{
		endpointOptions:    options.endpointOptions,
		ProjectWorkingRoot: options.WorkingRoot, ProjectScratchRoot: options.ScratchRoot,
		RuntimeTemporaryRoot: options.RuntimeRoot, RecordRoot: options.RecordRoot, Source: options.Target, Release: options.Tag,
		WorkingDirectory: "codex-rs", Arguments: rustSuiteCommand(plan),
		AttemptID: "full",
		Deadline:  suite.end.Sub(suite.started), Cancellation: cancellation, OnLive: options.OnLive,
		consumer:          rustSuiteConsumer(plan),
		roundBudget:       benchmark,
		completeSnapshots: true,
		resumeReaped:      true,
		replaceCurrent:    !options.Resume,
		resultFiles:       []string{"rust-suite-preparation.json", "nextest.jsonl"},
		sourceCommit:      sourceCommit,
		onPhase: func(phase string) error {
			if options.OnPhase != nil {
				options.OnPhase(phase)
			}
			return nil
		},
		selectionEnd: suite.end,
	}
	return runner, plan, nil
}

func runPatchReplaySnapshotProjection(options PatchReplayOptions, cancellation <-chan os.Signal) (nativePatchReplayResult, string, error) {
	runner, plan, err := patchReplaySnapshotRunnerOptions(options, cancellation)
	if err != nil {
		return nativePatchReplayResult{}, "", err
	}
	var terminal RunnerExecResult
	var runErr error
	if options.Resume {
		terminal, runErr = consumeRunnerTerminal(runner)
	}
	if !options.Resume || os.IsNotExist(runErr) {
		terminal, runErr = executeRunner(runner)
	}
	detail := runnerFailureDiagnostic(terminal)
	if terminal.Preparation == nil || terminal.Observation == nil || !terminal.ExecutionStarted || terminal.Preparation.SourceCommit != runner.sourceCommit {
		return nativePatchReplayResult{}, detail, errors.Join(runErr, errors.New("snapshot Finalize terminal input differs"))
	}
	suite := RustSuiteOptions{ProjectWorkingRoot: runner.ProjectWorkingRoot, end: time.Now().Add(core.CleanupDeadline())}
	stage := rustSuiteReapedStage(suite, rustSuitePreflight{Plan: plan}, snapshotSuiteProjection(terminal, runner.ProjectWorkingRoot), runErr)
	if stage.Terminal != "Pass" {
		if stage.Terminal == "Fail" {
			return nativePatchReplayResult{
				Result: "Conflict", Kind: "Transform", Lane: options.Lane, Transform: "regenerate-snapshots",
				Output: "SnapshotGenerationFailed", Diagnostics: rustSuiteResultForStage(plan, stage).Diagnostics,
			}, detail, nil
		}
		return nativePatchReplayResult{}, detail, errors.Join(runErr, errors.New("snapshot transform projection failed"))
	}
	window := deadline{runner.selectionEnd, cancellation}
	if err := validateCurrentPatchReplaySnapshotPlan(options, plan, window); err != nil {
		return nativePatchReplayResult{}, detail, err
	}
	if _, err := validateCompleteSnapshotUpdates(options.Target, terminal.GeneratedSnapshotUpdates); err != nil {
		return nativePatchReplayResult{}, detail, err
	}
	bundle, err := core.CanonicalJSON(struct {
		SourceCommit string
		Updates      []snapshotUpdate
	}{SourceCommit: terminal.Preparation.SourceCommit, Updates: terminal.GeneratedSnapshotUpdates})
	if err != nil {
		return nativePatchReplayResult{}, detail, err
	}
	path := filepath.Join(runnerOutputRoot(runner), "snapshot-bundle.json")
	if err := writeRuntimeAsset(path, bundle, 0o600); err != nil {
		return nativePatchReplayResult{}, detail, err
	}
	defer removeOwnedPaths(path)
	native, stderr, err := runPatchReplayProcess(options, path, window)
	return native, stderr, err
}

func snapshotSuiteProjection(terminal RunnerExecResult, workingRoot string) projectionResult {
	outputRoot := projectionOutputRoot(workingRoot, terminal.AttemptID)
	return projectionResult{
		AttemptID: terminal.AttemptID, ProcessState: "StartedReaped", DurationMilliseconds: terminal.Observation.DurationMilliseconds,
		Materialization:  terminal.Preparation.Materialization,
		ExecutionStarted: terminal.ExecutionStarted, FailureDomain: terminal.FailureDomain,
		FailureClass: terminal.FailureClass, SourcePostimagePath: terminal.SourcePostimagePath, Observation: terminal.Observation,
		ExitCode: terminal.Observation.ExitCode, TimedOut: terminal.Observation.TimedOut, Canceled: terminal.Observation.Canceled,
		SourceUnchanged: terminal.Observation.SourceUnchanged,
		Files: map[string]projectionFile{
			"rust-suite-preparation.json": {Path: filepath.Join(outputRoot, "rust-suite-preparation.json")},
			"nextest.jsonl":               {Path: filepath.Join(outputRoot, "nextest.jsonl")},
		},
	}
}

func validateCurrentPatchReplaySnapshotPlan(options PatchReplayOptions, plan RustSuitePlan, window deadline) error {
	toolchain, err := resolveToolchain(options.WorkingRoot, options.Target, options.RuntimeRoot, window)
	if err != nil {
		return err
	}
	suite := RustSuiteOptions{Tag: options.Tag, BaselineSelectionPath: filepath.Join(options.ScratchRoot, "tasks", "rust-suite-baselines.json")}
	generation, err := readRustSuiteEstablishedGeneration(suite, toolchain)
	if err != nil || !rustSuitePlanMatchesEstablished(plan, generation.Selection) {
		return errors.Join(err, errors.New("snapshot Established selection changed"))
	}
	return nil
}

func runPatchReplayProjection(options PatchReplayOptions, cancellation <-chan os.Signal) (nativePatchReplayResult, string, error) {
	data, headErr := repositoryGitOutput(options.Target, "rev-parse", "HEAD")
	head := strings.TrimSuffix(string(data), "\n")
	if headErr != nil || !validGitCommit(head) {
		return nativePatchReplayResult{}, "", errors.Join(headErr, errors.New("schema source commit differs"))
	}
	runner := patchReplayRunnerOptions(options, cancellation)
	terminal, runErr := consumeRunnerTerminal(runner)
	if runErr == nil && terminal.Preparation.SourceCommit != head {
		if err := removeSchemaRunnerResult(runner); err != nil {
			return nativePatchReplayResult{}, "", err
		}
		runErr = os.ErrNotExist
	}
	if os.IsNotExist(runErr) {
		runner.sourceCommit = head
		terminal, runErr = executeRunner(runner)
	}
	diagnostic := runnerFailureDiagnostic(terminal)
	if terminal.FailureDomain == failureDomainHelper {
		return nativePatchReplayResult{}, diagnostic, errors.Join(runErr, errors.New("schema transform helper failed"))
	}
	payloadFailure := terminal.ExecutionStarted &&
		terminal.Terminal == "Fail" &&
		terminal.FailureDomain == failureDomainProduct &&
		terminal.FailureClass == nil &&
		terminal.Observation != nil &&
		terminal.Observation.ExitCode == 1 &&
		strings.Contains(diagnostic, "SchemaPayloadTransportFailure")
	if payloadFailure {
		return nativePatchReplayResult{}, diagnostic, errors.Join(runErr, errors.New("schema transform payload transport failed"))
	}
	if terminal.Terminal == "Pass" {
		options.schemaSourceCommit = head
		return runPatchReplayProcess(options, filepath.Join(runnerOutputRoot(runner), "runner.stdout.log"), newDeadline(core.StageDeadline(), cancellation))
	}
	if !terminal.ExecutionStarted || terminal.FailureDomain != failureDomainProduct {
		return nativePatchReplayResult{}, diagnostic, errors.New("schema transform projection failed")
	}
	output := "SchemaTransformCommandFailed"
	if terminal.Observation != nil && terminal.Observation.ExitCode == 1 && strings.Contains(diagnostic, "SchemaPostimagesDiffer") {
		output = "SchemaPostimagesDiffer"
	}
	return nativePatchReplayResult{Result: "Conflict", Kind: "Transform", Lane: options.Lane, Transform: "regenerate-app-server-schema", Output: output}, diagnostic, nil
}

func runConfigSchemaProjection(options PatchReplayOptions, head string, window deadline) (nativePatchReplayResult, string, error) {
	runner := RunnerExecOptions{
		endpointOptions:    options.endpointOptions,
		ProjectWorkingRoot: options.WorkingRoot, ProjectScratchRoot: options.ScratchRoot,
		RuntimeTemporaryRoot: options.RuntimeRoot, RecordRoot: options.RecordRoot, Source: options.Target, Release: options.Tag,
		WorkingDirectory: "codex-rs", Arguments: []string{"/usr/bin/python3", "-s", "/runner/config-schema.py", head},
		AttemptID: "patch-replay-config-schema-" + options.Tag + "-" + filepath.Base(options.Target) + "-" + head,
		Deadline:  core.StageDeadline(), Cancellation: window.signals, OnLive: options.OnLive, selectionEnd: window.end,
		consumer:     runnerConsumer{assets: []runtimeAsset{{name: "config-schema.py", data: assets.ConfigSchemaGenerator(), mode: 0o600}}, ephemeralSourcePaths: []string{"codex-rs/core/config.schema.json"}},
		resumeReaped: true,
	}
	terminal, runErr := consumeRunnerTerminal(runner)
	if os.IsNotExist(runErr) {
		terminal, runErr = executeRunner(runner)
	}
	diagnostic := runnerFailureDiagnostic(terminal)
	if terminal.Terminal == "Pass" && runErr == nil {
		native, stderr, err := runPatchReplayProcess(options, filepath.Join(runnerOutputRoot(runner), "runner.stdout.log"), window)
		if err == nil && native.Result == "ConfigSchemaGenerated" {
			err = removeSchemaRunnerResult(runner)
		}
		return native, stderr, err
	}
	if terminal.ExecutionStarted && terminal.FailureDomain == failureDomainProduct {
		kind, identity, _ := strings.Cut(options.ExpectedTerminal, ":")
		lane, name, _ := strings.Cut(identity, "/")
		conflict := nativePatchReplayResult{Result: "Conflict", Kind: kind, Lane: lane, Output: "ConfigSchemaCommandFailed"}
		if kind == "FlatPatch" {
			conflict.FlatPatch = name
		} else {
			conflict.Unit = name
		}
		return conflict, diagnostic, nil
	}
	return nativePatchReplayResult{}, diagnostic, errors.Join(runErr, errors.New("config schema projection failed"))
}

func removeSchemaRunnerResult(runner RunnerExecOptions) error {
	return removeOwnedPaths(runnerTerminalPath(runner), runnerOutputRoot(runner), filepath.Join(runner.RecordRoot, "process", runner.AttemptID))
}

func verifyReleasePublication(options ReleaseBaselineOptions) ([]PatchReplayRow, error) {
	project := exportRepositoryRoot(options.WorkingRoot)
	data, err := runNativeCustody(options.RuntimeRoot, "patch-replay-verify-export", options.worktrees.Main, project, options.Tag)
	result := nativePatchReplayResult{}
	decodeErr := json.Unmarshal(data, &result)
	if err != nil || decodeErr != nil || result.Result != "Verified" || len(result.Rows) == 0 {
		return nil, errors.Join(err, decodeErr, errors.New("release publication bytes differ"))
	}
	return result.Rows, nil
}

func exportRepositoryRoot(workingRoot string) string {
	return filepath.Join(workingRoot, "build", "patches", "private")
}

func fixedMainProjection(scratchRoot, runtimeRoot, tag, source string) (sourceProjection, error) {
	main, err := sourceProjectionAt(runtimeRoot, source)
	if err != nil || main.SourceBranch != tag+"-main-worktree" || !main.SourceClean {
		return sourceProjection{}, errors.Join(err, errors.New("release MainWorktree differs"))
	}
	roles, err := readReleaseWorktrees(source, scratchRoot, tag, newDeadline(core.StageDeadline(), nil))
	if err != nil || roles.Registered[main.SourceBranch] != source {
		return sourceProjection{}, errors.Join(err, errors.New("registered MainWorktree differs"))
	}
	return main, nil
}

func validCapabilityNonce(value string) bool {
	return len(value) == 64 && strings.Trim(value, "0123456789abcdef") == ""
}

func listed(values, value string) bool {
	return !strings.Contains(value, " ") && strings.Contains(" "+values+" ", " "+value+" ")
}
