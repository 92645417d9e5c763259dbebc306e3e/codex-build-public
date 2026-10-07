package runtime

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"codex-build/internal/assets"
	"codex-build/internal/core"
)

const rustSuiteBudgetStep = int64((5 * time.Minute) / time.Millisecond)

type RustSuiteOptions struct {
	endpointOptions
	ProjectWorkingRoot    string
	ProjectScratchRoot    string
	RuntimeTemporaryRoot  string
	RecordRoot            string
	Source                string
	Scope                 string
	Tag                   string
	BaselineSelectionPath string
	Cancellation          <-chan os.Signal
	OnLive                func()
	OnPhase               func(string)

	releaseBaseline, cargoLockHeld, baselineReanchor bool
	started, end                                     time.Time
	calibrationPlan                                  *RustSuitePlan
	baselinePreimage                                 *rustSuiteBaselineInput
	baselineProposal                                 *rustSuiteBaselineProposal
}

type RustSuiteDiagnostic struct {
	Field    string
	Expected string
	Actual   string
}

type rustSuiteOutcomes struct {
	Failures []string
	Timeouts []string
}

func (stage RustSuiteStageResult) rustSuiteOutcomes() rustSuiteOutcomes {
	return rustSuiteOutcomes{Failures: stage.Failures, Timeouts: stage.Timeouts}
}

type RustSuiteTestOutcome struct {
	Identity, Terminal  string
	ElapsedMilliseconds int64
}

type RustSuiteBinaryBinding struct {
	Package, Path string
	Environments  []string
}

type RustSuiteDiscoveredTest struct {
	Identity, Package, BinaryID, BinaryName   string
	TestName, ResultName, ExpectationIdentity string
	Ignored                                   bool
}

const rustSuiteBaselineOutcomeLimit = 64

type RustSuitePreparation struct {
	Terminal                            string
	Diagnostics                         []RustSuiteDiagnostic
	CompileOnlyPackages, HelperBinaries []string
	BinaryBindings                      []RustSuiteBinaryBinding
	SelectedPackages                    []string
	SelectedTests                       []RustSuiteDiscoveredTest
}

type rustSuiteExecution struct {
	rustSuiteOutcomes
	SelectedPackages []string
	RustSuiteExecutionSummary
}

type RustSuiteExecutionSummary struct {
	CompletedTests, SkippedTests int
	FailureCount, TimeoutCount   int
	Validated                    bool
}

type RustSuitePreparationSummary struct {
	Terminal                              string
	Diagnostics                           []RustSuiteDiagnostic
	CompileOnlyPackages, HelperBinaries   []string
	SelectedPackages                      []string
	SelectedTestCount, BinaryBindingCount int
}

type rustSuiteBaselineInput struct {
	rustSuiteBaselineProposal
	ExpectedFailures, ExpectedTimeouts map[string][]string
}

type rustSuiteBaselineProposal struct {
	Tag, ToolchainRoot, CargoVersion, ToolchainTarget    string
	CompileOnlyPackages, HelperBinaries, CoveredPackages []string
	ExecutionProfile                                     rustSuiteBaselineProfile
}

type rustSuiteBaselineGeneration struct {
	Selection rustSuiteBaselineInput
}

type rustSuiteBaselineFile struct {
	Established *rustSuiteBaselineGeneration `json:",omitempty"`
}

type rustSuiteBaselineProfile struct {
	BaselineElapsedMilliseconds *int64
	SuiteTimeoutMilliseconds    int64
	TestNetwork                 string
	NextestConcurrency          int
	TestTimeoutMilliseconds     int64
}

type RustSuitePlan struct {
	TestNetwork                         string
	BaselineReanchor                    bool `json:",omitempty"`
	SchemaVersion                       int
	RecordType, Scope, Tag              string
	SourceRole                          string
	ToolchainRoot, CargoVersion         string
	ToolchainTarget                     string
	CacheRoot                           string
	NextestConcurrency                  int
	TestTimeoutMilliseconds             int64
	CompileOnlyPackages, HelperBinaries []string
	ExcludedPackages                    []string
	ExpectedFailures, ExpectedTimeouts  []string
}

type RustSuiteStageResult struct {
	SchemaVersion                 int
	RecordType                    string
	Scope, Tag                    string
	Stage, Terminal, ProcessState string
	AttemptCleanup                string
	Diagnostics                   []RustSuiteDiagnostic
	ElapsedMilliseconds           int64
	ExitCode                      int
	TimedOut, Canceled            bool
	Failures, Timeouts            []string
	SelectedPackages              []string
	Execution                     RustSuiteExecutionSummary
	Preparation                   *RustSuitePreparationSummary `json:",omitempty"`
	SourcePostimagePath           string
	Observation                   *processTerminalObservation       `json:",omitempty"`
	MaterializationEvidence       *RustSuiteMaterializationEvidence `json:",omitempty"`
}

type RustSuiteResult struct {
	SchemaVersion                          int
	RecordType                             string
	Scope, Tag                             string
	ProductEffect, Terminal, FailureDomain string
	Diagnostics                            []RustSuiteDiagnostic
}

type rustSuitePreflight struct {
	Plan          RustSuitePlan
	Source        sourceProjection
	PreparedCargo *preparedProjectionCargo
}

type rustSuitePreflightError struct {
	Diagnostics []RustSuiteDiagnostic
}

func (err rustSuitePreflightError) Error() string {
	parts := make([]string, 0, len(err.Diagnostics))
	for _, diagnostic := range err.Diagnostics {
		parts = append(parts, diagnostic.Field+" differs")
	}
	return strings.Join(parts, "; ")
}

func RunRustSuite(options RustSuiteOptions) (RustSuiteResult, error) {
	if options.started.IsZero() {
		options.started = time.Now()
	}
	result := rustSuiteImmediateResult(options, "Blocked", failureDomainInput)
	if err := validateRustSuiteOptions(options); err != nil {
		return rustSuiteBlocked(result, err)
	}
	route := rustSuiteRoute(options)
	var cargoLock *os.File
	var err error
	if !options.cargoLockHeld {
		if err := options.phase("CargoLock"); err != nil {
			return rustSuiteHelperResult(options, err)
		}
		if options.Scope == "baseline" {
			err = writeProjectionLockRecord(options.RecordRoot, options.ProjectScratchRoot, route)
		}
		if err == nil {
			select {
			case <-options.Cancellation:
				err = errors.New("CargoLock canceled")
			default:
				cargoLock, err = tryCargoLock(options.ProjectScratchRoot)
			}
		}
		if err != nil {
			return rustSuiteBlocked(rustSuiteImmediateResult(options, "Stopped", failureDomainHelper), err)
		}
		defer releaseCargoLock(cargoLock)
	}
	options.cargoLockHeld = true
	if err := options.startDeadline(); err != nil {
		return rustSuitePreflightResult(options, err)
	}
	if err := options.deadline().check(); err != nil {
		return rustSuiteHelperResult(options, err)
	}
	preflight, err := prepareRustSuite(options)
	if err != nil {
		return rustSuitePreflightResult(options, err)
	}
	planData, err := core.CanonicalJSON(preflight.Plan)
	if err != nil {
		return rustSuiteHelperResult(options, err)
	}
	replacePlan := false
	if options.Scope == "baseline" {
		replacePlan, err = replaceRustSuiteBaseline(options, planData)
		if err != nil {
			return rustSuiteHelperResult(options, err)
		}
	}
	if !replacePlan {
		if retained, present, retainedErr := readRustSuiteResult(options, false); present || retainedErr != nil {
			return retained, retainedErr
		}
	}
	if err := options.phase("PlanPublication"); err != nil {
		return rustSuiteHelperResult(options, err)
	}
	root := rustSuiteRecordRoot(options)
	planPath := filepath.Join(root, "plan.json")
	if err = ensureDirectoryNoFollow(filepath.Dir(planPath), 0o700); err == nil {
		err = publishNoReplace(planPath, planData, 0o600)
	}
	if err != nil {
		return rustSuiteHelperResult(options, err)
	}
	stagePath := filepath.Join(root, "stage.json")
	if pending, present, pendingErr := readRustSuiteStagePending(options, preflight.Plan); present || pendingErr != nil {
		if pendingErr != nil {
			return rustSuiteHelperResult(options, pendingErr)
		}
		if err := options.announce("Cleanup"); err != nil {
			return rustSuiteHelperResult(options, err)
		}
		if err := publishRustSuiteStageAfterAttemptCleanup(stagePath, rustSuiteStageData(pending), projectionAttemptRoot(options.ProjectWorkingRoot, route), options.RuntimeTemporaryRoot, options.RecordRoot, options.ProjectScratchRoot, pending.Observation); err != nil {
			return rustSuiteHelperResult(options, err)
		}
		return finishRustSuiteStage(options, preflight, pending)
	}
	if _, statErr := os.Lstat(stagePath); statErr == nil {
		stage := RustSuiteStageResult{}
		if _, readErr := readCanonicalValue(stagePath, &stage); readErr != nil {
			return rustSuiteHelperResult(options, readErr)
		}
		return finishRustSuiteStage(options, preflight, stage)
	} else if !os.IsNotExist(statErr) {
		return rustSuiteHelperResult(options, statErr)
	}
	if err := options.phase("Cargo"); err != nil {
		return stopRustSuiteBeforeExecution(options, preflight, route, err)
	}
	prepared, preparationErr := prepareProjectionCargo(projectionRequest{ProjectWorkingRoot: options.ProjectWorkingRoot, ProjectScratchRoot: options.ProjectScratchRoot, Source: options.Source, CargoStage: rustSuiteCargoStage(options.Scope), LockHeld: true, Deadline: options.remaining(), Cancellation: options.Cancellation, window: options.deadline()}, sourceToolchainProjection{
		sourceProjection: preflight.Source,
		Toolchain:        ToolchainProjection{Root: preflight.Plan.ToolchainRoot, CargoVersion: preflight.Plan.CargoVersion, Target: preflight.Plan.ToolchainTarget},
	})
	if preparationErr != nil {
		return stopRustSuiteBeforeExecution(options, preflight, route, preparationErr)
	}
	if prepared == nil {
		return stopRustSuiteBeforeExecution(options, preflight, route, errors.New("Cargo preparation is absent"))
	}
	preflight.PreparedCargo = prepared
	return executeRustSuiteStage(options, preflight, route, projectionAttemptRoot(options.ProjectWorkingRoot, route))
}

func rustSuiteRecordRoot(options RustSuiteOptions) string {
	return filepath.Join(options.RecordRoot, "rust-suite", options.Scope)
}

func rustSuiteRoute(options RustSuiteOptions) string {
	return options.Scope
}

func rustSuiteOutputRoot(options RustSuiteOptions) string {
	return projectionOutputRoot(options.ProjectWorkingRoot, rustSuiteRoute(options))
}

func (options *RustSuiteOptions) startDeadline() error {
	if !options.end.IsZero() {
		return nil
	}
	if options.started.IsZero() {
		options.started = time.Now()
	}
	limit := core.StageDeadline()
	if options.Scope != "baseline" {
		value, _, err := readRustSuiteBaselineFile(options.BaselineSelectionPath, options.Tag)
		if err != nil || value.Established == nil || value.Established.Selection.Tag != options.Tag || !validRustSuiteTiming(value.Established.Selection.ExecutionProfile) {
			return errors.Join(err, errors.New("established suite timing differs"))
		}
		limit = time.Duration(value.Established.Selection.ExecutionProfile.SuiteTimeoutMilliseconds) * time.Millisecond
	}
	options.end = options.started.Add(limit)
	return nil
}

func (options RustSuiteOptions) remaining() time.Duration {
	if options.end.IsZero() {
		return core.StageDeadline()
	}
	return time.Until(options.end)
}

func (options RustSuiteOptions) deadline() deadline {
	if options.end.IsZero() {
		return newDeadline(core.StageDeadline(), options.Cancellation)
	}
	return deadline{options.end, options.Cancellation}
}

func (options RustSuiteOptions) phase(name string) error {
	return errors.Join(options.announce(name), options.deadline().check())
}

func (options RustSuiteOptions) announce(name string) error {
	if options.cargoLockHeld && options.Scope == "baseline" {
		phase, _, _ := strings.Cut(name, " ")
		if err := writeProjectionLockRecord(options.RecordRoot, options.ProjectScratchRoot, rustSuiteRoute(options), phase); err != nil {
			return err
		}
	}
	if options.OnPhase != nil {
		options.OnPhase(name)
	}
	return nil
}

func replaceRustSuiteBaseline(options RustSuiteOptions, next []byte) (bool, error) {
	root := rustSuiteRecordRoot(options)
	planPath := filepath.Join(root, "plan.json")
	current := RustSuitePlan{}
	data, err := readCanonicalValue(planPath, &current)
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	pending := planPath + ".pending"
	intent, _, replacing, err := readOptionalRegular(pending)
	if err != nil || replacing && !bytes.Equal(intent, next) {
		return false, errors.Join(err, errors.New("baseline replacement input differs"))
	}
	if !replacing && bytes.Equal(data, next) {
		var result RustSuiteResult
		if _, err := readCanonicalValue(filepath.Join(root, "result.json"), &result); os.IsNotExist(err) {
			return false, nil
		} else if err != nil {
			return false, err
		} else if result.Terminal == "Pass" {
			return false, nil
		}
	}
	if !replacing {
		if err := writeTaskPending(pending, next, 0o600); err != nil {
			return false, err
		}
	}
	runner := RunnerExecOptions{ProjectWorkingRoot: options.ProjectWorkingRoot, ProjectScratchRoot: options.ProjectScratchRoot, RuntimeTemporaryRoot: options.RuntimeTemporaryRoot, RecordRoot: options.RecordRoot, AttemptID: "baseline"}
	if err := replaceRunnerEvidence(runner); err != nil {
		return false, err
	}
	for _, name := range []string{"result.json", "stage.json"} {
		path := filepath.Join(root, name)
		if err := removeOwnedPaths(path, path+".stage", path+".pending"); err != nil {
			return false, err
		}
	}
	if err := os.Rename(pending, planPath); err != nil {
		return false, err
	}
	return true, syncDirectory(root)
}

func readRustSuiteResult(options RustSuiteOptions, alreadyEstablished bool) (RustSuiteResult, bool, error) {
	root := rustSuiteRecordRoot(options)
	path := filepath.Join(root, "result.json")
	result, plan, stage := RustSuiteResult{}, RustSuitePlan{}, RustSuiteStageResult{}
	_, planErr := readCanonicalValue(filepath.Join(root, "plan.json"), &plan)
	_, stageErr := readCanonicalValue(filepath.Join(root, "stage.json"), &stage)
	_, resultErr := readCanonicalValue(path, &result)
	missingResult := os.IsNotExist(resultErr)
	if missingResult && os.IsNotExist(stageErr) {
		return RustSuiteResult{}, false, nil
	}
	if missingResult {
		resultErr = nil
	}
	source, sourceErr := suiteSource(options)
	if sourceErr == nil {
		sourceErr = validateRustSuiteSourceRole(options, source)
	}
	toolchain, toolchainErr := resolveToolchain(options.ProjectWorkingRoot, options.deadline())
	if options.Scope != "baseline" {
		generation, baselineErr := readRustSuiteEstablishedGeneration(options, toolchain)
		baselineErr = errors.Join(toolchainErr, baselineErr)
		planErr = errors.Join(planErr, baselineErr)
		if baselineErr == nil && !rustSuitePlanMatchesEstablished(plan, generation.Selection) {
			planErr = errors.Join(planErr, errors.New("RustSuite retained baseline selection differs"))
		}
	}
	valid := resultErr == nil && planErr == nil && stageErr == nil && sourceErr == nil && toolchainErr == nil && len(rustSuitePlanDiagnostics(plan, options.Scope, options.Tag, source, toolchain, options.ProjectWorkingRoot)) == 0 && validateRustSuiteStage(options.RecordRoot, options.ProjectScratchRoot, options.RuntimeTemporaryRoot, options.Source, plan, stage, false) == nil
	if valid && !missingResult {
		valid = rustSuiteSameValue(rustSuiteResultForStage(plan, stage), result)
	}
	if !valid {
		result, retainedErr := rustSuiteHelperResult(options, errors.Join(resultErr, planErr, stageErr, sourceErr, toolchainErr, errors.New("RustSuite retained terminal differs")))
		return result, true, retainedErr
	}
	if missingResult {
		result, resultErr = finishRustSuiteStage(options, rustSuitePreflight{Plan: plan, Source: source}, stage)
		return result, true, resultErr
	}
	if options.Scope == "baseline" && result.Terminal == "Pass" && !alreadyEstablished {
		publication, promotionErr := prepareRustSuiteBaselinePromotion(options, plan, stage)
		if promotionErr == nil && publication != nil {
			promotionErr = publication()
		}
		if promotionErr != nil {
			failed, _ := rustSuiteHelperResult(options, promotionErr)
			return failed, true, promotionErr
		}
	}
	return result, true, consumeRustSuiteOutput(options, result)
}

func rustSuitePlanMatchesEstablished(plan RustSuitePlan, selection rustSuiteBaselineInput) bool {
	packages := sortedRustSuitePackageUnion(plan.ExcludedPackages, selection.CoveredPackages)
	return rustSuitePlanMatchesPolicy(plan, selection.rustSuiteBaselineProposal, packages) && sameStrings(plan.ExpectedFailures, flattenRustSuiteExpectations(selection.ExpectedFailures)) && sameStrings(plan.ExpectedTimeouts, flattenRustSuiteExpectations(selection.ExpectedTimeouts))
}

func rustSuiteImmediateResult(options RustSuiteOptions, terminal, domain string) RustSuiteResult {
	return RustSuiteResult{
		SchemaVersion: core.SchemaVersion, RecordType: "RustSuiteResultRecord", Scope: options.Scope, Tag: options.Tag,
		ProductEffect: "None", Terminal: terminal, FailureDomain: domain,
		Diagnostics: []RustSuiteDiagnostic{},
	}
}

func rustSuiteBlocked(result RustSuiteResult, err error) (RustSuiteResult, error) {
	result.Diagnostics = rustSuiteDiagnostics(err)
	return result, err
}

func rustSuitePreflightResult(options RustSuiteOptions, err error) (RustSuiteResult, error) {
	if errors.Is(err, errNativeTimeout) || errors.Is(err, errOperationCanceled) {
		return rustSuiteHelperResult(options, err)
	}
	return rustSuiteBlocked(rustSuiteImmediateResult(options, "Blocked", rustSuiteFailureDomain(err)), err)
}

func rustSuiteHelperResult(options RustSuiteOptions, err error) (RustSuiteResult, error) {
	terminal := "Fail"
	if errors.Is(err, errNativeTimeout) || errors.Is(err, errOperationCanceled) {
		terminal = "Stopped"
	}
	result := rustSuiteImmediateResult(options, terminal, failureDomainHelper)
	result.Diagnostics = rustSuiteDiagnostics(err)
	return result, err
}

func executeRustSuiteStage(options RustSuiteOptions, preflight rustSuitePreflight, route, attemptRoot string) (RustSuiteResult, error) {
	plan := preflight.Plan
	remaining := options.remaining()
	if remaining <= 0 {
		return stopRustSuiteBeforeExecution(options, preflight, route, errors.Join(errNativeTimeout, errors.New("Runner phase exceeded total deadline")))
	}
	if err := options.phase("Runner"); err != nil {
		return stopRustSuiteBeforeExecution(options, preflight, route, err)
	}
	arguments := rustSuiteCommand(plan)
	var publishedStage *RustSuiteStageResult
	var terminalPublicationErr error
	request := projectionRequest{
		SkillRoot: options.SkillRoot, ProjectWorkingRoot: options.ProjectWorkingRoot, ProjectScratchRoot: options.ProjectScratchRoot, RuntimeTemporaryRoot: options.RuntimeTemporaryRoot,
		RecordRoot: options.RecordRoot, Source: options.Source, Release: options.Tag, WorkingDirectory: "codex-rs", Arguments: arguments, OutputRoot: rustSuiteOutputRoot(options), ResultFiles: []string{"rust-suite-preparation.json", "nextest.jsonl"},
		CargoStage: rustSuiteCargoStage(plan.Scope), Consumer: rustSuiteConsumer(plan), LockHeld: true,
		Deadline: remaining, Cancellation: options.Cancellation, OnLive: options.OnLive, OnPhase: options.phase, window: options.deadline(),
		AttemptIdentity: route, AttemptRoot: attemptRoot, PreparedCargo: preflight.PreparedCargo, RetainAttempt: true, ResumeReaped: true,
	}
	request.OnLive = func() {
		if options.calibrationPlan != nil {
			*options.calibrationPlan = plan
		}
		if options.OnLive != nil {
			options.OnLive()
		}
	}
	request.PublishTerminal = func(projection projectionResult, runErr error) error {
		if options.remaining() <= 0 && !projection.TimedOut && !projection.Canceled {
			setProjectionTimeout(&projection)
			runErr = errors.Join(runErr, errNativeTimeout)
		}
		stage := RustSuiteStageResult{}
		if projection.ProcessState == "IncompleteBeforeDurableRunnerEvidence" {
			stage = rustSuiteUnobservedStage(preflight.Plan, projection.ProcessState, "IncompleteAttemptCleanupComplete", rustSuiteCompletionDiagnostics(nil, nil, runErr, projection))
			appendRustSuiteOutputDiagnostics(&stage.Diagnostics, request.OutputRoot)
		} else {
			stage = rustSuiteReapedStage(options, preflight, projection, runErr)
		}
		data, err := core.CanonicalJSON(stage)
		if err == nil {
			err = options.announce("Cleanup")
		}
		if err == nil {
			err = publishRustSuiteStageAfterAttemptCleanup(filepath.Join(rustSuiteRecordRoot(options), "stage.json"), data, attemptRoot, options.RuntimeTemporaryRoot, options.RecordRoot, options.ProjectScratchRoot, stage.Observation)
		}
		if err == nil {
			publishedStage = &stage
		}
		terminalPublicationErr = err
		return err
	}
	projection, runErr := runProjection(request)
	if projection.Canceled && !projection.TimedOut && !projection.AttemptCleaned {
		return rustSuiteHelperResult(options, errors.Join(runErr, errOperationCanceled))
	}
	if projection.AttemptCleaned && errors.Is(runErr, errBuildTemporaryHeadroom) {
		return rustSuiteHelperResult(options, runErr)
	}
	if publishedStage != nil {
		return finishRustSuiteStage(options, preflight, *publishedStage)
	}
	if terminalPublicationErr != nil {
		return rustSuiteHelperResult(options, errors.Join(runErr, terminalPublicationErr))
	}
	return finishRustSuiteProjection(options, preflight, route, attemptRoot, projection, runErr)
}

func finishRustSuiteProjection(options RustSuiteOptions, preflight rustSuitePreflight, route, attemptRoot string, projection projectionResult, runErr error) (RustSuiteResult, error) {
	if projection.ProcessState == "OwnershipLostBeforeDurableRunnerEvidence" {
		return closeOwnershipLostRustSuite(options, preflight, route, attemptRoot)
	}
	if projection.ProcessState == "IncompleteBeforeDurableRunnerEvidence" {
		if runErr == nil {
			runErr = errors.New("process Terminal publication did not complete")
		}
		return closeIncompleteRustSuite(options, preflight, route, attemptRoot, projection, runErr)
	}
	if !projection.ExecutionStarted {
		if runErr == nil {
			runErr = errors.New("RustSuite materialization did not reach process ownership")
		}
		return closeIncompleteRustSuite(options, preflight, route, attemptRoot, projection, runErr)
	}
	if !projection.Reaped {
		actual := "runner reap remains unconfirmed"
		if runErr != nil {
			actual = runErr.Error()
		}
		return rustSuiteBlocked(rustSuiteImmediateResult(options, "Blocked", failureDomainHelper), errors.New(actual))
	}
	return rustSuiteHelperResult(options, errors.Join(runErr, errors.New("RustSuite Stage publication did not complete")))
}

func rustSuiteReapedStage(options RustSuiteOptions, preflight rustSuitePreflight, projection projectionResult, runErr error) RustSuiteStageResult {
	plan := preflight.Plan
	if projection.ProcessState == "OwnershipLostBeforeDurableRunnerEvidence" {
		return rustSuiteUnobservedStage(plan, projection.ProcessState, "OwnershipLostCleanupComplete", []RustSuiteDiagnostic{})
	}
	preparation, preparationErr := readRustSuitePreparation(projection, plan)
	preparationComplete := preparationErr == nil && preparation.Terminal == "Complete"
	if preparationComplete && !projection.TimedOut && !projection.Canceled && projection.ExitCode != 0 && projection.ExitCode != 100 {
		projection.FailureDomain = failureDomainHelper
		runErr = fmt.Errorf("Nextest execution failed with exit %d", projection.ExitCode)
	}
	execution, outcomeErr := rustSuiteExecution{}, error(nil)
	if preparationErr == nil {
		window := options.deadline()
		if projection.TimedOut || projection.Canceled {
			window = newDeadline(core.GracePeriod(), nil)
		}
		execution, outcomeErr = readRustSuiteExecution(window, projection, preparation, preparationComplete)
	}
	diagnostics := rustSuiteCompletionDiagnostics(preparationErr, outcomeErr, runErr, projection)
	completeExecution := preparationComplete && execution.Validated && execution.CompletedTests == len(preparation.SelectedTests)
	terminal := rustSuiteStageTerminal(plan, projection, execution.rustSuiteOutcomes)
	if terminal == "Pass" && !completeExecution {
		terminal = "Fail"
	}
	if preparationErr == nil && preparation.Terminal == "Fail" && len(preparation.Diagnostics) == 1 && strings.HasPrefix(preparation.Diagnostics[0].Field, "binary-") {
		terminal = "Stopped"
		diagnostics = append(diagnostics, preparation.Diagnostics...)
		sort.Slice(diagnostics, func(left, right int) bool { return diagnostics[left].Field < diagnostics[right].Field })
	}
	if projection.FailureDomain == failureDomainHelper {
		terminal = "Stopped"
	}
	if outcomeErr != nil {
		terminal = "Stopped"
	}
	if terminal != "Pass" {
		appendRustSuiteOutputDiagnostics(&diagnostics, rustSuiteOutputRoot(options))
	}
	var preparationRecord *RustSuitePreparationSummary
	if preparationErr == nil {
		preparationRecord = &RustSuitePreparationSummary{
			Terminal: preparation.Terminal, Diagnostics: preparation.Diagnostics,
			CompileOnlyPackages: preparation.CompileOnlyPackages, HelperBinaries: preparation.HelperBinaries,
			SelectedPackages: preparation.SelectedPackages, SelectedTestCount: len(preparation.SelectedTests), BinaryBindingCount: len(preparation.BinaryBindings),
		}
	}
	failures := execution.Failures[:min(len(execution.Failures), rustSuiteBaselineOutcomeLimit)]
	timeouts := execution.Timeouts[:min(len(execution.Timeouts), rustSuiteBaselineOutcomeLimit-len(failures))]
	return RustSuiteStageResult{
		SchemaVersion: core.SchemaVersion, RecordType: "RustSuiteStageResultRecord",
		Scope: plan.Scope, Tag: plan.Tag,
		Stage: projection.AttemptID, Terminal: terminal, Diagnostics: diagnostics,
		ProcessState: projection.ProcessState, AttemptCleanup: "ExactAttemptRootAbsent", ElapsedMilliseconds: projection.DurationMilliseconds,
		ExitCode: projection.ExitCode, TimedOut: projection.TimedOut, Canceled: projection.Canceled, Failures: failures, Timeouts: timeouts,
		SelectedPackages: execution.SelectedPackages, Execution: execution.RustSuiteExecutionSummary, Preparation: preparationRecord, SourcePostimagePath: projection.SourcePostimagePath,
		Observation: projection.Observation, MaterializationEvidence: &projection.Materialization,
	}
}

func rustSuiteStageData(stage RustSuiteStageResult) []byte {
	data, _ := core.CanonicalJSON(stage)
	return data
}

func readRustSuiteStagePending(options RustSuiteOptions, plan RustSuitePlan) (RustSuiteStageResult, bool, error) {
	var stage RustSuiteStageResult
	_, err := readCanonicalValue(filepath.Join(rustSuiteRecordRoot(options), "stage.json.pending"), &stage)
	if os.IsNotExist(err) {
		return stage, false, nil
	}
	if err == nil {
		err = validateRustSuiteStage(options.RecordRoot, options.ProjectScratchRoot, options.RuntimeTemporaryRoot, options.Source, plan, stage, true)
	}
	return stage, true, err
}

func rustSuiteCompletionDiagnostics(preparationErr, outcomeErr, runErr error, projection projectionResult) []RustSuiteDiagnostic {
	diagnostics := []RustSuiteDiagnostic{}
	for _, name := range projection.NativeAbsentOutputs {
		diagnostics = append(diagnostics, RustSuiteDiagnostic{Field: "native-absent-" + name, Expected: "present declared output", Actual: "absent"})
	}
	for _, name := range projection.FailedOutputs {
		diagnostics = append(diagnostics, RustSuiteDiagnostic{Field: "publication-failed-" + name, Expected: "published fixed output", Actual: "failed"})
	}
	for _, failure := range []struct {
		field, expected string
		err             error
	}{
		{"outcomes", "complete selected execution JSONL", outcomeErr},
		{"preparation", "canonical RustSuite preparation", preparationErr},
	} {
		if failure.err != nil {
			diagnostics = append(diagnostics, RustSuiteDiagnostic{Field: failure.field, Expected: failure.expected, Actual: failure.err.Error()})
		}
	}
	if projection.FailureDomain == failureDomainHelper {
		field, want, got := "runner-completion", "completion", "helper"
		if projection.FailureClass != nil && *projection.FailureClass == "OutputPublication" {
			field, want = "required-output", "outputs"
		}
		if runErr != nil {
			got = runErr.Error()
		} else if projection.FailureClass != nil {
			got = *projection.FailureClass
		}
		diagnostics = append(diagnostics, RustSuiteDiagnostic{Field: field, Expected: want, Actual: got})
	}
	sort.Slice(diagnostics, func(left, right int) bool { return diagnostics[left].Field < diagnostics[right].Field })
	return diagnostics
}

func closeIncompleteRustSuite(options RustSuiteOptions, preflight rustSuitePreflight, route, attemptRoot string, projection projectionResult, cause error) (RustSuiteResult, error) {
	diagnostics := rustSuiteCompletionDiagnostics(nil, nil, cause, projection)
	return closeRustSuiteWithoutEvidence(options, preflight, route, attemptRoot, "IncompleteBeforeDurableRunnerEvidence", "IncompleteAttemptCleanupComplete", diagnostics)
}

func stopRustSuiteBeforeExecution(options RustSuiteOptions, preflight rustSuitePreflight, route string, cause error) (RustSuiteResult, error) {
	return closeIncompleteRustSuite(options, preflight, route, projectionAttemptRoot(options.ProjectWorkingRoot, route), projectionResult{FailureDomain: failureDomainHelper}, cause)
}

func closeOwnershipLostRustSuite(options RustSuiteOptions, preflight rustSuitePreflight, route, attemptRoot string) (RustSuiteResult, error) {
	return closeRustSuiteWithoutEvidence(options, preflight, route, attemptRoot, "OwnershipLostBeforeDurableRunnerEvidence", "OwnershipLostCleanupComplete", []RustSuiteDiagnostic{})
}

func closeRustSuiteWithoutEvidence(options RustSuiteOptions, preflight rustSuitePreflight, route, attemptRoot, processState, attemptCleanup string, diagnostics []RustSuiteDiagnostic) (RustSuiteResult, error) {
	if err := options.announce("Cleanup"); err != nil {
		return rustSuiteHelperResult(options, err)
	}
	if err := removeProjectionScratch(attemptRoot, options.RuntimeTemporaryRoot, newDeadline(core.GracePeriod(), nil)); err != nil {
		return rustSuiteHelperResult(options, err)
	}
	if err := projectionAttemptAbsent(attemptRoot, options.RuntimeTemporaryRoot); err != nil {
		return rustSuiteHelperResult(options, err)
	}
	stage := rustSuiteUnobservedStage(preflight.Plan, processState, attemptCleanup, diagnostics)
	appendRustSuiteOutputDiagnostics(&stage.Diagnostics, rustSuiteOutputRoot(options))
	stageData, err := core.CanonicalJSON(stage)
	stagePath := filepath.Join(rustSuiteRecordRoot(options), "stage.json")
	if err == nil {
		err = publishNoReplace(stagePath, stageData, 0o600)
	}
	if err != nil {
		return rustSuiteHelperResult(options, err)
	}
	return finishRustSuiteStage(options, preflight, stage)
}

func rustSuiteUnobservedStage(plan RustSuitePlan, processState, attemptCleanup string, diagnostics []RustSuiteDiagnostic) RustSuiteStageResult {
	return RustSuiteStageResult{
		SchemaVersion: core.SchemaVersion, RecordType: "RustSuiteStageResultRecord", Scope: plan.Scope, Tag: plan.Tag,
		Stage: rustSuiteRoute(RustSuiteOptions{Tag: plan.Tag, Scope: plan.Scope}), Terminal: "Stopped", ProcessState: processState, AttemptCleanup: attemptCleanup, Diagnostics: diagnostics, ExitCode: -1,
		Failures: []string{}, Timeouts: []string{}, SelectedPackages: []string{},
	}
}

func appendRustSuiteOutputDiagnostics(diagnostics *[]RustSuiteDiagnostic, root string) {
	for _, name := range []string{"nextest.jsonl", "runner.stderr.log", "runner.stdout.log", "rust-suite-preparation.json", "sccache-stats.json"} {
		path := filepath.Join(root, name)
		info, err := os.Lstat(path)
		if err == nil && info.Mode().IsRegular() && !rustSuiteHasDiagnostic(*diagnostics, "output-"+name) {
			*diagnostics = append(*diagnostics, RustSuiteDiagnostic{Field: "output-" + name, Expected: "retained fixed path", Actual: path})
		}
	}
	sort.Slice(*diagnostics, func(left, right int) bool { return (*diagnostics)[left].Field < (*diagnostics)[right].Field })
}

func finishRustSuiteStage(options RustSuiteOptions, preflight rustSuitePreflight, stage RustSuiteStageResult) (RustSuiteResult, error) {
	finishErr := options.phase("Stage")
	if err := validateRustSuiteStage(options.RecordRoot, options.ProjectScratchRoot, options.RuntimeTemporaryRoot, options.Source, preflight.Plan, stage, false); err != nil {
		return rustSuiteHelperResult(options, err)
	}
	result := rustSuiteResultForStage(preflight.Plan, stage)
	var publication func() error
	if finishErr == nil && result.Terminal == "Pass" && options.Scope == "baseline" {
		publication, finishErr = prepareRustSuiteBaselinePromotion(options, preflight.Plan, stage)
	}
	if finishErr == nil {
		finishErr = options.phase("ResultPublication")
	}
	if finishErr != nil && result.Terminal != "Stopped" {
		stage, finishErr = stopRustSuiteStage(options, stage, finishErr)
		result = rustSuiteResultForStage(preflight.Plan, stage)
	}
	data, err := core.CanonicalJSON(result)
	path := filepath.Join(rustSuiteRecordRoot(options), "result.json")
	if err == nil {
		err = ensureDirectoryNoFollow(filepath.Dir(path), 0o700)
	}
	if err == nil {
		err = publishNoReplace(path, data, 0o600)
	}
	if err != nil {
		return rustSuiteHelperResult(options, err)
	}
	if publication != nil {
		if err := publication(); err != nil {
			failed, _ := rustSuiteHelperResult(options, err)
			return failed, err
		}
	}
	return result, errors.Join(finishErr, consumeRustSuiteOutput(options, result))
}

func consumeRustSuiteOutput(options RustSuiteOptions, result RustSuiteResult) error {
	return options.announce("Terminal")
}

func stopRustSuiteStage(options RustSuiteOptions, stage RustSuiteStageResult, cause error) (RustSuiteStageResult, error) {
	stage.Terminal, stage.Diagnostics = "Stopped", rustSuiteDiagnostics(cause)
	data, err := core.CanonicalJSON(stage)
	if err == nil {
		err = publishReplace(filepath.Join(rustSuiteRecordRoot(options), "stage.json"), data, 0o600)
	}
	return stage, errors.Join(cause, err)
}

func prepareRustSuiteBaselinePromotion(options RustSuiteOptions, plan RustSuitePlan, stage RustSuiteStageResult) (func() error, error) {
	if err := options.phase("BaselinePromotion"); err != nil {
		return nil, err
	}
	value, preimage, err := readRustSuiteBaselineFile(options.BaselineSelectionPath, options.Tag)
	if err != nil {
		return nil, err
	}
	toolchain, toolchainErr := resolveToolchain(options.ProjectWorkingRoot, options.deadline())
	packages, packageErr := discoverRustSuitePackages(options, toolchain)
	if options.baselinePreimage == nil || options.baselineProposal == nil {
		return nil, errors.New("baseline promotion input differs")
	}
	established, proposal := *options.baselinePreimage, *options.baselineProposal
	execution, aligned, executionErr := rustSuiteBaselineExecution(established, proposal, packages, options.baselineReanchor)
	if toolchainErr != nil || packageErr != nil || executionErr != nil || plan.BaselineReanchor != options.baselineReanchor || !validRustSuiteBaselineSelection(established, established.Tag, toolchain) || !validRustSuiteBaselineProposal(proposal, options.Tag, toolchain) || !rustSuitePlanMatchesProposal(plan, execution, packages) {
		return nil, errors.Join(toolchainErr, packageErr, executionErr, errors.New("baseline promotion binding differs"))
	}
	priorFailures, priorTimeouts := map[string][]string{}, map[string][]string{}
	executed := append(append(append([]string{}, plan.CompileOnlyPackages...), plan.HelperBinaries...), stage.SelectedPackages...)
	if established.Tag == proposal.Tag {
		priorFailures = established.ExpectedFailures
		priorTimeouts = established.ExpectedTimeouts
	}
	excluded, exclusionErr := rustSuiteTransientExclusions(packages, aligned)
	if exclusionErr != nil {
		return nil, exclusionErr
	}
	selection := rustSuiteBaselineInput{
		rustSuiteBaselineProposal: aligned,
		ExpectedFailures:          updateBaselineExpectations(priorFailures, packages, excluded, executed, stage.Failures),
		ExpectedTimeouts:          updateBaselineExpectations(priorTimeouts, packages, excluded, executed, stage.Timeouts),
	}
	expected := rustSuiteBaselineGeneration{Selection: selection}
	if value.Established != nil && rustSuiteSameValue(*value.Established, expected) {
		return nil, nil
	}
	if established.Tag == proposal.Tag {
		if value.Established == nil || !rustSuiteSameValue(value.Established.Selection, established) {
			return nil, errors.New("baseline promotion preimage differs")
		}
	} else if value.Established != nil {
		return nil, errors.New("initial baseline promotion preimage differs")
	}
	return func() error {
		return publishRustSuiteBaselineFile(options.BaselineSelectionPath, options.Tag, established.Tag, preimage, rustSuiteBaselineFile{Established: &expected})
	}, nil
}

func updateBaselineExpectations(current map[string][]string, packages, excluded, executed, observed []string) map[string][]string {
	members, replaced, result := rustSuitePackageSet(packages), rustSuitePackageSet(excluded, executed), map[string][]string{}
	for name, tests := range current {
		if members[name] && !replaced[name] {
			result[name] = append([]string{}, tests...)
		}
	}
	for name, tests := range rustSuiteExpectationMap(observed) {
		result[name] = tests
	}
	return result
}

func rustSuitePackageSet(groups ...[]string) map[string]bool {
	result := map[string]bool{}
	for _, group := range groups {
		for _, name := range group {
			result[name] = true
		}
	}
	return result
}

func sortedRustSuitePackageUnion(groups ...[]string) []string {
	result := []string{}
	for name := range rustSuitePackageSet(groups...) {
		result = append(result, name)
	}
	sort.Strings(result)
	return result
}

func rustSuiteTransientExclusions(packages []string, selection rustSuiteBaselineProposal) ([]string, error) {
	members, covered := rustSuitePackageSet(packages), rustSuitePackageSet(selection.CoveredPackages)
	roles := rustSuitePackageSet(selection.CompileOnlyPackages, selection.HelperBinaries)
	for name := range covered {
		if !members[name] || roles[name] {
			return nil, errors.New("covered package differs from workspace or role")
		}
	}
	for name := range roles {
		if !members[name] {
			return nil, errors.New("role package is absent from workspace")
		}
	}
	excluded := []string{}
	for _, name := range packages {
		if !covered[name] || roles[name] {
			excluded = append(excluded, name)
		}
	}
	return excluded, nil
}

func rustSuitePlanMatchesProposal(plan RustSuitePlan, selection rustSuiteBaselineProposal, packages []string) bool {
	return rustSuitePlanMatchesPolicy(plan, selection, packages) && len(plan.ExpectedFailures) == 0 && len(plan.ExpectedTimeouts) == 0
}

func rustSuitePlanMatchesPolicy(plan RustSuitePlan, selection rustSuiteBaselineProposal, packages []string) bool {
	excluded, err := rustSuiteTransientExclusions(packages, selection)
	return err == nil && plan.TestNetwork == selection.ExecutionProfile.TestNetwork && plan.Tag == selection.Tag && plan.ToolchainRoot == selection.ToolchainRoot && plan.CargoVersion == selection.CargoVersion && plan.ToolchainTarget == selection.ToolchainTarget && sameStrings(plan.CompileOnlyPackages, selection.CompileOnlyPackages) && sameStrings(plan.HelperBinaries, selection.HelperBinaries) && sameStrings(plan.ExcludedPackages, excluded) && plan.NextestConcurrency == selection.ExecutionProfile.NextestConcurrency && plan.TestTimeoutMilliseconds == selection.ExecutionProfile.TestTimeoutMilliseconds
}

func rustSuiteResultForStage(plan RustSuitePlan, stage RustSuiteStageResult) RustSuiteResult {
	result := RustSuiteResult{
		SchemaVersion: core.SchemaVersion, RecordType: "RustSuiteResultRecord",
		Scope: plan.Scope, Tag: plan.Tag, ProductEffect: "None", Terminal: "Pass", FailureDomain: failureDomainNone,
		Diagnostics: []RustSuiteDiagnostic{},
	}
	switch stage.ProcessState {
	case "OwnershipLostBeforeDurableRunnerEvidence":
		result.Terminal, result.FailureDomain = "Stopped", failureDomainHelper
		result.Diagnostics = []RustSuiteDiagnostic{{Field: "process-state", Expected: "durable runner evidence", Actual: stage.ProcessState}}
	case "IncompleteBeforeDurableRunnerEvidence":
		result.Terminal, result.FailureDomain = "Stopped", failureDomainHelper
		result.Diagnostics = append([]RustSuiteDiagnostic{}, stage.Diagnostics...)
	case "CanceledReaped":
		result.Terminal, result.FailureDomain = "Stopped", failureDomainProduct
		if stage.Observation != nil && !stage.Observation.ExecutionStarted {
			result.FailureDomain = failureDomainHelper
		}
		result.Diagnostics = []RustSuiteDiagnostic{{Field: "cancellation", Expected: "not canceled", Actual: "canceled"}}
		if stage.SourcePostimagePath != "" {
			result.Diagnostics = append(result.Diagnostics, RustSuiteDiagnostic{Field: "source-postimage-path", Expected: "declared target or admitted ephemeral path", Actual: stage.SourcePostimagePath})
		}
	case "StartedReaped":
		if stage.Terminal == "Stopped" {
			result.Terminal, result.FailureDomain = "Stopped", failureDomainHelper
			result.Diagnostics = append([]RustSuiteDiagnostic{}, stage.Diagnostics...)
		} else if stage.Terminal != "Pass" {
			result.Terminal, result.FailureDomain = "Fail", failureDomainProduct
			result.Diagnostics = rustSuiteStageDiagnostics(plan, stage)
		}
	}
	if plan.Scope != "baseline" && stage.Execution.Validated && stage.Terminal != "Stopped" {
		result.Diagnostics = append(result.Diagnostics, rustSuiteOutcomeDiagnostics(plan, stage)...)
		sort.Slice(result.Diagnostics, func(left, right int) bool { return result.Diagnostics[left].Field < result.Diagnostics[right].Field })
	}
	return result
}

func rustSuiteStageDiagnostics(plan RustSuitePlan, stage RustSuiteStageResult) []RustSuiteDiagnostic {
	diagnostics := []RustSuiteDiagnostic{{Field: "stage-terminal", Expected: "Pass", Actual: stage.Terminal}}
	if stage.SourcePostimagePath != "" {
		diagnostics = append(diagnostics, RustSuiteDiagnostic{Field: "source-postimage-path", Expected: "declared target or admitted ephemeral path", Actual: stage.SourcePostimagePath})
	}
	if stage.Preparation != nil && stage.Preparation.Terminal != "Complete" {
		diagnostics = append(diagnostics, stage.Preparation.Diagnostics...)
	}
	if stage.Preparation != nil && stage.Execution.CompletedTests != stage.Preparation.SelectedTestCount {
		diagnostics = append(diagnostics, RustSuiteDiagnostic{Field: "selected-test-completion", Expected: fmt.Sprintf("%d completed selected tests", stage.Preparation.SelectedTestCount), Actual: fmt.Sprintf("%d completed selected tests", stage.Execution.CompletedTests)})
	}

	return diagnostics
}

func rustSuiteOutcomeDiagnostics(plan RustSuitePlan, stage RustSuiteStageResult) []RustSuiteDiagnostic {
	diagnostics := []RustSuiteDiagnostic{}
	for _, group := range []struct {
		name              string
		current, expected []string
		count             int
	}{{"failures", stage.Failures, plan.ExpectedFailures, stage.Execution.FailureCount}, {"timeouts", stage.Timeouts, plan.ExpectedTimeouts, stage.Execution.TimeoutCount}} {
		for _, delta := range []struct {
			name        string
			left, right []string
		}{{"novel-", group.current, group.expected}, {"unobserved-established-", group.expected, group.current}} {
			values := []string{}
			for _, identity := range delta.left {
				if !sortedHas(delta.right, identity) {
					values = append(values, identity)
				}
			}
			actual := fmt.Sprintf("count=%d observed=%d identities=%s", len(values), group.count, strings.Join(values[:min(len(values), 8)], ","))
			if group.count != len(group.current) {
				actual = "sample-only " + actual
			}
			diagnostics = append(diagnostics, RustSuiteDiagnostic{Field: delta.name + group.name, Expected: "no novel outcome; unobserved does not attest a pass", Actual: boundedRunnerDiagnostic(actual)})
		}
	}
	return diagnostics
}

func rustSuiteEmptySelection(plan RustSuitePlan) bool {
	return len(plan.CompileOnlyPackages)+len(plan.HelperBinaries)+len(plan.ExpectedFailures)+len(plan.ExpectedTimeouts) != 0
}

func prepareRustSuite(options RustSuiteOptions) (rustSuitePreflight, error) {
	source, err := suiteSource(options)
	if err == nil {
		err = validateRustSuiteSourceRole(options, source)
	}
	if err != nil {
		return rustSuitePreflight{}, errors.Join(err, errors.New("RustSuite fixed source differs"))
	}
	role := "FixedUpstreamBaseline"
	if err := options.phase("Toolchain"); err != nil {
		return rustSuitePreflight{}, err
	}
	toolchain, err := resolveToolchain(options.ProjectWorkingRoot, options.deadline())
	if err != nil {
		return rustSuitePreflight{}, preflightHelperError{err}
	}
	preflight := rustSuitePreflight{}
	selection, packages, selectionErr := readRustSuitePendingBaseline(options, toolchain)
	if selectionErr != nil {
		return rustSuitePreflight{}, selectionErr
	}
	compileOnly, helpers := append([]string{}, selection.CompileOnlyPackages...), append([]string{}, selection.HelperBinaries...)
	concurrency := selection.ExecutionProfile.NextestConcurrency
	testTimeout := time.Duration(selection.ExecutionProfile.TestTimeoutMilliseconds) * time.Millisecond
	excluded, err := rustSuiteTransientExclusions(packages, selection)
	if err != nil {
		return rustSuitePreflight{}, err
	}
	if testTimeout <= 0 || testTimeout >= core.StageDeadline() {
		return rustSuitePreflight{}, rustSuitePreflightError{Diagnostics: []RustSuiteDiagnostic{{Field: "execution-bounds", Expected: "positive per-test timeout below total deadline", Actual: testTimeout.String()}}}
	}
	plan := RustSuitePlan{
		SchemaVersion: core.SchemaVersion, RecordType: "RustSuitePlanRecord", Scope: options.Scope, Tag: options.Tag, SourceRole: role, BaselineReanchor: options.baselineReanchor,
		ToolchainRoot: toolchain.Root, CargoVersion: toolchain.CargoVersion, ToolchainTarget: toolchain.Target,
		CacheRoot:               filepath.Join(options.ProjectWorkingRoot, "build", "cache"),
		TestNetwork:             "Shared",
		NextestConcurrency:      concurrency,
		TestTimeoutMilliseconds: testTimeout.Milliseconds(),
		CompileOnlyPackages:     compileOnly, HelperBinaries: helpers,
		ExcludedPackages: excluded, ExpectedFailures: []string{}, ExpectedTimeouts: []string{},
	}
	preflight.Plan, preflight.Source = plan, source
	return preflight, nil
}

func establishedRustSuitePlan(options RustSuiteOptions, toolchain ToolchainProjection, sourceRole string) (RustSuitePlan, error) {
	generation, err := readRustSuiteEstablishedGeneration(options, toolchain)
	if err != nil {
		return RustSuitePlan{}, err
	}
	selection := generation.Selection
	packages, packageErr := discoverRustSuitePackages(options, toolchain)
	excluded, exclusionErr := rustSuiteTransientExclusions(packages, selection.rustSuiteBaselineProposal)
	if packageErr != nil || exclusionErr != nil {
		return RustSuitePlan{}, errors.Join(packageErr, exclusionErr)
	}
	testTimeout := time.Duration(selection.ExecutionProfile.TestTimeoutMilliseconds) * time.Millisecond
	if testTimeout <= 0 || testTimeout >= core.StageDeadline() {
		return RustSuitePlan{}, rustSuitePreflightError{Diagnostics: []RustSuiteDiagnostic{{Field: "execution-bounds", Expected: "positive per-test timeout below total deadline", Actual: testTimeout.String()}}}
	}
	plan := RustSuitePlan{
		SchemaVersion: core.SchemaVersion, RecordType: "RustSuitePlanRecord", Scope: options.Scope, Tag: options.Tag, SourceRole: sourceRole,
		ToolchainRoot: toolchain.Root, CargoVersion: toolchain.CargoVersion, ToolchainTarget: toolchain.Target,
		CacheRoot:               filepath.Join(options.ProjectWorkingRoot, "build", "cache"),
		TestNetwork:             "Shared",
		NextestConcurrency:      selection.ExecutionProfile.NextestConcurrency,
		TestTimeoutMilliseconds: testTimeout.Milliseconds(),
		CompileOnlyPackages:     append([]string{}, selection.CompileOnlyPackages...),
		HelperBinaries:          append([]string{}, selection.HelperBinaries...),
		ExcludedPackages:        excluded,
		ExpectedFailures:        flattenRustSuiteExpectations(selection.ExpectedFailures),
		ExpectedTimeouts:        flattenRustSuiteExpectations(selection.ExpectedTimeouts),
	}
	if options.OnPhase != nil {
		options.OnPhase(fmt.Sprintf(
			"Established suite: covered=%d compile-only=%d helpers=%d known-failures=%d known-timeouts=%d excluded=%d network=%s concurrency=%d test=%s baseline=%s",
			len(selection.CoveredPackages), len(plan.CompileOnlyPackages), len(plan.HelperBinaries), len(plan.ExpectedFailures), len(plan.ExpectedTimeouts), len(plan.ExcludedPackages), plan.TestNetwork, plan.NextestConcurrency, testTimeout, time.Duration(selection.ExecutionProfile.SuiteTimeoutMilliseconds)*time.Millisecond,
		))
	}
	return plan, nil
}

func suiteSource(options RustSuiteOptions) (sourceProjection, error) {
	if err := options.phase("SourceProjection"); err != nil {
		return sourceProjection{}, err
	}
	source, err := runnerSource(options.Source, options.ProjectWorkingRoot, options.ProjectScratchRoot, options.RuntimeTemporaryRoot, options.Tag, options.deadline())
	if err != nil {
		err = errors.Join(err, errors.New("SourceProjection failed"))
	}
	return source, err
}

func validateRustSuiteSourceRole(options RustSuiteOptions, source sourceProjection) error {
	roles, err := readReleaseWorktrees(options.Source, options.ProjectScratchRoot, options.Tag, options.deadline())
	if err != nil || source == (sourceProjection{}) || roles.Registered[source.SourceBranch] != options.Source {
		return errors.Join(err, errors.New("RustSuite registered source role differs"))
	}
	if source.SourceBranch != options.Tag+"-upstream-baseline" {
		return errors.New("RustSuite baseline source role differs")
	}
	return nil
}

func validRustSuiteProductBranch(tag, branch string) bool {
	if branch == tag+"-main-worktree" {
		return true
	}
	prefix := tag + "-main-worktree-forked-"
	ordinal := strings.TrimPrefix(branch, prefix)
	if ordinal == branch || len(ordinal) != 3 {
		return false
	}
	value, err := strconv.Atoi(ordinal)
	return err == nil && value >= 1 && value <= 999 && fmt.Sprintf("%03d", value) == ordinal
}

func validateRustSuiteOptions(options RustSuiteOptions) error {
	if options.Scope != "baseline" || !options.releaseBaseline {
		return errors.New("baseline execution is internal to release-baseline")
	}
	if !strings.HasPrefix(options.Tag, "rust-v") || !validName(options.Tag, "-._") {
		return errors.New("RustSuite option differs")
	}
	expectedBaseline := filepath.Join(options.ProjectScratchRoot, "tasks", "rust-suite-baselines.json")
	if options.BaselineSelectionPath != expectedBaseline {
		return errors.New("RustSuite baseline selection differs")
	}
	if err := validateProjectionRoots(options.SkillRoot, options.ProjectWorkingRoot, options.ProjectScratchRoot, options.RuntimeTemporaryRoot, options.RecordRoot, options.Source); err != nil {
		return err
	}
	_, err := activeTaskPath(options.ProjectScratchRoot, options.Tag, "")
	if err != nil {
		return err
	}
	return nil
}

func readRustSuiteBaselineFile(path, tag string) (rustSuiteBaselineFile, []byte, error) {
	if !core.ValidReleaseTag(tag) || noFollowPath("RustSuite baseline selection", path) != nil {
		return rustSuiteBaselineFile{}, nil, errors.New("invalid baseline collection selection")
	}
	if err := initializeAbsentJSON(path, map[string]json.RawMessage{}); err != nil {
		return rustSuiteBaselineFile{}, nil, err
	}
	var entries map[string]json.RawMessage
	data, err := readCanonicalValue(path, &entries)
	if err == nil {
		err = validateJSONUniqueKeys(data)
	}
	if err != nil || entries == nil || len(entries) > 2 {
		return rustSuiteBaselineFile{}, nil, errors.Join(err, errors.New("baseline collection differs"))
	}
	selected := rustSuiteBaselineFile{}
	for entryTag, raw := range entries {
		value, err := decodeRustSuiteBaselineFile(raw)
		if err != nil || !core.ValidReleaseTag(entryTag) || value.Established.Selection.Tag != entryTag {
			return rustSuiteBaselineFile{}, nil, errors.Join(err, errors.New("baseline collection tag generation differs: "+entryTag))
		}
		if entryTag == tag {
			selected = value
		}
	}
	return selected, data, nil
}

func decodeRustSuiteBaselineFile(data []byte) (rustSuiteBaselineFile, error) {
	var value rustSuiteBaselineFile
	var fields map[string]json.RawMessage
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&value) != nil || json.Unmarshal(data, &fields) != nil || fields == nil {
		return value, errors.New("baseline generation fields differ")
	}
	expected, err := core.CanonicalJSON(value)
	var actualCompact, expectedCompact bytes.Buffer
	if err != nil || json.Compact(&actualCompact, data) != nil || json.Compact(&expectedCompact, expected) != nil || !bytes.Equal(actualCompact.Bytes(), expectedCompact.Bytes()) {
		return value, errors.Join(err, errors.New("baseline generation is not canonical"))
	}
	if value.Established == nil || !sameStrings(sortedMapKeys(fields), []string{"Established"}) {
		return rustSuiteBaselineFile{}, errors.New("baseline file fields differ")
	}
	var generationFields, selectionFields map[string]json.RawMessage
	if json.Unmarshal(fields["Established"], &generationFields) != nil || !sameStrings(sortedMapKeys(generationFields), []string{"Selection"}) || json.Unmarshal(generationFields["Selection"], &selectionFields) != nil || !sameStrings(sortedMapKeys(selectionFields), baselineSelectionFields()) {
		return rustSuiteBaselineFile{}, errors.New("established baseline fields differ")
	}

	return value, nil
}

func baselineSelectionFields() []string {
	return []string{"CargoVersion", "CompileOnlyPackages", "CoveredPackages", "ExecutionProfile", "ExpectedFailures", "ExpectedTimeouts", "HelperBinaries", "Tag", "ToolchainRoot", "ToolchainTarget"}
}

func readRustSuitePendingBaseline(options RustSuiteOptions, toolchain ToolchainProjection) (rustSuiteBaselineProposal, []string, error) {
	if options.baselinePreimage == nil || options.baselineProposal == nil || !validRustSuiteBaselineSelection(*options.baselinePreimage, options.baselinePreimage.Tag, toolchain) || !validRustSuiteBaselineProposal(*options.baselineProposal, options.Tag, toolchain) {
		return rustSuiteBaselineProposal{}, nil, rustSuitePreflightError{Diagnostics: []RustSuiteDiagnostic{{Field: "baseline-proposal", Expected: "one valid Plan-bound proposal and Established preimage", Actual: "generation differs"}}}
	}
	packages, packageErr := discoverRustSuitePackages(options, toolchain)
	if packageErr != nil {
		return rustSuiteBaselineProposal{}, nil, packageErr
	}
	execution, _, executionErr := rustSuiteBaselineExecution(*options.baselinePreimage, *options.baselineProposal, packages, options.baselineReanchor)
	return execution, packages, executionErr
}

func discoverRustSuitePackages(options RustSuiteOptions, toolchain ToolchainProjection) ([]string, error) {
	if err := options.phase("PackageDiscovery"); err != nil {
		return nil, err
	}
	root, script, err := prepareNativeAttempt(options.RuntimeTemporaryRoot, "rust-suite-packages")
	if err != nil {
		return nil, preflightHelperError{errors.Join(err, errors.New("PackageDiscovery failed"))}
	}
	if err := options.deadline().check(); err != nil {
		return nil, errors.Join(err, removePrivateAttemptWithin(root, newDeadline(core.GracePeriod(), nil)))
	}
	output := filepath.Join(root, "result")
	data, stderr, runErr := runNativeProcess(root, script, "rust-suite-packages", []string{options.Source, filepath.Join(toolchain.Path, "rust", "bin", "cargo"), filepath.Join(options.ProjectWorkingRoot, "build", "cache", "cargo-home"), output}, nativeEnvironment(root), 8<<20, output, options.remaining(), options.Cancellation, options.OnLive)
	runErr = errors.Join(runErr, removePrivateAttemptWithin(root, newDeadline(core.GracePeriod(), nil)))
	if diagnostic := strings.TrimSpace(stderr); runErr != nil && diagnostic != "" {
		runErr = errors.Join(runErr, errors.New(diagnostic))
	}
	packages := []string{}
	if runErr == nil {
		runErr = json.Unmarshal(data, &packages)
	}
	if runErr != nil || len(packages) == 0 || !validRustSuitePackages(packages) {
		return nil, preflightHelperError{errors.Join(runErr, errors.New("PackageDiscovery failed"))}
	}
	return packages, nil
}

func rustSuiteBaselineExecution(established rustSuiteBaselineInput, proposed rustSuiteBaselineProposal, packages []string, align bool) (rustSuiteBaselineProposal, rustSuiteBaselineProposal, error) {
	members := rustSuitePackageSet(packages)
	aligned := proposed
	if established.Tag != proposed.Tag {
		if align {
			return rustSuiteBaselineProposal{}, rustSuiteBaselineProposal{}, errors.New("alignment needs the current Established tag")
		}
		aligned.CompileOnlyPackages = rustSuitePresentPackages(proposed.CompileOnlyPackages, members)
		aligned.HelperBinaries = rustSuitePresentPackages(proposed.HelperBinaries, members)
		aligned.CoveredPackages = rustSuitePresentPackages(proposed.CoveredPackages, members)
		return aligned, aligned, nil
	}
	for _, names := range [][]string{proposed.CompileOnlyPackages, proposed.HelperBinaries, proposed.CoveredPackages} {
		for _, name := range names {
			if !members[name] {
				return rustSuiteBaselineProposal{}, rustSuiteBaselineProposal{}, errors.New("package absent from workspace")
			}
		}
	}
	if align {
		return proposed, proposed, nil
	}
	for _, pair := range [][2][]string{{established.CoveredPackages, proposed.CoveredPackages}, {established.CompileOnlyPackages, proposed.CompileOnlyPackages}, {established.HelperBinaries, proposed.HelperBinaries}} {
		current := rustSuitePackageSet(pair[1])
		for _, name := range pair[0] {
			if !current[name] {
				return rustSuiteBaselineProposal{}, rustSuiteBaselineProposal{}, errors.New("automatic coverage must retain established roles")
			}
		}
	}
	if rustSuiteSameValue(established.rustSuiteBaselineProposal, proposed) {
		return proposed, proposed, nil
	}
	oldCovered := rustSuitePackageSet(established.CoveredPackages)
	oldCompile, oldHelpers := rustSuitePackageSet(established.CompileOnlyPackages), rustSuitePackageSet(established.HelperBinaries)
	execution := proposed
	execution.CompileOnlyPackages, execution.HelperBinaries, execution.CoveredPackages = []string{}, []string{}, []string{}
	for _, name := range proposed.CoveredPackages {
		if !oldCovered[name] {
			execution.CoveredPackages = append(execution.CoveredPackages, name)
		}
	}
	for _, name := range proposed.CompileOnlyPackages {
		if !oldCompile[name] {
			execution.CompileOnlyPackages = append(execution.CompileOnlyPackages, name)
		}
	}
	for _, name := range proposed.HelperBinaries {
		if len(execution.CoveredPackages) != 0 || !oldHelpers[name] {
			execution.HelperBinaries = append(execution.HelperBinaries, name)
		}
	}
	return execution, aligned, nil
}

func rustSuitePresentPackages(names []string, members map[string]bool) []string {
	present := []string{}
	for _, name := range names {
		if members[name] {
			present = append(present, name)
		}
	}
	return present
}

func readRustSuiteEstablishedGeneration(options RustSuiteOptions, toolchain ToolchainProjection) (rustSuiteBaselineGeneration, error) {
	value, _, err := readRustSuiteBaselineFile(options.BaselineSelectionPath, options.Tag)
	if err != nil || value.Established == nil || !validRustSuiteBaselineSelection(value.Established.Selection, options.Tag, toolchain) {
		return rustSuiteBaselineGeneration{}, rustSuitePreflightError{Diagnostics: []RustSuiteDiagnostic{{Field: "baseline-established", Expected: "one valid established generation for this tag", Actual: errors.Join(err, errors.New("generation differs")).Error()}}}
	}
	return *value.Established, nil
}

func validRustSuiteBaselineProposal(selection rustSuiteBaselineProposal, tag string, toolchain ToolchainProjection) bool {
	return validRustSuiteBaselineCommon(selection.Tag, selection.ToolchainRoot, selection.CargoVersion, selection.ToolchainTarget, selection.CompileOnlyPackages, selection.HelperBinaries, selection.CoveredPackages, selection.ExecutionProfile, tag, toolchain)
}

func validRustSuiteBaselineSelection(selection rustSuiteBaselineInput, tag string, toolchain ToolchainProjection) bool {
	if !validRustSuiteBaselineCommon(selection.Tag, selection.ToolchainRoot, selection.CargoVersion, selection.ToolchainTarget, selection.CompileOnlyPackages, selection.HelperBinaries, selection.CoveredPackages, selection.ExecutionProfile, tag, toolchain) || !validRustSuiteExpectations(selection.ExpectedFailures) || !validRustSuiteExpectations(selection.ExpectedTimeouts) {
		return false
	}
	covered := rustSuitePackageSet(selection.CoveredPackages)
	failures, timeouts := flattenRustSuiteExpectations(selection.ExpectedFailures), flattenRustSuiteExpectations(selection.ExpectedTimeouts)
	for _, identity := range failures {
		packageName, _, valid := strings.Cut(identity, " ")
		position := sort.SearchStrings(timeouts, identity)
		if !valid || !covered[packageName] || position < len(timeouts) && timeouts[position] == identity {
			return false
		}
	}
	for _, identity := range timeouts {
		packageName, _, valid := strings.Cut(identity, " ")
		if !valid || !covered[packageName] {
			return false
		}
	}
	return true
}

func rustSuiteRoundedBudget(elapsed int64) int64 {
	return ((elapsed-1)/rustSuiteBudgetStep + 1) * rustSuiteBudgetStep
}

func validRustSuiteTiming(profile rustSuiteBaselineProfile) bool {
	limit := profile.SuiteTimeoutMilliseconds
	if limit <= 0 || limit > core.StageDeadline().Milliseconds() || limit%rustSuiteBudgetStep != 0 || profile.TestTimeoutMilliseconds <= 0 || profile.TestTimeoutMilliseconds > 120000 || profile.TestTimeoutMilliseconds >= limit {
		return false
	}
	elapsed := profile.BaselineElapsedMilliseconds
	return elapsed == nil || *elapsed > 0 && *elapsed <= core.StageDeadline().Milliseconds() && rustSuiteRoundedBudget(*elapsed) == limit
}

func publishRustSuiteBaselineTiming(options RustSuiteOptions) error {
	if options.calibrationPlan == nil || options.calibrationPlan.SchemaVersion == 0 {
		return nil
	}
	value, preimage, err := readRustSuiteBaselineFile(options.BaselineSelectionPath, options.Tag)
	if err != nil || value.Established == nil {
		return errors.Join(err, errors.New("complete baseline timing input differs"))
	}
	plan := *options.calibrationPlan
	selection := &value.Established.Selection
	packages := sortedRustSuitePackageUnion(plan.ExcludedPackages, selection.CoveredPackages)
	if !rustSuitePlanMatchesProposal(plan, selection.rustSuiteBaselineProposal, packages) {
		return nil
	}
	if err := options.deadline().check(); err != nil {
		return err
	}
	elapsed := time.Since(options.started).Milliseconds()
	selection.ExecutionProfile.BaselineElapsedMilliseconds = &elapsed
	selection.ExecutionProfile.SuiteTimeoutMilliseconds = rustSuiteRoundedBudget(elapsed)
	if !validRustSuiteTiming(selection.ExecutionProfile) {
		return errors.New("complete baseline timing differs")
	}
	return publishRustSuiteBaselineFile(options.BaselineSelectionPath, options.Tag, rustSuiteBaselinePredecessor(options), preimage, value)
}

func validRustSuiteBaselineCommon(tag, root, cargoVersion, target string, compileOnly, helpers, coveredPackages []string, profile rustSuiteBaselineProfile, expectedTag string, toolchain ToolchainProjection) bool {
	if profile.TestNetwork != "Shared" {
		return false
	}
	missing := compileOnly == nil || helpers == nil || coveredPackages == nil
	if missing || tag != expectedTag || root != toolchain.Root || cargoVersion != toolchain.CargoVersion || target != toolchain.Target || !validRustSuitePackages(compileOnly) || !validRustSuitePackages(helpers) || !validRustSuitePackages(coveredPackages) || profile.NextestConcurrency <= 0 || !validRustSuiteTiming(profile) {
		return false
	}
	covered, special := rustSuitePackageSet(coveredPackages), map[string]bool{}
	for _, name := range append(append([]string{}, compileOnly...), helpers...) {
		if special[name] || covered[name] {
			return false
		}
		special[name] = true
	}
	return true
}

func validRustSuiteExpectations(expectations map[string][]string) bool {
	for packageName, tests := range expectations {
		if !validName(packageName, "-_") || len(tests) == 0 {
			return false
		}
		for index, test := range tests {
			if test == "" || strings.ContainsAny(test, "\x00\r\n") || index > 0 && tests[index-1] >= test {
				return false
			}
		}
	}
	return true
}

func flattenRustSuiteExpectations(expectations map[string][]string) []string {
	result := []string{}
	for packageName, tests := range expectations {
		for _, test := range tests {
			result = append(result, packageName+" "+test)
		}
	}
	sort.Strings(result)
	return result
}

func rustSuiteExpectationMap(outcomes []string) map[string][]string {
	expectations := make(map[string][]string)
	for _, outcome := range outcomes {
		packageName, test, _ := strings.Cut(outcome, " ")
		expectations[packageName] = append(expectations[packageName], test)
	}
	return expectations
}

func sortedMapKeys(values map[string]json.RawMessage) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func validRustSuitePackages(packages []string) bool {
	for index, name := range packages {
		if !validName(name, "-_") || index > 0 && packages[index-1] >= name {
			return false
		}
	}
	return true
}

func establishZeroRustSuiteBaseline(options RustSuiteOptions) error {
	expected, err := zeroRustSuiteBaseline(options)
	value, preimage, readErr := readRustSuiteBaselineFile(options.BaselineSelectionPath, options.Tag)
	if err != nil || readErr != nil {
		return errors.Join(err, readErr)
	}
	if value.Established != nil {
		if rustSuiteSameValue(value, expected) {
			return nil
		}
		return errors.New("zero-coverage baseline input differs")
	}
	return publishRustSuiteBaselineWithin(options, preimage, expected, "ResultPublication")
}

func zeroRustSuiteBaseline(options RustSuiteOptions) (rustSuiteBaselineFile, error) {
	if err := options.phase("Toolchain"); err != nil {
		return rustSuiteBaselineFile{}, err
	}
	toolchain, err := resolveToolchain(options.ProjectWorkingRoot, options.deadline())
	if err != nil {
		return rustSuiteBaselineFile{}, err
	}
	_, err = discoverRustSuitePackages(options, toolchain)
	if err != nil {
		return rustSuiteBaselineFile{}, err
	}
	proposal := rustSuiteBaselineProposal{
		Tag: options.Tag, ToolchainRoot: toolchain.Root, CargoVersion: toolchain.CargoVersion, ToolchainTarget: toolchain.Target,
		CompileOnlyPackages: []string{}, HelperBinaries: []string{}, CoveredPackages: []string{},
		ExecutionProfile: rustSuiteBaselineProfile{TestNetwork: "Shared", NextestConcurrency: core.DefaultNextestConcurrency, TestTimeoutMilliseconds: 120000, SuiteTimeoutMilliseconds: core.StageDeadline().Milliseconds()},
	}
	value, _, err := readRustSuiteBaselineFile(options.BaselineSelectionPath, options.Tag)
	if err != nil {
		return rustSuiteBaselineFile{}, err
	}
	if value.Established != nil {
		selection := value.Established.Selection
		if !zeroRustSuiteProposal(selection.rustSuiteBaselineProposal) || !validRustSuiteBaselineSelection(selection, options.Tag, toolchain) {
			return rustSuiteBaselineFile{}, errors.New("established zero baseline differs")
		}
		proposal = selection.rustSuiteBaselineProposal
	} else if options.baselineProposal != nil {
		if !zeroRustSuiteProposal(*options.baselineProposal) || !validRustSuiteBaselineProposal(*options.baselineProposal, options.Tag, toolchain) {
			return rustSuiteBaselineFile{}, errors.New("inherited zero baseline proposal differs")
		}
		proposal = *options.baselineProposal
		proposal.ExecutionProfile.BaselineElapsedMilliseconds = nil
	}
	selection := rustSuiteBaselineInput{rustSuiteBaselineProposal: proposal, ExpectedFailures: map[string][]string{}, ExpectedTimeouts: map[string][]string{}}
	return rustSuiteBaselineFile{Established: &rustSuiteBaselineGeneration{Selection: selection}}, nil
}

func zeroRustSuiteProposal(proposal rustSuiteBaselineProposal) bool {
	return proposal.CoveredPackages != nil && proposal.CompileOnlyPackages != nil && proposal.HelperBinaries != nil && len(proposal.CoveredPackages)+len(proposal.CompileOnlyPackages)+len(proposal.HelperBinaries) == 0
}

func rustSuiteBaselinePredecessor(options RustSuiteOptions) string {
	if options.baselinePreimage != nil {
		return options.baselinePreimage.Tag
	}
	return ""
}

func publishRustSuiteBaselineFile(path, tag, predecessor string, preimage []byte, value rustSuiteBaselineFile) error {
	var entries map[string]json.RawMessage
	if !core.ValidReleaseTag(tag) || predecessor != "" && !core.ValidReleaseTag(predecessor) || value.Established == nil || value.Established.Selection.Tag != tag || json.Unmarshal(preimage, &entries) != nil || entries == nil {
		return errors.New("baseline collection publication preimage differs")
	}
	selected, err := core.CanonicalJSON(value)
	if err != nil {
		return err
	}
	if predecessor != tag {
		retained, present := entries[predecessor]
		entries = map[string]json.RawMessage{}
		if predecessor != "" {
			if !present {
				return errors.New("baseline collection predecessor differs")
			}
			entries[predecessor] = retained
		}
	}
	entries[tag] = selected
	if len(entries) > 2 {
		return errors.New("baseline collection publication differs")
	}
	postimage, err := core.CanonicalJSON(entries)
	if err != nil {
		return err
	}
	return publishExactPostimage(path, preimage, postimage)
}

func publishRustSuiteBaselineWithin(options RustSuiteOptions, preimage []byte, value rustSuiteBaselineFile, phase string) error {
	if err := options.phase(phase); err != nil {
		return err
	}
	return errors.Join(publishRustSuiteBaselineFile(options.BaselineSelectionPath, options.Tag, rustSuiteBaselinePredecessor(options), preimage, value), options.deadline().check())
}

func rustSuiteCargoStage(scope string) string {
	return "FinalProductRustSuite/" + scope
}

func rustSuiteCommand(plan RustSuitePlan) []string {
	data, _ := core.CanonicalJSON(plan)
	return []string{"/usr/bin/python3", "/runner/release-roll.py", "rust-suite-execute", "/workspace-lower", "/workspace", string(data)}
}

func rustSuiteConsumer(plan RustSuitePlan) runnerConsumer {
	environment := []string{"CARGO_NET_OFFLINE=true", "INSTA_UPDATE=no", "NEXTEST_EXPERIMENTAL_LIBTEST_JSON=1"}
	consumerAssets := []runtimeAsset{
		{name: "release-roll.py", data: assets.ReleaseRoll(), mode: 0o700},
	}
	return runnerConsumer{rustSuite: true, environment: environment, assets: consumerAssets, ephemeralSourcePaths: []string{"codex-rs/cloud-tasks/error.log"}}
}

func readRustSuitePreparation(projection projectionResult, plan RustSuitePlan) (RustSuitePreparation, error) {
	file, exists := projection.Files["rust-suite-preparation.json"]
	if !exists {
		return RustSuitePreparation{}, errors.New("RustSuite preparation result is absent")
	}
	var preparation RustSuitePreparation
	_, err := readCanonicalValue(file.Path, &preparation)
	if err != nil || !validRustSuitePreparation(plan, preparation) {
		return RustSuitePreparation{}, errors.Join(err, errors.New("RustSuite preparation result differs"))
	}
	return preparation, nil
}

func sortedHas(values []string, value string) bool {
	position := sort.SearchStrings(values, value)
	return position < len(values) && values[position] == value
}

func validRustSuitePreparation(plan RustSuitePlan, preparation RustSuitePreparation) bool {
	if !listed("Complete Fail", preparation.Terminal) || !sameStrings(preparation.CompileOnlyPackages, plan.CompileOnlyPackages) || !sameStrings(preparation.HelperBinaries, plan.HelperBinaries) || !validRustSuitePackages(preparation.SelectedPackages) || !validRustSuiteDiscoveredTests(preparation.SelectedTests) {
		return false
	}
	previous, environments, helpers := "", map[string]bool{}, map[string]bool{}
	for _, binding := range preparation.BinaryBindings {
		name := filepath.Base(binding.Path)
		normalized := strings.ReplaceAll(name, "-", "_")
		expected := []string{"CARGO_BIN_EXE_" + name, "NEXTEST_BIN_EXE_" + normalized}
		if normalized != name {
			expected = append(expected, "CARGO_BIN_EXE_"+normalized)
		}
		sort.Strings(expected)
		key := binding.Package + "\x00" + binding.Path
		helper := sortedHas(plan.HelperBinaries, binding.Package)
		if key <= previous || !validName(binding.Package, "-_") || sortedHas(plan.ExcludedPackages, binding.Package) && !helper || binding.Path != filepath.Join("/cache/target/debug", name) || !validName(name, "-_") || !sameStrings(binding.Environments, expected) {
			return false
		}
		for _, environment := range binding.Environments {
			if environments[environment] {
				return false
			}
			environments[environment] = true
		}
		previous = key
		if helper {
			helpers[binding.Package] = true
		}
	}
	selected := map[string]bool{}
	for _, name := range preparation.SelectedPackages {
		selected[name] = true
	}
	for _, test := range preparation.SelectedTests {
		if !selected[test.Package] {
			return false
		}
	}
	if preparation.Terminal == "Fail" {
		return len(preparation.Diagnostics) != 0 && len(preparation.BinaryBindings) == 0 && len(preparation.SelectedPackages) == 0 && len(preparation.SelectedTests) == 0
	}
	maintenanceOnly := rustSuiteEmptySelection(plan) && len(preparation.SelectedPackages) == 0 && len(preparation.SelectedTests) == 0
	if len(preparation.Diagnostics) != 0 || !maintenanceOnly && (len(preparation.BinaryBindings) == 0 || len(preparation.SelectedPackages) == 0 || len(preparation.SelectedTests) == 0) {
		return false
	}
	for _, helper := range plan.HelperBinaries {
		if !helpers[helper] {
			return false
		}
	}
	for _, name := range preparation.SelectedPackages {
		if sortedHas(plan.ExcludedPackages, name) {
			return false
		}
	}
	return true
}

func validRustSuiteDiscoveredTests(tests []RustSuiteDiscoveredTest) bool {
	resultNames := map[string]bool{}
	for index, test := range tests {
		plain := func(value string) bool { return value != "" && !strings.ContainsAny(value, "\x00\r\n") }
		if !plain(test.BinaryID) || !plain(test.BinaryName) || !plain(test.TestName) || !validName(test.Package, "-_") || test.Identity != test.BinaryID+" "+test.TestName || test.ResultName != test.Package+"::"+test.BinaryName+"$"+test.TestName || test.ExpectationIdentity != test.Package+" "+test.TestName || index > 0 && tests[index-1].Identity >= test.Identity || resultNames[test.ResultName] {
			return false
		}
		resultNames[test.ResultName] = true
	}
	return tests != nil
}

func readRustSuiteExecution(window deadline, projection projectionResult, preparation RustSuitePreparation, testsStarted bool) (rustSuiteExecution, error) {
	file, exists := projection.Files["nextest.jsonl"]
	if !exists {
		return rustSuiteExecution{}, errors.New("RustSuite runner output is absent")
	}
	data, err := readRegularNoFollow(file.Path, window)
	if err != nil {
		return rustSuiteExecution{}, err
	}
	content := strings.TrimSpace(string(data))
	interrupted := projection.TimedOut || projection.Canceled || projection.ExitCode != 0
	if content == "" && (!testsStarted || len(preparation.SelectedTests) == 0) {
		return rustSuiteExecution{rustSuiteOutcomes: rustSuiteOutcomes{}, SelectedPackages: []string{}, RustSuiteExecutionSummary: RustSuiteExecutionSummary{Validated: true}}, nil
	}
	selectedByResult, selectedByIdentity := map[string]RustSuiteDiscoveredTest{}, map[string]RustSuiteDiscoveredTest{}
	for _, test := range preparation.SelectedTests {
		selectedByResult[test.ResultName] = test
		selectedByIdentity[test.Identity] = test
	}
	open := map[string]bool{}
	terminals := map[string]RustSuiteTestOutcome{}
	lines := strings.Split(content, "\n")
	for index, line := range lines {
		if err := window.check(); err != nil {
			return rustSuiteExecution{}, err
		}
		var event struct {
			Type     string   `json:"type"`
			Event    string   `json:"event"`
			Name     string   `json:"name"`
			Reason   string   `json:"reason"`
			ExecTime *float64 `json:"exec_time"`
		}
		if json.Unmarshal([]byte(line), &event) != nil {
			if interrupted && index == len(lines)-1 {
				continue
			}
			return rustSuiteExecution{}, errors.New("RustSuite runner output differs")
		}
		if event.Type == "codex-build-round" && event.Event == "continued" {
			clear(open)
		}
		if event.Type != "test" {
			continue
		}
		resultName := event.Name
		test, selected := selectedByResult[resultName]
		if !selected && event.Event != "started" && (strings.HasSuffix(resultName, "#2") || strings.HasSuffix(resultName, "#3")) {
			test, selected = selectedByResult[resultName[:len(resultName)-2]]
		}
		if !selected {
			return rustSuiteExecution{}, fmt.Errorf("RustSuite execution result is absent from discovery: %s", resultName)
		}
		if event.Event == "started" {
			if test.Ignored {
				open[test.Identity] = true
				continue
			}
			if open[test.Identity] {
				return rustSuiteExecution{}, errors.New("RustSuite test start sequence differs")
			}
			if previous, exists := terminals[test.Identity]; exists && !listed("Failure Timeout", previous.Terminal) {
				return rustSuiteExecution{}, errors.New("RustSuite test retry admission differs")
			}
			open[test.Identity] = true
			continue
		}
		terminal := ""
		switch {
		case event.Event == "ignored" && test.Ignored:
			terminal = "Skipped"
		case test.Ignored || event.Event == "ignored":
			return rustSuiteExecution{}, errors.New("RustSuite ignored terminal differs from discovery")
		case event.Event == "ok":
			terminal = "Pass"
		case event.Event == "failed" && event.Reason == "time limit exceeded":
			terminal = "Timeout"
		case event.Event == "failed":
			terminal = "Failure"
		default:
			return rustSuiteExecution{}, errors.New("RustSuite test terminal differs")
		}
		elapsed := int64(0)
		if event.ExecTime != nil {
			if math.IsNaN(*event.ExecTime) || math.IsInf(*event.ExecTime, 0) || *event.ExecTime < 0 {
				return rustSuiteExecution{}, errors.New("RustSuite test elapsed time differs")
			}
			elapsed = int64(math.Ceil(*event.ExecTime * 1000))
		} else if terminal != "Skipped" {
			return rustSuiteExecution{}, errors.New("RustSuite test elapsed time is absent")
		}
		previous, exists := terminals[test.Identity]
		if terminal != "Skipped" && !open[test.Identity] {
			return rustSuiteExecution{}, errors.New("RustSuite test terminal start is absent")
		}
		open[test.Identity] = false
		if exists {
			if terminal == "Skipped" && previous.Terminal == terminal {
				continue
			}
			if test.Ignored || !listed("Failure Timeout", previous.Terminal) {
				return rustSuiteExecution{}, errors.New("RustSuite test terminal is duplicated")
			}
		}
		terminals[test.Identity] = RustSuiteTestOutcome{Identity: test.Identity, Terminal: terminal, ElapsedMilliseconds: elapsed}
	}
	for _, test := range preparation.SelectedTests {
		if !test.Ignored && !interrupted && open[test.Identity] {
			return rustSuiteExecution{}, errors.New("RustSuite test terminal is absent")
		}
		if _, exists := terminals[test.Identity]; exists {
			continue
		}
		if test.Ignored {
			terminals[test.Identity] = RustSuiteTestOutcome{Identity: test.Identity, Terminal: "Skipped", ElapsedMilliseconds: 0}
		} else if !interrupted {
			return rustSuiteExecution{}, errors.New("RustSuite test terminal is absent")
		}
	}
	execution := rustSuiteExecution{SelectedPackages: append([]string{}, preparation.SelectedPackages...)}
	failures, timeouts := map[string]bool{}, map[string]bool{}
	for _, outcome := range terminals {
		execution.CompletedTests++
		if outcome.Terminal == "Skipped" {
			execution.SkippedTests++
		}
		test := selectedByIdentity[outcome.Identity]
		if outcome.Terminal == "Failure" {
			failures[test.ExpectationIdentity] = true
		} else if outcome.Terminal == "Timeout" {
			timeouts[test.ExpectationIdentity] = true
		}
	}
	for identity := range failures {
		execution.Failures = append(execution.Failures, identity)
	}
	for identity := range timeouts {
		execution.Timeouts = append(execution.Timeouts, identity)
	}
	sort.Strings(execution.Failures)
	sort.Strings(execution.Timeouts)
	if !interrupted && execution.CompletedTests != len(preparation.SelectedTests) {
		return rustSuiteExecution{}, errors.New("RustSuite execution inventory is empty")
	}
	execution.FailureCount, execution.TimeoutCount = len(execution.Failures), len(execution.Timeouts)
	execution.Validated = true
	return execution, nil
}

func subsetRustSuiteOutcomes(current, established rustSuiteOutcomes) bool {
	return subsetSortedStrings(current.Failures, established.Failures) && subsetSortedStrings(current.Timeouts, established.Timeouts)
}

func subsetSortedStrings(current, established []string) bool {
	position := 0
	for _, value := range current {
		for position < len(established) && established[position] < value {
			position++
		}
		if position == len(established) || established[position] != value {
			return false
		}
	}
	return true
}

func rustSuiteStageTerminal(plan RustSuitePlan, projection projectionResult, outcomes rustSuiteOutcomes) string {
	if projection.Canceled || projection.TimedOut {
		return "Stopped"
	}
	accepted := len(outcomes.Failures)+len(outcomes.Timeouts) <= rustSuiteBaselineOutcomeLimit
	if plan.Scope != "baseline" {
		accepted = accepted && subsetRustSuiteOutcomes(outcomes, rustSuiteOutcomes{Failures: plan.ExpectedFailures, Timeouts: plan.ExpectedTimeouts})
	}
	expectedExit := 0
	if len(outcomes.Failures)+len(outcomes.Timeouts) != 0 {
		expectedExit = 100
	}
	if !projection.SourceUnchanged || projection.FailureClass != nil || projection.ExitCode != expectedExit || !accepted {
		return "Fail"
	}
	return "Pass"
}

func rustSuiteFailureDomain(err error) string {
	var helper preflightHelperError
	if errors.As(err, &helper) {
		return "Helper"
	}
	return "Input"
}

func rustSuiteDiagnostics(err error) []RustSuiteDiagnostic {
	if err == nil {
		return []RustSuiteDiagnostic{}
	}
	var preflight rustSuitePreflightError
	if errors.As(err, &preflight) {
		return append([]RustSuiteDiagnostic{}, preflight.Diagnostics...)
	}
	return []RustSuiteDiagnostic{{Field: "preflight", Expected: "fixed product plan", Actual: err.Error()}}
}

func rustSuiteAppendDiagnostic(diagnostics *[]RustSuiteDiagnostic, field, expected, actual string, valid bool) {
	if !valid {
		*diagnostics = append(*diagnostics, RustSuiteDiagnostic{Field: field, Expected: expected, Actual: actual})
	}
}

func rustSuitePlanDiagnostics(plan RustSuitePlan, scope, tag string, source sourceProjection, toolchain ToolchainProjection, workingRoot string) []RustSuiteDiagnostic {
	diagnostics := []RustSuiteDiagnostic{}
	rustSuiteAppendDiagnostic(&diagnostics, "test-network", "Shared", plan.TestNetwork, plan.TestNetwork == "Shared")
	rustSuiteAppendDiagnostic(&diagnostics, "plan-schema", fmt.Sprint(core.SchemaVersion), fmt.Sprint(plan.SchemaVersion), plan.SchemaVersion == core.SchemaVersion)
	rustSuiteAppendDiagnostic(&diagnostics, "plan-type", "RustSuitePlanRecord", plan.RecordType, plan.RecordType == "RustSuitePlanRecord")
	rustSuiteAppendDiagnostic(&diagnostics, "plan-scope", scope, plan.Scope, plan.Scope == scope)
	rustSuiteAppendDiagnostic(&diagnostics, "plan-baseline-reanchor", "false outside baseline", fmt.Sprint(plan.BaselineReanchor), scope == "baseline" || !plan.BaselineReanchor)
	rustSuiteAppendDiagnostic(&diagnostics, "plan-tag", tag, plan.Tag, plan.Tag == tag)
	role := "ExplicitProductWorktree"
	if scope == "baseline" {
		role = "FixedUpstreamBaseline"
	}
	rustSuiteAppendDiagnostic(&diagnostics, "plan-source-role", role, plan.SourceRole, plan.SourceRole == role)
	rustSuiteAppendDiagnostic(&diagnostics, "plan-toolchain-root", toolchain.Root, plan.ToolchainRoot, toolchain.Root != "" && plan.ToolchainRoot == toolchain.Root)
	rustSuiteAppendDiagnostic(&diagnostics, "plan-cargo-version", toolchain.CargoVersion, plan.CargoVersion, toolchain.CargoVersion != "" && plan.CargoVersion == toolchain.CargoVersion)
	rustSuiteAppendDiagnostic(&diagnostics, "plan-toolchain-target", toolchain.Target, plan.ToolchainTarget, toolchain.Target != "" && plan.ToolchainTarget == toolchain.Target)
	expectedCache := filepath.Join(workingRoot, "build", "cache")
	rustSuiteAppendDiagnostic(&diagnostics, "plan-cache-root", expectedCache, plan.CacheRoot, plan.CacheRoot == expectedCache)
	rustSuiteAppendDiagnostic(&diagnostics, "plan-concurrency", "positive", fmt.Sprint(plan.NextestConcurrency), plan.NextestConcurrency > 0)
	rustSuiteAppendDiagnostic(&diagnostics, "plan-test-timeout", "positive and at most 120 seconds", fmt.Sprint(plan.TestTimeoutMilliseconds), plan.TestTimeoutMilliseconds > 0 && plan.TestTimeoutMilliseconds <= 120000)
	rustSuiteAppendDiagnostic(&diagnostics, "plan-compile-only-packages", "sorted unique packages excluded from execution", strings.Join(plan.CompileOnlyPackages, ","), validRustSuitePreparationPackageSets(plan))
	rustSuiteAppendDiagnostic(&diagnostics, "plan-helper-binaries", "sorted unique packages excluded from execution", strings.Join(plan.HelperBinaries, ","), validRustSuitePreparationPackageSets(plan))
	rustSuiteAppendDiagnostic(&diagnostics, "plan-excluded-packages", "sorted unique packages", strings.Join(plan.ExcludedPackages, ","), validRustSuitePackages(plan.ExcludedPackages))
	rustSuiteAppendDiagnostic(&diagnostics, "plan-expected-failures", "sorted unique outcomes", strings.Join(plan.ExpectedFailures, ","), sortedUniqueRustSuiteOutcomes(plan.ExpectedFailures))
	rustSuiteAppendDiagnostic(&diagnostics, "plan-expected-timeouts", "sorted unique outcomes", strings.Join(plan.ExpectedTimeouts, ","), sortedUniqueRustSuiteOutcomes(plan.ExpectedTimeouts))
	return diagnostics
}

func validRustSuitePreparationPackageSets(plan RustSuitePlan) bool {
	if !validRustSuitePackages(plan.CompileOnlyPackages) || !validRustSuitePackages(plan.HelperBinaries) {
		return false
	}
	excluded, seen := map[string]bool{}, map[string]bool{}
	for _, name := range plan.ExcludedPackages {
		excluded[name] = true
	}
	for _, name := range append(append([]string{}, plan.CompileOnlyPackages...), plan.HelperBinaries...) {
		if seen[name] || !excluded[name] {
			return false
		}
		seen[name] = true
	}
	return true
}

func rustSuiteValidCargoIdentity(identity cargoIdentity) bool {
	return identity.Generation > 0 && identity.CargoVersion != "" && identity.ToolchainRoot == nativeToolchainRoot && validName(identity.Target, "-_") && validCargoSourceSelection(identity.SourceSelection)
}

func validCargoSourceSelection(selection string) bool {
	return filepath.IsLocal(selection) && selection != "." && filepath.ToSlash(filepath.Clean(selection)) == selection
}

func validateRustSuiteStage(recordRoot, scratchRoot, runtimeRoot, source string, plan RustSuitePlan, stage RustSuiteStageResult, recoverTerminal bool) error {
	sourceSelection, selectionErr := filepath.Rel(scratchRoot, source)
	if selectionErr != nil || !validCargoSourceSelection(sourceSelection) {
		return errors.Join(selectionErr, errors.New("RustSuite physical source selection differs"))
	}
	if stage.SchemaVersion != core.SchemaVersion || stage.RecordType != "RustSuiteStageResultRecord" || stage.Scope != plan.Scope || stage.Tag != plan.Tag || !validName(stage.Stage, "-._") || !listed("Pass Fail Stopped", stage.Terminal) || !listed("StartedReaped CanceledReaped IncompleteBeforeDurableRunnerEvidence OwnershipLostBeforeDurableRunnerEvidence", stage.ProcessState) || !listed("ExactAttemptRootAbsent IncompleteAttemptCleanupComplete OwnershipLostCleanupComplete", stage.AttemptCleanup) || stage.ElapsedMilliseconds < 0 || !validRustSuiteDiagnostics(stage.Diagnostics) || !sortedUniqueRustSuiteOutcomes(stage.Failures) || !sortedUniqueRustSuiteOutcomes(stage.Timeouts) || stage.Observation != nil && (stage.ExitCode != stage.Observation.ExitCode || stage.TimedOut != stage.Observation.TimedOut || stage.Canceled != stage.Observation.Canceled || stage.ElapsedMilliseconds != stage.Observation.DurationMilliseconds) {
		return errors.New("RustSuite Stage fields differ")
	}
	if stage.TimedOut && (stage.ProcessState != "StartedReaped" || stage.Terminal != "Stopped" || stage.Canceled) {
		return errors.New("RustSuite timeout Stage differs")
	}
	if !recoverTerminal {
		if err := projectionAttemptAbsent(projectionAttemptRoot(filepath.Dir(filepath.Dir(plan.CacheRoot)), stage.Stage), runtimeRoot); err != nil {
			return err
		}
	}
	switch stage.ProcessState {
	case "OwnershipLostBeforeDurableRunnerEvidence":
		if stage.Terminal != "Stopped" || len(stage.Diagnostics) != 0 || stage.TimedOut || stage.Canceled || stage.ExitCode != -1 || stage.SourcePostimagePath != "" || stage.Preparation != nil || stage.Observation != nil || stage.MaterializationEvidence != nil || len(stage.Failures) != 0 || len(stage.Timeouts) != 0 || len(stage.SelectedPackages) != 0 || stage.Execution != (RustSuiteExecutionSummary{}) || stage.AttemptCleanup != "OwnershipLostCleanupComplete" {
			return errors.New("RustSuite ownership-loss Stage differs")
		}
		return nil
	case "IncompleteBeforeDurableRunnerEvidence":
		completionDiagnostic := rustSuiteHasDiagnostic(stage.Diagnostics, "runner-completion")
		outputDiagnostic, typedOutputDiagnostic := rustSuiteHasDiagnostic(stage.Diagnostics, "required-output"), false
		for _, diagnostic := range stage.Diagnostics {
			typedOutputDiagnostic = typedOutputDiagnostic || strings.HasPrefix(diagnostic.Field, "native-absent-") || strings.HasPrefix(diagnostic.Field, "publication-failed-")
		}
		if stage.Terminal != "Stopped" || completionDiagnostic == outputDiagnostic || outputDiagnostic && !typedOutputDiagnostic || stage.TimedOut || stage.Canceled || stage.ExitCode != -1 || stage.SourcePostimagePath != "" || stage.Preparation != nil || stage.Observation != nil || stage.MaterializationEvidence != nil || len(stage.Failures) != 0 || len(stage.Timeouts) != 0 || len(stage.SelectedPackages) != 0 || stage.Execution != (RustSuiteExecutionSummary{}) || stage.AttemptCleanup != "IncompleteAttemptCleanupComplete" {
			return errors.New("RustSuite incomplete Stage differs")
		}
		return nil
	case "CanceledReaped":
		if stage.Observation == nil || stage.Terminal != "Stopped" || stage.TimedOut || !stage.Canceled {
			return errors.New("RustSuite canceled Stage differs")
		}
	case "StartedReaped":
		if stage.Observation == nil || stage.Canceled || stage.TimedOut && stage.Terminal != "Stopped" || stage.Terminal == "Pass" && len(stage.Diagnostics) != 0 || stage.Terminal == "Stopped" && len(stage.Diagnostics) == 0 {
			return errors.New("RustSuite reaped Stage differs")
		}
	}
	if plan.Scope != "baseline" || stage.Stage != rustSuiteRoute(RustSuiteOptions{Tag: plan.Tag, Scope: plan.Scope}) {
		return errors.New("baseline process identity differs")
	}
	ownership, _, _, processErr := projectionAttemptOwnershipState(recordRoot, scratchRoot, stage.Stage, recoverTerminal, nil, stage.Observation)
	if processErr != nil || ownership != "Reaped" {
		return errors.Join(processErr, errors.New("RustSuite fixed process Terminal differs"))
	}
	completionFailure := listed("StartedReaped CanceledReaped", stage.ProcessState) && stage.Terminal != "Pass"
	missingPreparation := rustSuiteHasDiagnostic(stage.Diagnostics, "preparation")
	preparationValid := stage.Preparation == nil && completionFailure && missingPreparation || stage.Preparation != nil && validRustSuitePreparationSummary(plan, *stage.Preparation)
	if stage.AttemptCleanup != "ExactAttemptRootAbsent" || !preparationValid || stage.MaterializationEvidence == nil || !validRustSuiteMaterialization(plan, *stage.MaterializationEvidence, sourceSelection) || !rustSuiteValidCargoIdentity(stage.MaterializationEvidence.Current) || !validRustSuiteStageExecution(plan, stage) {
		return errors.New("RustSuite reaped Stage evidence differs")
	}
	return nil
}

func validRustSuiteDiagnostics(diagnostics []RustSuiteDiagnostic) bool {
	previous := ""
	for _, diagnostic := range diagnostics {
		if diagnostic.Field == "" || diagnostic.Expected == "" || diagnostic.Actual == "" || diagnostic.Field <= previous {
			return false
		}
		previous = diagnostic.Field
	}
	return true
}

func rustSuiteHasDiagnostic(diagnostics []RustSuiteDiagnostic, field string) bool {
	for _, diagnostic := range diagnostics {
		if diagnostic.Field == field {
			return true
		}
	}
	return false
}

func validRustSuiteMaterialization(plan RustSuitePlan, materialization RustSuiteMaterializationEvidence, expectedSource string) bool {
	stateMatches := materialization.Current.CargoVersion == plan.CargoVersion && materialization.Current.ToolchainRoot == plan.ToolchainRoot && materialization.Current.Target == plan.ToolchainTarget && materialization.Current.SourceSelection == expectedSource
	return stateMatches && validCargoMaterializationAction(materialization) && validCargoReuse(materialization)
}

func sortedUniqueRustSuiteOutcomes(values []string) bool {
	for index, value := range values {
		if value == "" || index > 0 && values[index-1] >= value {
			return false
		}
	}
	return true
}

func validRustSuitePreparationSummary(plan RustSuitePlan, preparation RustSuitePreparationSummary) bool {
	if !listed("Complete Fail", preparation.Terminal) || !sameStrings(preparation.CompileOnlyPackages, plan.CompileOnlyPackages) || !sameStrings(preparation.HelperBinaries, plan.HelperBinaries) || !validRustSuitePackages(preparation.SelectedPackages) || preparation.SelectedTestCount < 0 || preparation.BinaryBindingCount < 0 || !validRustSuiteDiagnostics(preparation.Diagnostics) {
		return false
	}
	if preparation.Terminal == "Fail" {
		return len(preparation.Diagnostics) != 0 && preparation.SelectedTestCount == 0 && preparation.BinaryBindingCount == 0 && len(preparation.SelectedPackages) == 0
	}
	maintenanceOnly := rustSuiteEmptySelection(plan) && len(preparation.SelectedPackages) == 0 && preparation.SelectedTestCount == 0
	if len(preparation.Diagnostics) != 0 || !maintenanceOnly && (preparation.BinaryBindingCount == 0 || len(preparation.SelectedPackages) == 0 || preparation.SelectedTestCount == 0) {
		return false
	}
	for _, name := range preparation.SelectedPackages {
		if sortedHas(plan.ExcludedPackages, name) {
			return false
		}
	}
	return true
}

func validRustSuiteStageExecution(plan RustSuitePlan, stage RustSuiteStageResult) bool {
	execution := stage.Execution
	if execution.CompletedTests < 0 || execution.FailureCount < 0 || execution.TimeoutCount < 0 || execution.SkippedTests < 0 || execution.FailureCount > execution.CompletedTests || execution.TimeoutCount > execution.CompletedTests-execution.FailureCount || execution.SkippedTests > execution.CompletedTests-execution.FailureCount-execution.TimeoutCount {
		return false
	}
	if len(stage.Failures) != min(execution.FailureCount, rustSuiteBaselineOutcomeLimit) || len(stage.Timeouts) != min(execution.TimeoutCount, rustSuiteBaselineOutcomeLimit-len(stage.Failures)) {
		return false
	}
	if stage.Preparation == nil {
		validTerminal := stage.ProcessState == "StartedReaped" && (stage.Terminal == "Stopped" || stage.Terminal == "Fail") || stage.ProcessState == "CanceledReaped" && stage.Terminal == "Stopped"
		return validTerminal && len(stage.Diagnostics) != 0 && len(stage.SelectedPackages) == 0 && execution == (RustSuiteExecutionSummary{})
	}
	preparation := *stage.Preparation
	if preparation.Terminal == "Fail" {
		validTerminal := stage.Terminal == "Fail" || stage.Terminal == "Stopped" && (stage.ProcessState == "CanceledReaped" || len(stage.Diagnostics) != 0)
		return validTerminal && len(stage.SelectedPackages) == 0 && execution.CompletedTests == 0
	}
	validPathTerminal := stage.Terminal == "Fail" || stage.ProcessState == "CanceledReaped" && stage.Terminal == "Stopped"
	maintenanceOnly := rustSuiteEmptySelection(plan) && len(stage.SelectedPackages) == 0
	if stage.SourcePostimagePath != "" && !validPathTerminal || !validRustSuitePackages(stage.SelectedPackages) || execution.CompletedTests > preparation.SelectedTestCount {
		return false
	}
	if !execution.Validated && (execution.CompletedTests != 0 || len(stage.Diagnostics) == 0) {
		return false
	}
	for _, name := range stage.SelectedPackages {
		if sortedHas(plan.ExcludedPackages, name) || !sortedHas(preparation.SelectedPackages, name) {
			return false
		}
	}
	if stage.ProcessState == "StartedReaped" && !stage.TimedOut && len(stage.Diagnostics) == 0 {
		if !execution.Validated || execution.CompletedTests != preparation.SelectedTestCount || !sameStrings(stage.SelectedPackages, preparation.SelectedPackages) || len(stage.SelectedPackages) == 0 && !maintenanceOnly {
			return false
		}
	}
	if stage.Terminal != "Pass" {
		return true
	}
	if !execution.Validated || execution.CompletedTests != preparation.SelectedTestCount || execution.CompletedTests == 0 && !maintenanceOnly || execution.FailureCount != len(stage.Failures) || execution.TimeoutCount != len(stage.Timeouts) {
		return false
	}
	return rustSuiteStageTerminal(plan, projectionResult{ExitCode: stage.ExitCode, SourceUnchanged: stage.Observation.SourceUnchanged && stage.SourcePostimagePath == "", FailureClass: stage.Observation.FailureClass}, stage.rustSuiteOutcomes()) == "Pass"
}

func sameStrings(left, right []string) bool {
	return strings.Join(left, "\x00") == strings.Join(right, "\x00")
}

func rustSuiteSameValue(left, right any) bool {
	leftData, leftErr := core.CanonicalJSON(left)
	rightData, rightErr := core.CanonicalJSON(right)
	return leftErr == nil && rightErr == nil && bytes.Equal(leftData, rightData)
}
