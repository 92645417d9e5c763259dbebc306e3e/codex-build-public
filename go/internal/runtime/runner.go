package runtime

import (
	"bufio"
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"slices"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"

	"codex-build/internal/assets"
	"codex-build/internal/core"
)

type RunnerExecOptions struct {
	endpointOptions
	ProjectWorkingRoot   string
	ProjectScratchRoot   string
	RuntimeTemporaryRoot string
	RecordRoot           string
	Source               string
	Release              string
	WorkingDirectory     string
	Arguments            []string
	TestSource           string
	TestNames            []string
	AttemptID            string
	CompleteSuite        bool
	Deadline             time.Duration
	Cancellation         <-chan os.Signal
	OnLive               func()
	consumer             runnerConsumer
	completeSnapshots    bool
	resumeReaped         bool
	replaceCurrent       bool
	resultFiles          []string
	sourceCommit         string
	onPhase              func(string) error
	selectionEnd         time.Time
	roundBudget          time.Duration
}

type RunnerExecResult struct {
	SchemaVersion            int
	RecordType               string
	Release                  string
	AttemptID                string
	Request                  runnerRequestBinding
	Preparation              *cargoPreparation           `json:",omitempty"`
	Observation              *processTerminalObservation `json:",omitempty"`
	GeneratedSnapshotUpdates []snapshotUpdate
	PreExecutionFailure      string `json:",omitempty"`
	DeadlineMilliseconds     int64
	Terminal, ProductEffect  string
	FailureDomain            string
	FailureClass             *string
	SourcePostimagePath      string
	FailureSummary           *RunnerFailureSummary `json:",omitempty"`
	ExecutionStarted         bool
}

type RunnerFailureSummary struct {
	Diagnostic, Reason       string
	StdoutBytes, StderrBytes int64
}

type runnerTestPreparation struct {
	Diagnostic, FailureDomain, Phase string
}

const runnerDiagnosticLimit = 2048

var runnerDiagnosticMarkers = []string{
	"SchemaPayloadTransportFailure",
	"SchemaPostimagesDiffer",
}

type runnerLogWriter struct {
	writer  io.Writer
	size    int64
	tail    []byte
	overlap []byte
	seen    map[string]bool
	err     error
}

func (writer *runnerLogWriter) Write(data []byte) (int, error) {
	written, err := writer.writer.Write(data)
	writer.size += int64(written)
	writer.observe(data[:written])
	if err != nil {
		writer.err = err
	}
	return written, err
}

func (writer *runnerLogWriter) observe(data []byte) {
	window := append(writer.overlap, data...)
	overlap := 0
	for _, marker := range runnerDiagnosticMarkers {
		overlap = max(overlap, len(marker)-1)
		position := bytes.Index(window, []byte(marker))
		if position >= 0 {
			if writer.seen == nil {
				writer.seen = map[string]bool{}
			}
			writer.seen[marker] = true
		}
	}
	writer.overlap = append(writer.overlap[:0], window[max(0, len(window)-overlap):]...)
	if len(data) >= runnerDiagnosticLimit {
		writer.tail = append(writer.tail[:0], data[len(data)-runnerDiagnosticLimit:]...)
	} else {
		writer.tail = append(writer.tail, data...)
		writer.tail = writer.tail[max(0, len(writer.tail)-runnerDiagnosticLimit):]
	}
}

func boundedRunnerDiagnostic(value string) string {
	return strings.ToValidUTF8(value[:min(len(value), runnerDiagnosticLimit)], "?")
}

func summarizeRunnerLogs(stdout, stderr *runnerLogWriter) *RunnerFailureSummary {
	summary := &RunnerFailureSummary{StdoutBytes: stdout.size, StderrBytes: stderr.size}
	markers := []string{}
	for _, marker := range runnerDiagnosticMarkers {
		if stderr.seen[marker] || stdout.seen[marker] {
			markers = append(markers, marker)
		}
	}
	for _, err := range []error{stdout.err, stderr.err} {
		if err != nil {
			markers = append(markers, err.Error())
		}
	}
	tail := stderr.tail
	if len(tail) == 0 {
		tail = stdout.tail
	}
	prefix := boundedRunnerDiagnostic(strings.Join(markers, "\n"))
	remaining := max(0, runnerDiagnosticLimit-len(prefix)-1)
	summary.Diagnostic = boundedRunnerDiagnostic(strings.TrimSpace(prefix + "\n" + string(tail[max(0, len(tail)-remaining):])))
	return summary
}

func runnerLogPostimage(path string, inspect bool, window deadline) (*runnerLogWriter, error) {
	stream := &runnerLogWriter{size: -1}
	file, err := os.OpenFile(path, os.O_RDONLY|syscall.O_NOFOLLOW, 0)
	if err != nil {
		return stream, err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil || !info.Mode().IsRegular() {
		return stream, errors.Join(err, errors.New("runner log is not regular"))
	}
	stream.size = info.Size()
	if inspect {
		buffer := make([]byte, 32<<10)
		for {
			if err := window.check(); err != nil {
				return stream, err
			}
			count, err := file.Read(buffer)
			stream.observe(buffer[:count])
			if errors.Is(err, io.EOF) {
				break
			}
			if err != nil {
				return stream, err
			}
		}
	}
	after, err := os.Lstat(path)
	if err != nil || !os.SameFile(info, after) || info.Size() != after.Size() || !info.ModTime().Equal(after.ModTime()) {
		return stream, errors.Join(err, errors.New("runner log changed after execution"))
	}
	return stream, nil
}

func validRunnerFailureSummary(result RunnerExecResult) bool {
	if result.Terminal == "Pass" {
		return result.FailureSummary == nil
	}
	summary := result.FailureSummary
	return summary != nil && validName(summary.Reason, "-") && len(summary.Reason) <= 64 && len(summary.Diagnostic) <= runnerDiagnosticLimit && summary.StdoutBytes >= -1 && summary.StderrBytes >= -1
}

func runnerFailureDiagnostic(result RunnerExecResult) string {
	if result.FailureSummary != nil {
		return result.FailureSummary.Diagnostic
	}
	return boundedRunnerDiagnostic(result.PreExecutionFailure)
}

type runnerRequestBinding struct {
	ProjectWorkingRoot, ProjectScratchRoot, RuntimeTemporaryRoot, RecordRoot string
	Source, Release, WorkingDirectory, AttemptID                             string
	Arguments                                                                []string
	CompleteSuite                                                            bool
	DeadlineMilliseconds                                                     int64
}

func validGitCommit(value string) bool {
	return len(value) == 40 && validName(value, "") && value == strings.ToLower(value)
}

type sourceToolchainProjection struct {
	sourceProjection
	Toolchain ToolchainProjection
}

type runtimeAsset struct {
	name string
	data []byte
	mode os.FileMode
}

type runnerConsumer struct {
	rustSuite            bool
	environment          []string
	assets               []runtimeAsset
	ephemeralSourcePaths []string
	networkDisabled      bool
}

type snapshotTarget struct {
	Path                string
	Kind                string
	Current             string
	ExistingInlineSpans []snapshotTargetSpan
}

type snapshotTargetSpan struct {
	Start    int
	End      int
	Line     int
	Preimage string
}

type snapshotUpdate struct {
	Target               snapshotTarget
	Generated            string
	GeneratedInlineSpans []snapshotUpdateSpan
}

type snapshotUpdateSpan struct {
	Start     int
	End       int
	Line      int
	Postimage string
}

func repositoryTestArguments(arguments []string) bool {
	if len(arguments) < 2 || arguments[0] != "/toolchain/bin/just" || arguments[1] != "test" {
		return false
	}
	for _, selector := range arguments[2:] {
		if selector == "" {
			return false
		}
	}
	return true
}

func runnerTestConcurrency(arguments []string) (int, error) {
	if !repositoryTestArguments(arguments) {
		return 0, nil
	}
	concurrency, selected := core.DefaultNextestConcurrency, false
	for index := 2; index < len(arguments); index++ {
		argument, value := arguments[index], ""
		if argument == "--" {
			break
		}
		switch {
		case listed("--test-threads --jobs -j", argument):
			index++
			if index >= len(arguments) {
				return 0, errors.New("runner Nextest concurrency value is absent")
			}
			value = arguments[index]
		case strings.HasPrefix(argument, "--test-threads="):
			value = strings.TrimPrefix(argument, "--test-threads=")
		case strings.HasPrefix(argument, "--jobs="):
			value = strings.TrimPrefix(argument, "--jobs=")
		case strings.HasPrefix(argument, "-j"):
			value = strings.TrimPrefix(strings.TrimPrefix(argument, "-j"), "=")
		default:
			continue
		}
		parsed, err := strconv.Atoi(value)
		if selected || err != nil || parsed <= 0 || parsed > core.DefaultNextestConcurrency {
			return 0, fmt.Errorf("runner Nextest concurrency requires one positive integer no greater than %d", core.DefaultNextestConcurrency)
		}
		concurrency, selected = parsed, true
	}
	return concurrency, nil
}

func validateRunnerArguments(arguments []string, completeSuite bool) error {
	if len(arguments) == 0 || arguments[0] == "" {
		return errors.New("runner needs argv")
	}
	base := filepath.Base(arguments[0])
	if arguments[0] == "/toolchain/bin/just" {
		if !repositoryTestArguments(arguments) {
			return errors.New("unsupported runner just command")
		}
		if completeSuite != (len(arguments) == 2) {
			return errors.New("runner complete-suite admission differs")
		}
		return nil
	}
	if base == "just" {
		return errors.New("unsupported runner just path")
	}
	if listed("bash cargo cargo-nextest dash env rustc rustdoc sh zsh", base) {
		return errors.New("unsupported runner executable")
	}
	if completeSuite {
		return errors.New("runner complete-suite admission differs")
	}
	return nil
}

func validateRunnerSelection(options RunnerExecOptions) error {
	if options.TestSource == "" && len(options.TestNames) == 0 {
		return validateRunnerArguments(options.Arguments, options.CompleteSuite)
	}
	if len(options.Arguments) != 0 || options.CompleteSuite || options.WorkingDirectory != "codex-rs" || !filepath.IsLocal(options.TestSource) || filepath.ToSlash(filepath.Clean(options.TestSource)) != options.TestSource || !strings.HasPrefix(options.TestSource, "codex-rs/") || !strings.HasSuffix(options.TestSource, ".rs") || strings.ContainsAny(options.TestSource, "\\\x00\r\n") || len(options.TestNames) == 0 {
		return errors.New("exact tests require a source-relative Cargo target root and native test names, codex-rs workdir, and no argv or complete-suite")
	}
	for _, name := range options.TestNames {
		if name == "" || strings.HasPrefix(name, "-") || strings.ContainsAny(name, "\x00\r\n") {
			return errors.New("exact test name must be nonempty and contain no option prefix or control separator")
		}
	}
	return nil
}

func bindRunnerTestSelection(options *RunnerExecOptions, toolchain ToolchainProjection) error {
	if options.TestSource == "" {
		return nil
	}
	window := newDeadline(options.Deadline, options.Cancellation)
	options.selectionEnd = window.end
	path := filepath.Join(options.Source, options.TestSource)
	if err := noFollowPath("test target source", path); err != nil {
		return err
	}
	if info, err := os.Lstat(path); err != nil || !info.Mode().IsRegular() {
		return errors.Join(err, errors.New("test target source must be a regular file"))
	}
	lock, err := cargoLock(options.ProjectScratchRoot)
	if err != nil {
		return preflightHelperError{err}
	}
	defer releaseCargoLock(lock)
	if err := ensureDirectoryNoFollow(cacheProjection(options.ProjectWorkingRoot).CargoHome, 0o700); err != nil {
		return preflightHelperError{err}
	}
	arguments := []string{options.Source, filepath.Join(toolchain.Path, "rust", "bin", "cargo"), filepath.Join(options.ProjectWorkingRoot, "build", "cache", "cargo-home"), options.TestSource}
	arguments = append(arguments, options.TestNames...)
	data, err := runNativeWithin(options.RuntimeTemporaryRoot, "runner-test-selection", window.remaining(), options.Cancellation, arguments...)
	if err != nil {
		return preflightHelperError{err}
	}
	var selection struct{ Arguments, TargetRoots []string }
	if err := json.Unmarshal(data, &selection); err != nil {
		return preflightHelperError{err}
	}
	if len(selection.Arguments) == 0 {
		return fmt.Errorf("test source %s is not an enabled Cargo target root; metadata candidates: %v", options.TestSource, selection.TargetRoots)
	}
	if err := validateRunnerArguments(selection.Arguments, false); err != nil {
		return preflightHelperError{err}
	}
	options.Arguments = selection.Arguments
	options.TestSource, options.TestNames = "", nil
	return nil
}

type snapshotOutputError struct {
	Path string
	error
}

func (err snapshotOutputError) Unwrap() error { return err.error }

type projectionRequest struct {
	SkillRoot            string
	CodexHome            string
	ProjectWorkingRoot   string
	ProjectScratchRoot   string
	RuntimeTemporaryRoot string
	RecordRoot           string
	Source               string
	Release              string
	WorkingDirectory     string
	Arguments            []string
	WritableSourceRoot   string
	AttemptRoot          string
	OutputRoot           string
	ResultFiles          []string
	CaptureSnapshots     bool
	SourceCommit         string
	CargoStage           string
	Consumer             runnerConsumer
	LockHeld             bool
	Deadline             time.Duration
	Cancellation         <-chan os.Signal
	OnLive               func()
	AttemptIdentity      string
	OnPhase              func(string) error
	RoundBudget          time.Duration
	continueSuite        bool
	suiteRound           int
	roundEnd             time.Time
	PreparedCargo        *preparedProjectionCargo
	RetainAttempt        bool
	ResumeReaped         bool
	RunnerRequest        *runnerRequestBinding
	PublishTerminal      func(projectionResult, error) error
	window               deadline
}

type projectionFile struct {
	Path string
}

type preparedProjectionCargo struct {
	Cache       CacheProjection
	Preparation cargoPreparation
}

type projectionResult struct {
	AttemptID                string
	Reaped                   bool
	ExecutionStarted         bool
	AttemptCleaned           bool
	ProcessState             string
	AttemptCleanup           string
	ExitCode                 int
	TimedOut                 bool
	Canceled                 bool
	DurationMilliseconds     int64
	Files                    map[string]projectionFile
	NativeAbsentOutputs      []string
	FailedOutputs            []string
	GeneratedSnapshotUpdates []snapshotUpdate
	SourceUnchanged          bool
	FailureDomain            string
	FailureClass             *string
	SourcePostimagePath      string
	LogsTruncated            bool
	Materialization          RustSuiteMaterializationEvidence
	Added                    bool
	OwnedReap                bool
	Preparation              *cargoPreparation
	Observation              *processTerminalObservation
	LogSummary               *RunnerFailureSummary
}

type processTerminalObservation struct {
	ExecutionStarted     bool
	ExitCode             int
	TimedOut, Canceled   bool
	DurationMilliseconds int64
	LogsTruncated        bool
	SourceUnchanged      bool
	FailureClass         *string `json:",omitempty"`
}

type processGroupRecord struct {
	SchemaVersion, ProcessGroupID int
	RecordType, AttemptID         string
	Transition                    string
	Preparation                   *cargoPreparation           `json:",omitempty"`
	Request                       *runnerRequestBinding       `json:",omitempty"`
	Observation                   *processTerminalObservation `json:",omitempty"`
}

type boundedWriter struct {
	writer    io.Writer
	remaining int64
	truncated bool
}

func executeRunner(options RunnerExecOptions) (RunnerExecResult, error) {
	failure := RunnerExecResult{SchemaVersion: core.SchemaVersion, RecordType: "RunnerExecResult", Release: options.Release, ProductEffect: "None", FailureDomain: "Input", AttemptID: options.AttemptID, Request: runnerRequestFor(options), GeneratedSnapshotUpdates: []snapshotUpdate{}, DeadlineMilliseconds: options.Deadline.Milliseconds(), Terminal: "Blocked"}
	if err := validateRunnerExecOptions(options); err != nil {
		failure.FailureDomain = runnerFailureDomain(err)
		failure.PreExecutionFailure = err.Error()
		return failure, err
	}
	if _, err := runnerSourceProjection(options.Source, options.ProjectWorkingRoot, options.ProjectScratchRoot, options.RuntimeTemporaryRoot, options.Release); err != nil {
		failure.FailureDomain = runnerFailureDomain(err)
		failure.PreExecutionFailure = err.Error()
		return failure, err
	}
	lock, err := cargoLock(options.ProjectScratchRoot)
	if err != nil {
		failure.FailureDomain = failureDomainHelper
		failure.PreExecutionFailure = err.Error()
		return failure, err
	}
	defer releaseCargoLock(lock)
	if options.replaceCurrent {
		if err := replaceRunnerEvidence(options); err != nil {
			return failure, err
		}
	} else if options.consumer.rustSuite && options.AttemptID == "full" {
		binding := runnerRequestFor(options)
		ownership, _, _, err := projectionAttemptOwnershipState(options.RecordRoot, options.ProjectScratchRoot, options.AttemptID, true, &binding, nil)
		if err != nil || ownership == "Absent" {
			return failure, errors.Join(err, errors.New("resume requires the current full execution; select a fresh check explicitly"))
		}
	}
	consumer := options.consumer
	resultFiles := append([]string{}, options.resultFiles...)
	if repositoryTestArguments(options.Arguments) {
		concurrency, err := runnerTestConcurrency(options.Arguments)
		if err != nil {
			return failure, err
		}
		consumer.environment = append(append([]string{}, consumer.environment...), "CARGO_NET_OFFLINE=true", "NEXTEST_TEST_THREADS="+strconv.Itoa(concurrency))
		consumer.assets = append(consumer.assets, runtimeAsset{name: "runner-binaries.py", data: assets.RunnerBinaries(), mode: 0o600})
		resultFiles = append(resultFiles, "runner-test-preparation.json")
	}
	captureSnapshots := options.completeSnapshots || repositoryTestArguments(options.Arguments)
	if captureSnapshots {
		update := false
		for index, value := range consumer.environment {
			if strings.HasPrefix(value, "INSTA_UPDATE=") {
				consumer.environment[index] = "INSTA_UPDATE=always"
				update = true
			}
		}
		if !update {
			consumer.environment = append(consumer.environment, "INSTA_UPDATE=always")
		}
		consumer.environment = append(consumer.environment, "INSTA_FORCE_PASS=1")
		hasReleaseRoll := false
		for _, asset := range consumer.assets {
			hasReleaseRoll = hasReleaseRoll || asset.name == "release-roll.py"
		}
		if !hasReleaseRoll {
			consumer.assets = append(consumer.assets, runtimeAsset{name: "release-roll.py", data: assets.ReleaseRoll(), mode: 0o700})
		}
		consumer.assets = append(consumer.assets, runtimeAsset{name: "complete-snapshots", data: []byte{}, mode: 0o600})
	}
	var published *RunnerExecResult
	binding := runnerRequestFor(options)
	request := projectionRequest{
		SkillRoot: options.SkillRoot, ProjectWorkingRoot: options.ProjectWorkingRoot,
		ProjectScratchRoot: options.ProjectScratchRoot, RuntimeTemporaryRoot: options.RuntimeTemporaryRoot,
		RecordRoot: options.RecordRoot, Source: options.Source, Release: options.Release,
		WorkingDirectory: options.WorkingDirectory, Arguments: options.Arguments,
		AttemptRoot: projectionAttemptRoot(options.ProjectWorkingRoot, options.AttemptID), OutputRoot: runnerOutputRoot(options),
		Consumer: consumer, LockHeld: true, ResultFiles: resultFiles,
		CaptureSnapshots: captureSnapshots,
		SourceCommit:     options.sourceCommit, Deadline: options.Deadline, Cancellation: options.Cancellation, OnLive: options.OnLive, OnPhase: options.onPhase,
		AttemptIdentity: options.AttemptID, RetainAttempt: true, ResumeReaped: options.resumeReaped, RunnerRequest: &binding, window: deadline{options.selectionEnd, options.Cancellation},
		RoundBudget: options.roundBudget,
	}
	finalize := func(projection projectionResult, runErr error) error {
		terminal := runnerExecResult(projection, options, runErr)
		outputErr := bindRunnerTerminalOutput(&terminal, projection)
		if err := publishRunnerTerminal(options, terminal); err != nil {
			return err
		}
		published = &terminal
		return outputErr
	}
	request.PublishTerminal = finalize
	result, err := runProjection(request)
	result.AttemptID = failure.AttemptID
	if published != nil {
		return *published, err
	}
	return runnerExecResult(result, options, err), err
}

func runnerExecResult(result projectionResult, options RunnerExecOptions, runErr error) RunnerExecResult {
	terminal := "Pass"
	failureDomain := result.FailureDomain
	if failureDomain == "" {
		failureDomain = failureDomainNone
	}
	preExecutionFailure := ""
	if listed("OwnershipLostBeforeDurableRunnerEvidence IncompleteBeforeDurableRunnerEvidence", result.ProcessState) {
		terminal = "Stopped"
		failureDomain = failureDomainHelper
	} else if result.ExecutionStarted && !result.Reaped {
		terminal = "Blocked"
		failureDomain = failureDomainHelper
	} else if !result.ExecutionStarted && result.Observation == nil {
		terminal = "Blocked"
		if failureDomain == failureDomainNone {
			failureDomain = runnerFailureDomain(runErr)
		}
		if runErr != nil {
			preExecutionFailure = runErr.Error()
		}
	} else if failureDomain == failureDomainHelper {
		if runnerOwnershipLost(result) || result.TimedOut || result.Canceled {
			terminal = "Stopped"
			failureDomain = failureDomainHelper
		} else {
			terminal = "Fail"
			failureDomain = failureDomainHelper
		}
	} else if result.Canceled || runnerOwnershipLost(result) {
		terminal = "Stopped"
		if result.Canceled {
			failureDomain = failureDomainProduct
		} else {
			failureDomain = failureDomainHelper
		}
	} else if runErr != nil || result.ExitCode != 0 || result.TimedOut || !result.SourceUnchanged || result.FailureClass != nil {
		terminal = "Fail"
		failureDomain = failureDomainProduct
	}
	updates := result.GeneratedSnapshotUpdates
	if updates == nil {
		updates = []snapshotUpdate{}
	}
	return RunnerExecResult{
		SchemaVersion: core.SchemaVersion, RecordType: "RunnerExecResult", Release: options.Release, ProductEffect: "None",
		AttemptID: result.AttemptID, Request: runnerRequestFor(options), Preparation: result.Preparation, Observation: result.Observation, GeneratedSnapshotUpdates: updates, PreExecutionFailure: preExecutionFailure,
		DeadlineMilliseconds: options.Deadline.Milliseconds(), Terminal: terminal, FailureDomain: failureDomain, FailureClass: result.FailureClass, SourcePostimagePath: result.SourcePostimagePath, ExecutionStarted: result.ExecutionStarted,
	}
}

func runnerRequestFor(options RunnerExecOptions) runnerRequestBinding {
	return runnerRequestBinding{
		ProjectWorkingRoot: options.ProjectWorkingRoot, ProjectScratchRoot: options.ProjectScratchRoot,
		RuntimeTemporaryRoot: options.RuntimeTemporaryRoot, RecordRoot: options.RecordRoot,
		Source: options.Source, Release: options.Release, WorkingDirectory: options.WorkingDirectory, AttemptID: options.AttemptID,
		Arguments: options.Arguments, CompleteSuite: options.CompleteSuite, DeadlineMilliseconds: options.Deadline.Milliseconds(),
	}
}

func runnerOwnershipLost(result projectionResult) bool { return result.Reaped && !result.OwnedReap }

func setProjectionProcessState(result *projectionResult) {
	if !result.Reaped {
		return
	}
	if runnerOwnershipLost(*result) {
		result.ProcessState = "OwnershipLostBeforeDurableRunnerEvidence"
	} else if result.Canceled {
		result.ProcessState = "CanceledReaped"
	} else {
		result.ProcessState = "StartedReaped"
	}
}

func projectionHasProductFailure(result projectionResult) bool {
	if result.FailureClass != nil {
		return *result.FailureClass == "ControllerCancellation" || *result.FailureClass == "SourcePostimage"
	}
	return result.FailureDomain == failureDomainProduct || result.ExitCode != 0 || result.TimedOut || result.Canceled || result.ExecutionStarted && !result.SourceUnchanged
}

func setProjectionHelperFailure(result *projectionResult, class string) {
	if class != "OutputPublication" && projectionHasProductFailure(*result) {
		return
	}
	if result.FailureClass == nil || class == "OutputPublication" && !result.Canceled {
		result.FailureClass = &class
	}
	result.FailureDomain = failureDomainHelper
}

func setProjectionTimeout(result *projectionResult) {
	result.TimedOut, result.FailureDomain = true, failureDomainHelper
}

func setProjectionCanceled(result *projectionResult) {
	class := "ControllerCancellation"
	result.Canceled, result.FailureClass, result.FailureDomain = true, &class, failureDomainProduct
}

func setProjectionFailure(result *projectionResult, class string, err error) {
	if errors.Is(err, errNativeTimeout) {
		setProjectionTimeout(result)
	} else if err != nil {
		setProjectionHelperFailure(result, class)
	}
}

func bindSccacheStatistics(result *projectionResult, preparation *cargoPreparation, window deadline) error {
	statistics, exists := result.Files["sccache-stats.json"]
	if !exists && (result.TimedOut || result.Canceled) {
		return nil
	}
	if !exists {
		return errors.New("sccache statistics output is absent")
	}
	data, err := readRegularNoFollow(statistics.Path, window)
	if err != nil {
		return errors.Join(err, errors.New("sccache statistics output differs"))
	}
	return applySccacheReuse(preparation, data)
}

func validateRunnerTerminal(options RunnerExecOptions, result RunnerExecResult, recoverProcess bool) error {
	request := runnerRequestFor(options)
	ownership, preparation, observation, processErr := projectionAttemptOwnershipState(options.RecordRoot, options.ProjectScratchRoot, options.AttemptID, recoverProcess, &request, result.Observation)
	reaped := ownership == "Reaped" && reflect.DeepEqual(result.Observation, observation)
	incomplete := ownership == "Incomplete" && result.Observation == nil && !result.ExecutionStarted && result.Terminal == "Stopped" && result.FailureDomain == failureDomainHelper && result.FailureClass != nil && listed("TerminalPublication OutputPublication", *result.FailureClass)
	validSourceCommit := preparation != nil && (preparation.SourceCommit == "" || validGitCommit(preparation.SourceCommit)) && (options.sourceCommit == "" || preparation.SourceCommit == options.sourceCommit)
	if result.SchemaVersion != core.SchemaVersion || result.RecordType != "RunnerExecResult" || result.Release != options.Release || result.AttemptID != options.AttemptID || result.DeadlineMilliseconds != options.Deadline.Milliseconds() || !listed("Pass Stopped Fail", result.Terminal) || result.ProductEffect != "None" || !listed("None Product Helper", result.FailureDomain) || !result.ExecutionStarted && (result.FailureDomain != failureDomainHelper || !listed("Fail Stopped", result.Terminal)) || result.PreExecutionFailure != "" || !reflect.DeepEqual(result.Request, runnerRequestFor(options)) || processErr != nil || !validRunnerFailureSummary(result) || result.Preparation == nil || !reflect.DeepEqual(result.Preparation, preparation) || !reaped && !incomplete || !validSourceCommit || !validSourcePostimageClassification(result.FailureClass, result.SourcePostimagePath) {
		return errors.New("cached runner terminal differs")
	}
	switch result.Terminal {
	case "Pass":
		if result.FailureDomain != failureDomainNone || result.FailureClass != nil {
			return errors.New("cached runner pass terminal differs")
		}
	case "Stopped":
		if result.FailureDomain == failureDomainNone {
			return errors.New("cached runner stopped terminal differs")
		}
	case "Fail":
		if result.FailureDomain == failureDomainNone {
			return errors.New("cached runner failure terminal differs")
		}
	}
	return nil
}

func bindRunnerTerminalOutput(result *RunnerExecResult, projection projectionResult) error {
	streams := []*runnerLogWriter{{size: -1}, {size: -1}}
	var outputErr error
	window := newDeadline(core.GracePeriod(), nil)
	for index, name := range []string{"runner.stdout.log", "runner.stderr.log"} {
		file, present := projection.Files[name]
		if !present {
			outputErr = errors.Join(outputErr, errors.New("runner output was not published: "+name))
			continue
		}
		var err error
		streams[index], err = runnerLogPostimage(file.Path, projection.LogSummary == nil && result.Terminal != "Pass", window)
		outputErr = errors.Join(outputErr, err)
	}
	summary := summarizeRunnerLogs(streams[0], streams[1])
	if projection.LogSummary != nil {
		if summary.StdoutBytes != projection.LogSummary.StdoutBytes || summary.StderrBytes != projection.LogSummary.StderrBytes {
			outputErr = errors.Join(outputErr, errors.New("runner log byte count differs"))
		}
		summary.Diagnostic, summary.Reason = projection.LogSummary.Diagnostic, projection.LogSummary.Reason
	}
	if result.Observation != nil && !result.Observation.TimedOut && !result.Observation.Canceled {
		statistics, present := projection.Files["sccache-stats.json"]
		if !present {
			outputErr = errors.Join(outputErr, errors.New("runner statistics output was not published"))
		} else if _, err := readRegularNoFollow(statistics.Path); err != nil {
			outputErr = errors.Join(outputErr, err, errors.New("runner statistics output is unavailable"))
		}
	}
	if repositoryTestArguments(result.Request.Arguments) && result.ExecutionStarted && result.Observation != nil && !result.Observation.TimedOut && !result.Observation.Canceled {
		file, present := projection.Files["runner-test-preparation.json"]
		info, statErr := os.Lstat(file.Path)
		preparation := runnerTestPreparation{}
		preparationErr := statErr
		if present && statErr == nil && info.Mode().IsRegular() && info.Size() <= 16<<10 {
			_, preparationErr = readCanonicalValue(file.Path, &preparation)
		} else {
			preparationErr = errors.Join(statErr, errors.New("repository test preparation output is absent or invalid"))
		}
		valid := preparationErr == nil && listed("Metadata SelectedTestBuild BinaryContract BinaryBuild BinaryBindings TestExecution", preparation.Phase) && listed("None Product Helper", preparation.FailureDomain) && len(preparation.Diagnostic) <= runnerDiagnosticLimit
		ready := preparation.FailureDomain == failureDomainNone && preparation.Phase == "TestExecution" && preparation.Diagnostic == ""
		failed := preparation.FailureDomain != failureDomainNone && preparation.Diagnostic != "" && result.Observation.ExitCode != 0
		if !valid || !ready && !failed {
			outputErr = errors.Join(outputErr, preparationErr, errors.New("repository test preparation terminal differs"))
		} else if failed {
			summary.Diagnostic = boundedRunnerDiagnostic("RunnerTestPreparation/" + preparation.Phase + ": " + preparation.Diagnostic + "\n" + summary.Diagnostic)
			if preparation.FailureDomain == failureDomainHelper && result.Observation.SourceUnchanged && result.FailureClass == nil {
				class := "TestPreparation"
				result.FailureDomain, result.FailureClass, result.Terminal = failureDomainHelper, &class, "Fail"
			}
		}
	}
	if outputErr != nil {
		setRunnerOutputFailure(result)
		summary.Diagnostic = boundedRunnerDiagnostic(outputErr.Error() + "\n" + summary.Diagnostic)
	}
	if result.Terminal != "Pass" {
		summary.Reason = "CommandFailure"
		if result.FailureDomain == failureDomainHelper {
			summary.Reason = "HelperFailure"
		}
		if result.FailureClass != nil {
			summary.Reason = *result.FailureClass
		}
		if result.Observation != nil && result.Observation.TimedOut {
			summary.Reason = "TimedOut"
		}
		result.FailureSummary = summary
	}
	return outputErr
}

func setRunnerOutputFailure(result *RunnerExecResult) {
	class := "OutputPublication"
	if result.Terminal != "Stopped" {
		result.Terminal = "Fail"
	}
	result.FailureDomain, result.FailureClass = failureDomainHelper, &class
}

func projectionTemporaryRoot(workingRoot string) string {
	return filepath.Join(workingRoot, "build", "tmp")
}

var errBuildTemporaryHeadroom = errors.New("build/tmp headroom")

func requireBuildTemporaryHeadroom(workingRoot string) error {
	var volume syscall.Statfs_t
	if err := syscall.Statfs(projectionTemporaryRoot(workingRoot), &volume); err != nil {
		return fmt.Errorf("%w: statfs: %w", errBuildTemporaryHeadroom, err)
	}
	free := volume.Bavail * uint64(volume.Bsize)
	if free <= 100<<30 {
		return fmt.Errorf("%w: free bytes=%d; require more than 100 GiB", errBuildTemporaryHeadroom, free)
	}
	return nil
}

func projectionAttemptRoot(workingRoot, attemptID string) string {
	return filepath.Join(projectionTemporaryRoot(workingRoot), "runner", attemptID)
}

func projectionOutputRoot(workingRoot, attemptID string) string {
	return filepath.Join(projectionTemporaryRoot(workingRoot), "runner-output", attemptID)
}

func projectionPublishedOutputRoot(workingRoot, attemptID string) string {
	return filepath.Join(projectionTemporaryRoot(workingRoot), "published-output", attemptID)
}

func projectionRuntimeRoot(runtimeRoot, attemptID string) string {
	return filepath.Join(runtimeRoot, "runner-runtime", attemptID)
}

func projectionScratchRoots(attemptRoot, runtimeRoot string) []string {
	attemptID := filepath.Base(attemptRoot)
	return []string{attemptRoot, projectionRuntimeRoot(runtimeRoot, attemptID)}
}

func runnerOutputRoot(options RunnerExecOptions) string {
	return projectionOutputRoot(options.ProjectWorkingRoot, options.AttemptID)
}

func replaceRunnerEvidence(options RunnerExecOptions) error {
	directory := filepath.Join(options.RecordRoot, "process", options.AttemptID)
	var prior processGroupRecord
	var err error
	for _, suffix := range []string{"", ".pending", ".stage"} {
		_, err = readCanonicalValue(filepath.Join(directory, "add.json"+suffix), &prior)
		if !os.IsNotExist(err) {
			break
		}
	}
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	if _, _, _, err := projectionAttemptOwnershipState(options.RecordRoot, options.ProjectScratchRoot, options.AttemptID, false, prior.Request, nil); err != nil {
		return err
	}
	if err := removeProjectionScratch(projectionAttemptRoot(options.ProjectWorkingRoot, options.AttemptID), options.RuntimeTemporaryRoot, newDeadline(core.CleanupDeadline(), nil)); err != nil {
		return err
	}
	path := runnerTerminalPath(options)
	return removeOwnedPaths(path, path+".pending", path+".stage", runnerOutputRoot(options), directory)
}

func consumeRunnerTerminal(options RunnerExecOptions) (RunnerExecResult, error) {
	result := RunnerExecResult{}
	if _, err := readCanonicalValue(runnerTerminalPath(options), &result); err != nil {
		return RunnerExecResult{}, err
	}
	lock, err := cargoLock(options.ProjectScratchRoot)
	if err != nil {
		return RunnerExecResult{}, err
	}
	defer releaseCargoLock(lock)
	if err := publishRunnerTerminal(options, result); err != nil {
		return RunnerExecResult{}, err
	}
	return result, nil
}

func publishRunnerTerminal(options RunnerExecOptions, result RunnerExecResult) error {
	if err := validateRunnerTerminal(options, result, true); err != nil {
		return err
	}
	path := runnerTerminalPath(options)
	data := runnerTerminalData(result)
	attemptRoot := projectionAttemptRoot(options.ProjectWorkingRoot, options.AttemptID)
	if err := ensureDirectoryNoFollow(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	if current, err := readRegularNoFollow(path); err == nil {
		if !bytes.Equal(current, data) {
			return errors.New("published terminal differs")
		}
	} else if !os.IsNotExist(err) {
		return err
	} else {
		request := runnerRequestFor(options)
		if ownership, _, _, err := projectionAttemptOwnershipState(options.RecordRoot, options.ProjectScratchRoot, options.AttemptID, true, &request, result.Observation); err != nil || !listed("Reaped Incomplete", ownership) {
			return errors.Join(err, errors.New("fixed process Terminal is absent"))
		}
	}
	if err := publishNoReplace(path, data, 0o600); err != nil {
		return err
	}
	if err := removeProjectionScratch(attemptRoot, options.RuntimeTemporaryRoot, newDeadline(core.StageDeadline(), nil)); err != nil {
		return err
	}
	if err := projectionAttemptAbsent(attemptRoot, options.RuntimeTemporaryRoot); err != nil {
		return err
	}
	return equalRecord(path, data)
}

func publishRustSuiteStageAfterAttemptCleanup(path string, data []byte, attemptRoot, runtimeRoot, recordRoot, scratchRoot string, observation *processTerminalObservation) error {
	directory, pending := filepath.Dir(path), path+".pending"
	if err := ensureDirectoryNoFollow(directory, 0o700); err != nil {
		return err
	}
	if current, err := readRegularNoFollow(path); err == nil {
		if !bytes.Equal(current, data) {
			return errors.New("published terminal differs")
		}
		return projectionAttemptAbsent(attemptRoot, runtimeRoot)
	} else if !os.IsNotExist(err) {
		return err
	}
	wantOwnership := "Reaped"
	if observation == nil {
		wantOwnership = "Incomplete"
	}
	if ownership, _, _, err := projectionAttemptOwnershipState(recordRoot, scratchRoot, filepath.Base(attemptRoot), true, nil, observation); err != nil || ownership != wantOwnership {
		return errors.Join(err, errors.New("fixed process Terminal is absent"))
	}
	if err := publishNoReplace(pending, data, 0o600); err != nil {
		return err
	}
	if err := removeProjectionScratch(attemptRoot, runtimeRoot, newDeadline(core.GracePeriod(), nil)); err != nil {
		return err
	}
	if err := projectionAttemptAbsent(attemptRoot, runtimeRoot); err != nil {
		return err
	}
	if err := os.Rename(pending, path); err != nil {
		return err
	}
	return errors.Join(syncDirectory(directory), equalRecord(path, data))
}

func validSourcePostimageClassification(class *string, path string) bool {
	isSourcePostimage := class != nil && *class == "SourcePostimage"
	return isSourcePostimage == (path != "") && (path == "" || filepath.IsLocal(path) && filepath.Clean(path) == path && filepath.ToSlash(path) == path && !strings.ContainsAny(path, "\\\x00\r\n"))
}

func runnerFailureDomain(err error) string {
	var helper preflightHelperError
	if errors.As(err, &helper) {
		return "Helper"
	}
	return "Input"
}

func validateRunnerExecOptions(options RunnerExecOptions) error {
	if !validName(options.AttemptID, "-._") {
		return errors.New("runner attempt-id differs")
	}
	if options.Release == "" || !validName(options.Release, "-._") {
		return errors.New("invalid runner release")
	}
	if !validWorkingDirectory(options.WorkingDirectory) {
		return errors.New("invalid runner workdir")
	}
	if err := validateRunnerSelection(options); err != nil {
		return err
	}
	if _, err := runnerTestConcurrency(options.Arguments); err != nil {
		return err
	}
	if !validRunnerDeadline(options.Deadline, options.consumer.rustSuite) {
		return errors.New("invalid runner deadline")
	}
	if err := validateProjectionRoots(options.SkillRoot, options.ProjectWorkingRoot, options.ProjectScratchRoot, options.RuntimeTemporaryRoot, options.RecordRoot, options.Source); err != nil {
		return err
	}
	if options.completeSnapshots && !validGitCommit(options.sourceCommit) {
		return errors.New("complete snapshot source commit differs")
	}
	return nil
}

func validAcceptanceDeadline(deadline time.Duration) bool {
	return deadline > 0 && deadline <= core.StageDeadline()
}

func validRunnerDeadline(deadline time.Duration, rustSuite bool) bool {
	limit := core.StageDeadline()
	if rustSuite {
		limit += limit/2 + limit/4 + core.CleanupDeadline()
	}
	return deadline > 0 && deadline <= limit
}

func validateProjectionRoots(skillRoot, projectWorkingRoot, projectScratchRoot, runtimeTemporaryRoot, recordRoot, source string) error {
	paths := map[string]string{
		"skill root": skillRoot, "project working root": projectWorkingRoot,
		"project build temporary root": projectionTemporaryRoot(projectWorkingRoot),
		"project scratch root":         projectScratchRoot, "runtime temporary root": runtimeTemporaryRoot,
		"record root": recordRoot, "source": source,
	}
	for name, path := range paths {
		if err := noFollowPath(name, path); err != nil {
			return err
		}
	}
	if !containedBy(projectScratchRoot, source) {
		return errors.New("source outside project scratch root")
	}
	if !containedBy(projectScratchRoot, recordRoot) {
		return errors.New("record root outside project scratch root")
	}
	for name, path := range map[string]string{
		"project lock root": filepath.Join(projectScratchRoot, "locks"),
		"record root":       recordRoot, "runtime temporary root": runtimeTemporaryRoot,
		"project working root": projectWorkingRoot,
	} {
		if pathsOverlap(source, path) {
			return fmt.Errorf("%s overlaps source", name)
		}
	}
	return nil
}

func validWorkingDirectory(value string) bool {
	if value == "." {
		return true
	}
	return value != "" && !filepath.IsAbs(value) && filepath.Clean(value) == value && value != ".." && !strings.HasPrefix(value, "../")
}

func freezeSourceToolchainProjection(source, projectWorkingRoot, projectScratchRoot, runtimeTemporaryRoot, release string) (sourceToolchainProjection, error) {
	return sourceToolchain(source, projectWorkingRoot, projectScratchRoot, runtimeTemporaryRoot, release, newDeadline(core.StageDeadline(), nil))
}

func sourceToolchain(source, projectWorkingRoot, projectScratchRoot, runtimeTemporaryRoot, release string, window deadline) (sourceToolchainProjection, error) {
	projection, err := runnerSource(source, projectWorkingRoot, projectScratchRoot, runtimeTemporaryRoot, release, window)
	if err != nil {
		return sourceToolchainProjection{}, err
	}
	toolchain, err := resolveToolchain(projectWorkingRoot, window)
	if err != nil {
		return sourceToolchainProjection{}, preflightHelperError{err}
	}
	if err := window.check(); err != nil {
		return sourceToolchainProjection{}, err
	}
	return sourceToolchainProjection{sourceProjection: projection, Toolchain: toolchain}, nil
}

func runnerSourceProjection(source, projectWorkingRoot, projectScratchRoot, runtimeTemporaryRoot, release string) (sourceProjection, error) {
	return runnerSource(source, projectWorkingRoot, projectScratchRoot, runtimeTemporaryRoot, release, newDeadline(core.StageDeadline(), nil))
}

func runnerSource(source, projectWorkingRoot, projectScratchRoot, runtimeTemporaryRoot, release string, window deadline) (sourceProjection, error) {
	if !strings.HasPrefix(release, "rust-v") || !validName(release, "-._") {
		return sourceProjection{}, errors.New("invalid projection release")
	}
	projection, projectionErr := nativeSourceWithin(runtimeTemporaryRoot, source, "custody-source-projection", window.remaining(), window.signals)
	if projectionErr != nil {
		condition := fmt.Sprintf("source role rejected: path=%s branch=%s clean=%t", source, projection.SourceBranch, projection.SourceClean)
		return sourceProjection{}, errors.Join(projectionErr, errors.New(condition))
	}
	baseline := projection.SourceBranch == release+"-upstream-baseline"
	fixed := containedBy(projectScratchRoot, source) && projection.SourceClean && (projection.SourceBranch != "" || filepath.Dir(source) == filepath.Join(projectScratchRoot, "tmp"))
	if baseline {
		roles, roleErr := readReleaseWorktrees(source, projectScratchRoot, release, window)
		fixed = fixed && roleErr == nil && roles.Registered[projection.SourceBranch] == source
		projectionErr = errors.Join(roleErr, runnerTagParentAt(source, release, window))
	}
	if projectionErr != nil || !fixed {
		condition := fmt.Sprintf("source role rejected: path=%s branch=%s clean=%t", source, projection.SourceBranch, projection.SourceClean)
		return sourceProjection{}, errors.Join(projectionErr, errors.New(condition))
	}
	if err := window.check(); err != nil {
		return sourceProjection{}, err
	}
	return projection, nil
}

func runnerTagParentAt(source, release string, window deadline) error {
	if err := runGitWithin(window, nil, "-C", source, "diff", "--quiet", "--no-ext-diff", "HEAD^", "HEAD", "--"); err != nil {
		return errors.Join(err, errors.New("upstream baseline marker is not empty"))
	}
	if err := runGitWithin(window, nil, "-C", source, "merge-base", "--is-ancestor", release+"^", "HEAD^"); err != nil {
		return errors.Join(err, errors.New("upstream baseline marker parent differs"))
	}
	if err := runGitWithin(window, nil, "-C", source, "merge-base", "--is-ancestor", "HEAD^", release+"^"); err != nil {
		return errors.Join(err, errors.New("upstream baseline marker parent differs"))
	}
	if err := runGitWithin(window, nil, "-C", source, "rev-parse", "--verify", "--quiet", "HEAD^2"); err == nil {
		return errors.New("upstream baseline marker has multiple parents")
	} else if errors.Is(err, errNativeTimeout) || errors.Is(err, errOperationCanceled) {
		return err
	}
	return nil
}

func runProjection(request projectionRequest) (projectionResult, error) {
	if request.window.end.IsZero() {
		request.window = newDeadline(request.Deadline, request.Cancellation)
	}
	if request.LockHeld == false {
		return projectionResult{FailureDomain: failureDomainHelper}, errors.New("ProjectedRunner requires CargoLock")
	}
	if err := validateProjectionRoots(request.SkillRoot, request.ProjectWorkingRoot, request.ProjectScratchRoot, request.RuntimeTemporaryRoot, request.RecordRoot, request.Source); err != nil {
		return projectionResult{FailureDomain: failureDomainInput}, err
	}
	if request.Release == "" || !validName(request.Release, "-._") {
		return projectionResult{FailureDomain: failureDomainInput}, errors.New("invalid projection release")
	}
	if (request.Consumer.rustSuite || request.RunnerRequest != nil) && (request.OutputRoot != projectionOutputRoot(request.ProjectWorkingRoot, request.AttemptIdentity) || noFollowPath("runner output root", request.OutputRoot) != nil) {
		return projectionResult{FailureDomain: failureDomainInput}, errors.New("runner output root differs")
	}
	if !validWorkingDirectory(request.WorkingDirectory) || len(request.Arguments) == 0 || request.Arguments[0] == "" {
		return projectionResult{FailureDomain: failureDomainInput}, errors.New("invalid projection command")
	}
	if !validRunnerDeadline(request.Deadline, request.Consumer.rustSuite) {
		return projectionResult{FailureDomain: failureDomainInput}, errors.New("invalid projection deadline")
	}
	if err := request.window.check(); err != nil {
		return projectionResult{FailureDomain: failureDomainHelper}, err
	}
	if request.WritableSourceRoot != "" {
		root := filepath.Join(request.Source, request.WritableSourceRoot)
		info, rootErr := os.Lstat(root)
		if !validWorkingDirectory(request.WritableSourceRoot) || request.WritableSourceRoot == "." || rootErr != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			return projectionResult{FailureDomain: failureDomainInput}, errors.Join(rootErr, errors.New("invalid writable source root"))
		}
	}
	projection, err := sourceToolchain(request.Source, request.ProjectWorkingRoot, request.ProjectScratchRoot, request.RuntimeTemporaryRoot, request.Release, request.window)
	if err != nil {
		return projectionResult{FailureDomain: runnerFailureDomain(err)}, err
	}
	if request.CaptureSnapshots {
		data, headErr := repositoryGitOutput(request.Source, "rev-parse", "HEAD")
		current := strings.TrimSuffix(string(data), "\n")
		if headErr != nil || !validGitCommit(current) || request.SourceCommit != "" && request.SourceCommit != current {
			return projectionResult{FailureDomain: failureDomainHelper}, errors.Join(headErr, errors.New("source commit differs"))
		}
		request.SourceCommit = current
	}
	attemptRoot := request.AttemptRoot
	if attemptRoot == "" {
		attemptID, idErr := randomID()
		if idErr != nil {
			return projectionResult{FailureDomain: failureDomainHelper}, idErr
		}
		attemptRoot = projectionAttemptRoot(request.ProjectWorkingRoot, attemptID)
	}
	if attemptRoot != projectionAttemptRoot(request.ProjectWorkingRoot, filepath.Base(attemptRoot)) || noFollowPath("projection attempt root", attemptRoot) != nil {
		return projectionResult{FailureDomain: failureDomainInput}, errors.New("attempt outside project build temporary root")
	}
	request.AttemptRoot = attemptRoot
	ownership := "Absent"
	var retainedPreparation *cargoPreparation
	var terminalObservation *processTerminalObservation
	if request.AttemptIdentity != "" {
		ownership, retainedPreparation, terminalObservation, err = projectionAttemptOwnershipState(request.RecordRoot, request.ProjectScratchRoot, request.AttemptIdentity, true, request.RunnerRequest, nil)
		if err != nil {
			return projectionResult{FailureDomain: failureDomainHelper}, err
		}
		if ownership != "Absent" && (retainedPreparation == nil || retainedPreparation.SourceCommit != request.SourceCommit) {
			return projectionResult{FailureDomain: failureDomainHelper}, errors.New("retained source commit differs")
		}
		if err := request.window.check(); err != nil {
			return projectionResult{FailureDomain: failureDomainHelper}, err
		}
	}
	if request.Consumer.rustSuite && request.ResumeReaped && (ownership == "Incomplete" || ownership == "Reaped" && terminalObservation.Canceled && !terminalObservation.TimedOut) {
		request.continueSuite = true
		retainedPreparation.stage = request.CargoStage
		prepared := &preparedProjectionCargo{Cache: cacheProjection(request.ProjectWorkingRoot), Preparation: *retainedPreparation}
		terminalPath := filepath.Join(request.RecordRoot, "process", request.AttemptIdentity, "terminal.json")
		if err := removeOwnedPaths(terminalPath, terminalPath+".pending", terminalPath+".stage", filepath.Join(request.OutputRoot, "snapshot-updates.json")); err != nil {
			return projectionResult{FailureDomain: failureDomainHelper}, err
		}
		result, runErr := executeProjection(request, attemptRoot, projection, prepared)
		return finishProjectionAttempt(request, attemptRoot, prepared, result, runErr)
	}
	if ownership == "Incomplete" {
		class := "TerminalPublication"
		result := projectionResult{AttemptID: filepath.Base(attemptRoot), ProcessState: "IncompleteBeforeDurableRunnerEvidence", FailureClass: &class, FailureDomain: failureDomainHelper, Preparation: retainedPreparation}
		resultFiles := append([]string{"runner.stdout.log", "runner.stderr.log"}, request.ResultFiles...)
		resultFiles = append(resultFiles, "sccache-stats.json")
		outputErr := publishProjectionFiles(request, request.stagePaths(attemptRoot)["result"], resultFiles, &result)
		if outputErr != nil {
			class = "OutputPublication"
		}
		terminalErr := errors.Join(outputErr, errors.New("process Terminal publication did not complete after group absence"))
		if request.PublishTerminal != nil {
			return result, errors.Join(terminalErr, request.PublishTerminal(result, terminalErr))
		}
		cleanupErr := removeProjectionAttempt(attemptRoot, request.RuntimeTemporaryRoot, request.OutputRoot)
		result.AttemptCleaned = cleanupErr == nil
		result.AttemptCleanup = "IncompleteAttemptCleanupComplete"
		return result, errors.Join(terminalErr, cleanupErr)
	}
	if ownership == "Reaped" && !request.ResumeReaped {
		class := "TerminalPublication"
		result := projectionResult{
			AttemptID: filepath.Base(attemptRoot), Reaped: true, ExecutionStarted: terminalObservation.ExecutionStarted, ExitCode: terminalObservation.ExitCode,
			TimedOut: terminalObservation.TimedOut, Canceled: terminalObservation.Canceled, DurationMilliseconds: terminalObservation.DurationMilliseconds,
			LogsTruncated: terminalObservation.LogsTruncated, SourceUnchanged: terminalObservation.SourceUnchanged,
			FailureClass: &class, FailureDomain: failureDomainHelper, Observation: terminalObservation, Preparation: retainedPreparation,
		}
		if request.OutputRoot != "" {
			result.Files = map[string]projectionFile{}
			outputNames := []string{"runner.stdout.log", "runner.stderr.log"}
			if !result.TimedOut && !result.Canceled {
				outputNames = append(outputNames, "sccache-stats.json")
			}
			for _, name := range outputNames {
				path := filepath.Join(request.OutputRoot, name)
				if _, err := readRegularNoFollow(path); err == nil {
					result.Files[name] = projectionFile{Path: path}
				}
			}
		}
		setProjectionProcessState(&result)
		interrupted := errors.New("consumer terminal publication was interrupted")
		if request.PublishTerminal != nil {
			return result, errors.Join(interrupted, request.PublishTerminal(result, interrupted))
		}
		cleanupErr := removeProjectionAttempt(attemptRoot, request.RuntimeTemporaryRoot, request.OutputRoot)
		result.AttemptCleaned = cleanupErr == nil
		result.AttemptCleanup = "ExactAttemptRootAbsent"
		return result, errors.Join(cleanupErr, interrupted)
	}
	if ownership == "Reaped" {
		retainedPreparation.stage = request.CargoStage
		prepared := &preparedProjectionCargo{Cache: cacheProjection(request.ProjectWorkingRoot), Preparation: *retainedPreparation}
		paths := request.stagePaths(attemptRoot)
		result, runErr := completeReapedProjection(request, paths, projection, terminalObservation, cleanupSccacheSocket(filepath.Join(paths["result"], "sccache.sock")))
		return finishProjectionAttempt(request, attemptRoot, prepared, result, runErr)
	}
	outputRoot := request.stagePaths(attemptRoot)["result"]
	if _, outputErr := os.Lstat(outputRoot); outputErr == nil {
		return projectionResult{FailureDomain: failureDomainHelper}, fmt.Errorf("fresh projection output root remains present: %s", outputRoot)
	} else if !os.IsNotExist(outputErr) {
		return projectionResult{FailureDomain: failureDomainHelper}, outputErr
	}
	prepared, err := projectionCargoPreparation(request, projection)
	if err == nil {
		prepared, err = applyProjectionCargoPreparation(request, projection, prepared)
	}
	if err != nil {
		if prepared != nil {
			class := "Materialization"
			return projectionResult{Materialization: prepared.Preparation.Materialization, FailureClass: &class, FailureDomain: failureDomainHelper}, err
		}
		return projectionResult{FailureDomain: runnerFailureDomain(err)}, err
	}
	if prepared.Preparation.SourceCommit != "" {
		return projectionResult{FailureDomain: failureDomainHelper}, errors.New("fresh source commit differs")
	}
	prepared.Preparation.SourceCommit = request.SourceCommit
	if err := removeProjectionScratch(attemptRoot, request.RuntimeTemporaryRoot, request.window); err != nil {
		return projectionResult{Materialization: prepared.Preparation.Materialization, FailureDomain: failureDomainHelper}, err
	}
	if err := ensureDirectoryNoFollow(attemptRoot, 0o700); err != nil {
		return projectionResult{Materialization: prepared.Preparation.Materialization, FailureDomain: failureDomainHelper}, err
	}
	request.Deadline = request.window.remaining()
	if request.Deadline <= 0 {
		return finishProjectionAttempt(request, attemptRoot, prepared, projectionResult{}, errNativeTimeout)
	}
	result, runErr := executeProjection(request, attemptRoot, projection, prepared)
	return finishProjectionAttempt(request, attemptRoot, prepared, result, runErr)
}

func finishProjectionAttempt(request projectionRequest, attemptRoot string, prepared *preparedProjectionCargo, result projectionResult, runErr error) (projectionResult, error) {
	result.AttemptID = filepath.Base(attemptRoot)
	addPreparation := prepared.Preparation
	result.Preparation = &addPreparation
	if request.Consumer.rustSuite && result.Canceled && !result.TimedOut && result.Reaped && result.Added {
		result.Observation = terminalObservation(result)
		_, _, _, err := projectionAttemptOwnershipState(request.RecordRoot, request.ProjectScratchRoot, result.AttemptID, true, request.RunnerRequest, result.Observation)
		return result, errors.Join(runErr, err, errOperationCanceled)
	}
	setProjectionFailure(&result, "Execution", runErr)
	preparation := prepared.Preparation
	err := request.window.check()
	result.Materialization = preparation.Materialization
	if err != nil {
		setProjectionFailure(&result, "Materialization", err)
		runErr = errors.Join(runErr, err)
	}
	if result.Reaped && result.ExecutionStarted && !runnerOwnershipLost(result) && !result.TimedOut {
		if err := bindSccacheStatistics(&result, &preparation, request.window); err != nil {
			setProjectionFailure(&result, "SccacheStatistics", err)
			runErr = errors.Join(runErr, err)
		}
		result.Materialization = preparation.Materialization
	}
	setProjectionProcessState(&result)
	if result.Observation == nil && result.Reaped && result.Added && !runnerOwnershipLost(result) {
		result.Observation = terminalObservation(result)
	}
	if result.Observation != nil && request.PublishTerminal == nil {
		ownership, _, _, terminalErr := projectionAttemptOwnershipState(request.RecordRoot, request.ProjectScratchRoot, result.AttemptID, true, request.RunnerRequest, result.Observation)
		if terminalErr != nil || ownership != "Reaped" {
			setProjectionHelperFailure(&result, "TerminalPublication")
			runErr = errors.Join(runErr, terminalErr, errors.New("fixed process Terminal is absent"))
		}
	}
	publishTerminal := request.RetainAttempt && result.Reaped && result.Observation != nil
	retainAttempt := publishTerminal || result.ExecutionStarted && !result.Reaped
	cleanupErr := error(nil)
	if !retainAttempt {
		cleanupErr = removeProjectionScratch(attemptRoot, request.RuntimeTemporaryRoot, newDeadline(core.GracePeriod(), nil))
	}
	if cleanupErr != nil {
		setProjectionFailure(&result, "Cleanup", cleanupErr)
		runErr = errors.Join(runErr, cleanupErr)
	} else if !retainAttempt {
		result.AttemptCleaned = true
		if result.ExecutionStarted && result.Reaped {
			if runnerOwnershipLost(result) {
				result.AttemptCleanup = "OwnershipLostCleanupComplete"
			} else {
				result.AttemptCleanup = "ExactAttemptRootAbsent"
			}
		}
	}
	if result.FailureDomain == "" {
		result.FailureDomain = failureDomainNone
	}
	if publishTerminal {
		publishErr := errors.New("RustSuite Stage publisher is absent")
		if request.PublishTerminal != nil {
			publishErr = request.PublishTerminal(result, runErr)
		}
		if publishErr != nil {
			setProjectionHelperFailure(&result, "StagePublication")
			runErr = errors.Join(runErr, publishErr)
		} else {
			result.AttemptCleaned = true
			result.AttemptCleanup = "ExactAttemptRootAbsent"
		}
	}
	if runErr == nil && result.FailureDomain == failureDomainNone && !projectionHasProductFailure(result) && request.RunnerRequest == nil && !request.Consumer.rustSuite && request.OutputRoot != "" && request.OutputRoot != request.stagePaths(attemptRoot)["result"] {
		if err := removePrivateAttemptWithin(request.stagePaths(attemptRoot)["result"], newDeadline(core.GracePeriod(), nil)); err != nil {
			setProjectionHelperFailure(&result, "OutputCleanup")
			runErr = err
		}
	}
	if runErr != nil {
		return result, runErr
	}
	return result, nil
}

func removeProjectionScratch(attemptRoot, runtimeRoot string, window deadline) error {
	paths := projectionScratchRoots(attemptRoot, runtimeRoot)
	for index, path := range paths {
		if err := noFollowPath("projection scratch", path); err != nil {
			return err
		}
		for _, previous := range paths[:index] {
			if pathsOverlap(previous, path) {
				return errors.New("projection scratch roots overlap")
			}
		}
		if info, err := os.Lstat(path); err == nil {
			if !info.IsDir() {
				return fmt.Errorf("projection scratch is not a directory: %s", path)
			}
		} else if !os.IsNotExist(err) {
			return err
		}
	}
	var cleanupErr error
	for _, path := range paths {
		cleanupErr = errors.Join(cleanupErr, removePrivateAttemptWithin(path, window))
	}
	return cleanupErr
}

func projectionAttemptAbsent(attemptRoot, runtimeRoot string) error {
	for _, path := range projectionScratchRoots(attemptRoot, runtimeRoot) {
		if _, err := os.Lstat(path); os.IsNotExist(err) {
			continue
		} else if err != nil {
			return err
		}
		return fmt.Errorf("projection scratch remains present: %s", path)
	}
	return nil
}

func removeProjectionAttempt(attemptRoot, runtimeRoot, outputRoot string) error {
	if err := removeProjectionScratch(attemptRoot, runtimeRoot, newDeadline(core.StageDeadline(), nil)); err != nil || outputRoot == "" {
		return err
	}
	return removeOwnedPaths(outputRoot)
}

func projectionAttemptOwnershipState(recordRoot, projectScratchRoot, attemptID string, recoverTerminal bool, expectedRequest *runnerRequestBinding, expected *processTerminalObservation) (string, *cargoPreparation, *processTerminalObservation, error) {
	if !validName(attemptID, "-._") {
		return "", nil, nil, errors.New("projection attempt identity differs")
	}
	directory := filepath.Join(recordRoot, "process", attemptID)
	lockPath := filepath.Join(directory, "lock.json")
	publicationSource := func(path string) string {
		for _, candidate := range []string{path, path + ".pending", path + ".stage"} {
			if _, err := os.Lstat(candidate); !os.IsNotExist(err) {
				return candidate
			}
		}
		return path
	}
	var lock struct {
		SchemaVersion   int
		RecordType      string
		AttemptID, Lock string
		Path            string
		Phase           string `json:",omitempty"`
	}
	data, err := readRegularNoFollow(publicationSource(lockPath))
	if os.IsNotExist(err) {
		return "Absent", nil, nil, nil
	}
	if err != nil {
		return "", nil, nil, errors.Join(err, errors.New("projection lock record differs"))
	}
	decodeErr := json.Unmarshal(data, &lock)
	canonical, canonicalErr := core.CanonicalJSON(&lock)
	keysErr := validateJSONUniqueKeys(data)
	if decodeErr != nil || canonicalErr != nil || keysErr != nil || !bytes.Equal(data, canonical) || lock.SchemaVersion != core.SchemaVersion || lock.RecordType != "LockRecord" || lock.AttemptID != attemptID || lock.Lock != "CargoLock" || lock.Path != filepath.Join(projectScratchRoot, "locks", "cargo-lock") || lock.Phase != "" && !validProjectionPhase(lock.Phase) {
		return "", nil, nil, errors.Join(decodeErr, canonicalErr, keysErr, errors.New("projection lock record differs"))
	}
	if err := publishNoReplace(lockPath, data, 0o600); err != nil {
		return "", nil, nil, err
	}
	readGroup := func(path, transition string) (processGroupRecord, bool, error) {
		var record processGroupRecord
		data, err := readCanonicalValue(publicationSource(path), &record)
		if os.IsNotExist(err) {
			return record, false, nil
		}
		terminal, preparation := record.Observation, record.Preparation
		requestMatches := expectedRequest == nil && record.Request == nil || expectedRequest != nil && record.Request != nil && reflect.DeepEqual(*record.Request, *expectedRequest)
		if err != nil || record.SchemaVersion != core.SchemaVersion || record.RecordType != "LockGroupRecord" || record.AttemptID != attemptID || record.ProcessGroupID <= 0 || record.Transition != transition || transition == "Add" && (terminal != nil || preparation == nil || !validCargoMaterializationAction(preparation.Materialization) || preparation.stage != "" || !requestMatches) || transition == "Terminal" && (preparation != nil || record.Request != nil || !validProcessTerminal(terminal)) {
			return record, false, errors.Join(err, errors.New("projection process record differs"))
		}
		err = publishNoReplace(path, data, 0o600)
		return record, err == nil, err
	}
	addProcess, addPresent, err := readGroup(filepath.Join(directory, "add.json"), "Add")
	if err != nil {
		return "", nil, nil, err
	}
	if !addPresent {
		return "Absent", nil, nil, nil
	}
	terminalPath := filepath.Join(directory, "terminal.json")
	terminalProcess, terminalPresent, err := readGroup(terminalPath, "Terminal")
	if err != nil {
		return "", nil, nil, err
	}
	if !terminalPresent && recoverTerminal {
		if expected != nil {
			if !validProcessTerminal(expected) {
				return "", nil, nil, errors.New("prospective process Terminal differs")
			}
			if _, err := writeProcessGroupRecord(recordRoot, attemptID, addProcess.ProcessGroupID, "Terminal", nil, nil, expected); err != nil {
				return "", nil, nil, err
			}
			terminalProcess, terminalPresent, err = readGroup(terminalPath, "Terminal")
			if err != nil {
				return "", nil, nil, err
			}
		}
	}
	if terminalPresent && terminalProcess.ProcessGroupID != addProcess.ProcessGroupID {
		return "", nil, nil, errors.New("projection Terminal has no matching Add")
	}
	if terminalPresent && expected != nil && !reflect.DeepEqual(terminalProcess.Observation, expected) {
		return "", nil, nil, errors.New("prospective process Terminal changed")
	}
	if terminalPresent {
		return "Reaped", addProcess.Preparation, terminalProcess.Observation, nil
	}
	if err := syscall.Kill(-addProcess.ProcessGroupID, 0); !errors.Is(err, syscall.ESRCH) {
		return "", nil, nil, errors.Join(err, errors.New("projection process group remains live"))
	}
	return "Incomplete", addProcess.Preparation, nil, nil
}

func cargoIdentityFor(request projectionRequest, view sourceToolchainProjection) (cargoIdentity, error) {
	selection, err := cargoWorktreeSelection(request.ProjectScratchRoot, request.Source)
	if err != nil {
		return cargoIdentity{}, err
	}
	return cargoIdentity{CargoVersion: view.Toolchain.CargoVersion, ToolchainRoot: view.Toolchain.Root, Target: view.Toolchain.Target, SourceSelection: selection}, nil
}

func prepareProjectionCargo(request projectionRequest, projection sourceToolchainProjection) (*preparedProjectionCargo, error) {
	if !request.LockHeld {
		return nil, errors.New("Cargo preparation requires CargoLock")
	}
	cache := cacheProjection(request.ProjectWorkingRoot)
	identity, err := cargoIdentityFor(request, projection)
	if err != nil {
		return nil, err
	}
	preparation, err := prepareCargo(cache, identity, request.CargoStage, request.window)
	return &preparedProjectionCargo{Cache: cache, Preparation: preparation}, err
}

func projectionCargoPreparation(request projectionRequest, projection sourceToolchainProjection) (*preparedProjectionCargo, error) {
	if request.PreparedCargo == nil {
		return prepareProjectionCargo(request, projection)
	}
	prepared := request.PreparedCargo
	cache := cacheProjection(request.ProjectWorkingRoot)
	if cache != prepared.Cache || prepared.Preparation.stage != request.CargoStage {
		return nil, errors.New("prepared Cargo cache identity changed")
	}
	current, err := cargoIdentityFor(request, projection)
	if err != nil {
		return nil, err
	}
	expected := prepared.Preparation.Materialization.Current
	current.Generation = expected.Generation
	if current != expected {
		return nil, errors.New("prepared Cargo structural identity changed")
	}
	return prepared, nil
}

func applyProjectionCargoPreparation(request projectionRequest, projection sourceToolchainProjection, prepared *preparedProjectionCargo) (*preparedProjectionCargo, error) {
	if err := request.window.check(); err != nil {
		return prepared, err
	}
	if prepared == nil || prepared.Cache != cacheProjection(request.ProjectWorkingRoot) || prepared.Preparation.stage != request.CargoStage {
		return prepared, errors.New("prepared Cargo handoff differs")
	}
	current, err := cargoIdentityFor(request, projection)
	if err != nil {
		return prepared, err
	}
	current.Generation = prepared.Preparation.Materialization.Current.Generation
	if current != prepared.Preparation.Materialization.Current {
		return prepared, errors.New("prepared Cargo structural identity changed")
	}
	if err := ensureCargoProjection(prepared.Cache); err != nil {
		return prepared, err
	}
	state, present, err := readCargoState(prepared.Cache.CargoBuild)
	if err != nil {
		return prepared, err
	}
	if prepared.Preparation.Materialization.Action == "ReusedWorkspace" && (!present || state != current) {
		return prepared, errors.New("reused Cargo workspace state changed before execution")
	}
	if prepared.Preparation.Materialization.Action == "InvalidatedWorkspace" && (!present || state != current) {
		if err := invalidateCargoWorkspace(request, projection.Toolchain, prepared.Cache); err != nil {
			return prepared, fmt.Errorf("CargoPreparation/WorkspaceInvalidation: %w", err)
		}
	}
	if err := request.window.check(); err != nil {
		return prepared, err
	}
	if err := publishCargoState(prepared.Cache.CargoBuild, prepared.Preparation.Materialization.Current); err != nil {
		return prepared, err
	}
	return prepared, nil
}

func invalidateCargoWorkspace(request projectionRequest, toolchain ToolchainProjection, cache CacheProjection) error {
	outputRoot := request.stagePaths(request.AttemptRoot)["result"]
	if err := ensureDirectoryNoFollow(outputRoot, 0o700); err != nil {
		return err
	}
	stdoutFile, err := openProjectionLog(filepath.Join(outputRoot, "cargo-invalidation.stdout.log"), false)
	if err != nil {
		return err
	}
	stderrFile, err := openProjectionLog(filepath.Join(outputRoot, "cargo-invalidation.stderr.log"), false)
	if err != nil {
		return errors.Join(err, stdoutFile.Close())
	}
	stdout := &runnerLogWriter{writer: stdoutFile}
	stderr := &runnerLogWriter{writer: stderrFile}
	command := exec.Command(filepath.Join(toolchain.Path, "rust", "bin", "cargo"), "clean", "--workspace", "--target-dir", cache.CargoTarget, "--frozen")
	command.Dir = filepath.Join(request.Source, "codex-rs")
	command.Env = []string{"PATH=/usr/bin", "CARGO_HOME=" + cache.CargoHome, "CARGO_BUILD_BUILD_DIR=" + cache.CargoBuild, "RUSTC_BOOTSTRAP=1"}
	command.Stdout, command.Stderr = stdout, stderr
	command.WaitDelay = core.GracePeriod()
	runErr := request.window.run(command)
	runErr = errors.Join(runErr, stdout.err, stderr.err, stdoutFile.Close(), stderrFile.Close())
	if runErr == nil {
		return nil
	}
	summary := summarizeRunnerLogs(stdout, stderr)
	return fmt.Errorf("%w\ndiagnostic: %s\nstdout-bytes=%d stderr-bytes=%d\ncommand=%q\ncwd=%q\noutput=%q", runErr, summary.Diagnostic, summary.StdoutBytes, summary.StderrBytes, command.Args, command.Dir, outputRoot)
}

func (request projectionRequest) stagePaths(attemptRoot string) map[string]string {
	stageRoot := filepath.Join(attemptRoot, "stage")
	attemptID := filepath.Base(attemptRoot)
	runtimeRoot := projectionRuntimeRoot(request.RuntimeTemporaryRoot, attemptID)
	return map[string]string{
		"codex": filepath.Join(runtimeRoot, "codex"), "harness": filepath.Join(runtimeRoot, "harness"),
		"tmp-lower": filepath.Join(attemptRoot, "tmp", "lower"), "tmp-upper": filepath.Join(attemptRoot, "tmp", "upper"),
		"tmp-work": filepath.Join(attemptRoot, "tmp", "work"), "run": filepath.Join(runtimeRoot, "run"), "home": filepath.Join(runtimeRoot, "home"),
		"result": projectionOutputRoot(request.ProjectWorkingRoot, attemptID),
		"shm":    filepath.Join(runtimeRoot, "shm"),
		"runner": filepath.Join(stageRoot, "runner"), "upper": filepath.Join(stageRoot, "upper"),
		"work": filepath.Join(stageRoot, "work"),
	}
}

type suiteRoundEvent struct {
	Type                 string `json:"type"`
	Event                string `json:"event"`
	Round                int    `json:"round"`
	DeadlineMilliseconds int64  `json:"deadline_ms"`
	ExitCode             int    `json:"exit"`
}

func prepareSuiteRound(request *projectionRequest, path string) error {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR|syscall.O_NOFOLLOW, 0o600)
	if err != nil {
		return err
	}
	defer file.Close()
	decoder := json.NewDecoder(file)
	current := suiteRoundEvent{}
	for {
		var event suiteRoundEvent
		err := decoder.Decode(&event)
		if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
			break
		}
		if err != nil {
			return err
		}
		if event.Type == "codex-build-round" {
			if event.Round < 1 || event.Round > 3 || event.DeadlineMilliseconds <= 0 || !listed("started continued completed", event.Event) {
				return errors.New("suite round evidence differs")
			}
			current = event
		}
	}
	if err := file.Truncate(decoder.InputOffset()); err != nil {
		return err
	}
	if _, err := file.Seek(0, io.SeekEnd); err != nil {
		return err
	}
	end := request.window.end
	if request.RoundBudget > 0 {
		end = end.Add(-request.RoundBudget/2 - request.RoundBudget/4 - core.CleanupDeadline())
	}
	if current.Round == 0 {
		if request.continueSuite {
			return errors.New("interrupted suite has no original round deadline")
		}
		current = suiteRoundEvent{Type: "codex-build-round", Event: "started", Round: 1, DeadlineMilliseconds: end.UnixMilli()}
	} else if current.Event == "completed" {
		if current.ExitCode != 100 || current.Round == 3 {
			request.suiteRound, request.roundEnd = current.Round, time.Now().Add(core.CleanupDeadline())
			return nil
		}
		current.Round++
		if request.RoundBudget > 0 {
			current.DeadlineMilliseconds = time.Now().Add(request.RoundBudget / time.Duration(1<<uint(current.Round-1))).UnixMilli()
		}
		current.Event = "started"
	} else {
		current.Event = "continued"
	}
	request.suiteRound, request.roundEnd = current.Round, time.UnixMilli(current.DeadlineMilliseconds)
	data, err := json.Marshal(current)
	if err == nil {
		_, err = file.Write(append(append([]byte{'\n'}, data...), '\n'))
	}
	return errors.Join(err, file.Sync())
}

func openProjectionPhases(request projectionRequest, path string) (func() error, <-chan time.Time, error) {
	if !request.Consumer.rustSuite {
		return func() error { return nil }, nil, nil
	}
	if request.OnPhase == nil {
		return nil, nil, errors.New("RustSuite phase consumer is absent")
	}
	if request.continueSuite {
		if err := removeOwnedPaths(path); err != nil {
			return nil, nil, err
		}
	}
	if err := syscall.Mkfifo(path, 0o600); err != nil {
		return nil, nil, err
	}
	file, err := os.OpenFile(path, os.O_RDWR|syscall.O_NONBLOCK|syscall.O_NOFOLLOW, 0)
	if err != nil {
		return nil, nil, errors.Join(err, os.Remove(path))
	}
	done := make(chan error, 1)
	deadlines := make(chan time.Time, 3)
	go func() {
		scanner := bufio.NewScanner(file)
		scanner.Buffer(make([]byte, 128), 128)
		count := 0
		for scanner.Scan() {
			phase := scanner.Text()
			var status int
			var nextDeadline int64
			valid := count == 0 && phase == "Compilation" || count == 1 && phase == "TestExecution"
			progress := count >= 2 && count <= 4 && strings.HasPrefix(phase, "TestExecution started=")
			if progress {
				var started, round int
				_, err := fmt.Sscanf(phase, "TestExecution started=%d round=%d/3", &started, &round)
				valid = err == nil && started > 0 && round == request.suiteRound+count-2 && round <= 3
			}
			if count >= 2 && count <= 4 && !progress {
				var round int
				_, err := fmt.Sscanf(phase, "TestExecution completed-round=%d/3 exit=%d deadline=%d", &round, &status, &nextDeadline)
				valid = err == nil && round == request.suiteRound+count-2 && round <= 3 && nextDeadline >= 0
				phase = fmt.Sprintf("TestExecution completed-round=%d/3 exit=%d", round, status)
			}
			if !valid {
				done <- errors.New("native RustSuite phase differs")
				return
			}
			if !progress {
				count++
			}
			if count >= 3 && !progress {
				end := time.UnixMilli(nextDeadline)
				if nextDeadline == 0 {
					end = time.Now().Add(core.CleanupDeadline())
				}
				if request.window.end.Before(end) {
					end = request.window.end
				}
				deadlines <- end
			}
			if err := request.OnPhase(phase); err != nil {
				done <- err
				return
			}
		}
		err := scanner.Err()
		if errors.Is(err, os.ErrClosed) {
			err = nil
		}
		done <- err
	}()
	return func() error {
		closeErr := file.Close()
		return errors.Join(closeErr, <-done, os.Remove(path))
	}, deadlines, nil
}

func executeProjection(request projectionRequest, attemptRoot string, projection sourceToolchainProjection, prepared *preparedProjectionCargo) (projectionResult, error) {
	if request.Consumer.rustSuite || request.RunnerRequest != nil && request.RunnerRequest.CompleteSuite {
		if err := requireBuildTemporaryHeadroom(request.ProjectWorkingRoot); err != nil {
			return projectionResult{FailureDomain: failureDomainHelper}, err
		}
	}
	codexHome := request.CodexHome
	if codexHome == "" {
		codexHome = os.Getenv("CODEX_HOME")
	}
	cache := prepared.Cache
	paths := request.stagePaths(attemptRoot)
	for _, path := range paths {
		if err := ensureDirectoryNoFollow(path, 0o700); err != nil {
			return projectionResult{}, err
		}
	}
	if request.Consumer.rustSuite {
		if err := prepareSuiteRound(&request, filepath.Join(paths["result"], "nextest.jsonl")); err != nil {
			return projectionResult{}, err
		}
		request.Consumer.environment = append(append([]string{}, request.Consumer.environment...), "CODEX_SUITE_ROUND_BUDGET_MS="+strconv.FormatInt(request.RoundBudget.Milliseconds(), 10))
	}
	environment, err := runnerEnvironment(request.Consumer.environment, codexHome)
	if err != nil {
		return projectionResult{}, err
	}
	if err := ensureDirectoryNoFollow(filepath.Join(paths["result"], "nextest"), 0o700); err != nil {
		return projectionResult{}, err
	}
	assetsToWrite := []runtimeAsset{
		{name: "bwrap.args", data: assets.RunnerBwrapArguments(), mode: 0o600},
		{name: "runner.env", data: assets.RunnerEnvironment(), mode: 0o600},
		{name: "runner-stage.zsh", data: assets.RunnerStage(), mode: 0o700},
	}
	assetsToWrite = append(assetsToWrite, request.Consumer.assets...)
	for _, asset := range assetsToWrite {
		if err := writeRuntimeAsset(filepath.Join(paths["runner"], asset.name), asset.data, asset.mode); err != nil {
			return projectionResult{}, err
		}
	}
	systemArguments, err := runnerSystemArguments(request.Consumer.networkDisabled)
	if err != nil {
		return projectionResult{}, err
	}
	arguments := runnerSandboxArguments(request, paths, projection.Toolchain, cache, codexHome, systemArguments, environment)
	limitArgs := append(core.ResourceLimitArgs(), "--", "/usr/bin/bwrap")
	executable := "/proc/self/exe"
	attemptID := filepath.Base(attemptRoot)
	if request.AttemptIdentity == "" || !validName(request.AttemptIdentity, "-._") {
		return projectionResult{}, errors.New("projection attempt identity is invalid")
	}
	started := time.Now()
	var command *exec.Cmd
	var stdoutFile, stderrFile *os.File
	var stdout, stderr *runnerLogWriter
	nonce, nonceErr := randomNonce()
	if nonceErr != nil {
		return projectionResult{}, nonceErr
	}
	readCapability, writeCapability, pipeErr := os.Pipe()
	if pipeErr != nil {
		return projectionResult{}, pipeErr
	}
	readExecStatus, writeExecStatus, pipeErr := os.Pipe()
	if pipeErr != nil {
		closeFiles(readCapability, writeCapability)
		return projectionResult{}, pipeErr
	}
	payload := []string{"runner-child-exec", "--capability-fd", "3", "--exec-status-fd", "4", "--parent-pid", strconv.Itoa(os.Getpid()), "--nonce", nonce}
	payload = append(payload, "--", "/usr/bin/prlimit")
	payload = append(payload, limitArgs...)
	payload = append(payload, arguments...)
	command = exec.Command(executable, payload...)
	command.ExtraFiles = []*os.File{readCapability, writeExecStatus}
	command.Env = []string{}
	command.SysProcAttr = &syscall.SysProcAttr{Setpgid: true, Pdeathsig: syscall.SIGKILL}
	command.WaitDelay = core.GracePeriod()
	stdoutFile, err = openProjectionLog(filepath.Join(paths["result"], "runner.stdout.log"), request.continueSuite)
	if err != nil {
		closeFiles(readCapability, writeCapability, readExecStatus, writeExecStatus)
		return projectionResult{}, err
	}
	stderrFile, err = openProjectionLog(filepath.Join(paths["result"], "runner.stderr.log"), request.continueSuite)
	if err != nil {
		closeFiles(readCapability, writeCapability, readExecStatus, writeExecStatus, stdoutFile)
		return projectionResult{}, err
	}
	stdout = &runnerLogWriter{writer: stdoutFile}
	stderr = &runnerLogWriter{writer: stderrFile}
	command.Stdout = stdout
	command.Stderr = stderr
	if request.Consumer.rustSuite {
		command.Stdout, command.Stderr = stdoutFile, stderrFile
	}
	finishPhases, roundDeadlines, phaseErr := openProjectionPhases(request, filepath.Join(paths["result"], ".phase"))
	if phaseErr != nil {
		closeFiles(readCapability, writeCapability, readExecStatus, writeExecStatus, stdoutFile, stderrFile)
		return projectionResult{}, phaseErr
	}
	if err = writeProjectionLockRecord(request.RecordRoot, request.ProjectScratchRoot, attemptID); err == nil {
		err = request.window.check()
	}
	if err == nil {
		err = command.Start()
	}
	if err != nil {
		closeFiles(readCapability, writeCapability, readExecStatus, writeExecStatus, stdoutFile, stderrFile)
		return projectionResult{}, errors.Join(err, finishPhases())
	}
	closeFiles(readCapability, writeExecStatus)
	addPublished, addErr := writeProcessGroupRecord(request.RecordRoot, attemptID, command.Process.Pid, "Add", &prepared.Preparation, request.RunnerRequest, nil)
	if addErr == nil && !addPublished {
		addErr = errors.New("runner Add failed")
	}
	if addErr == nil {
		addErr = request.window.check()
	}
	if addErr == nil && request.Consumer.rustSuite && !time.Now().Before(request.roundEnd) {
		addErr = errNativeTimeout
	}
	if addErr == nil {
		_, addErr = io.WriteString(writeCapability, nonce)
	} else {
		_ = syscall.Kill(-command.Process.Pid, syscall.SIGKILL)
	}
	addErr = errors.Join(addErr, writeCapability.Close())
	execProof, proofErr := awaitRead(request.window, io.LimitReader(readExecStatus, 4097), func() { _ = syscall.Kill(-command.Process.Pid, syscall.SIGKILL) })
	proofErr = errors.Join(addErr, proofErr)
	if proofErr == nil && len(execProof) == 0 && request.OnLive != nil && !request.continueSuite {
		request.OnLive()
	}
	firstEnd := request.window.end
	if request.Consumer.rustSuite {
		firstEnd = request.roundEnd
	}
	processErr, timedOut, canceled := waitProcessGroup(command, time.Until(firstEnd), core.GracePeriod(), request.Cancellation, roundDeadlines)
	phaseErr = finishPhases()
	if request.OnPhase != nil {
		phaseErr = errors.Join(phaseErr, request.OnPhase("Drain"))
	}
	timedOut = timedOut || errors.Is(proofErr, errNativeTimeout)
	canceled = canceled || errors.Is(proofErr, errOperationCanceled)
	streamErr := errors.Join(syncClose(stdoutFile), syncClose(stderrFile), stdout.err, stderr.err)
	closeErr := errors.Join(phaseErr, readExecStatus.Close(), streamErr)
	executionStarted := proofErr == nil && len(execProof) == 0 && addPublished
	postErr := errors.Join(proofErr, closeErr, cleanupSccacheSocket(filepath.Join(paths["result"], "sccache.sock")))
	exitCode := 0
	if processErr != nil {
		exitCode = 1
		var exitError *exec.ExitError
		if errors.As(processErr, &exitError) {
			exitCode = exitError.ExitCode()
		}
	}
	result := reapedProjectionResult(request, projection, started, projectionResult{
		ExecutionStarted: executionStarted, ExitCode: exitCode, TimedOut: timedOut, Canceled: canceled,
		Added: addPublished, OwnedReap: command.ProcessState != nil,
	})
	if !request.Consumer.rustSuite {
		result.LogSummary = summarizeRunnerLogs(stdout, stderr)
	} else if exitCode != 0 && !result.TimedOut && !result.Canceled {
		stdoutLog, stdoutErr := runnerLogPostimage(filepath.Join(paths["result"], "runner.stdout.log"), true, request.window)
		stderrLog, stderrErr := runnerLogPostimage(filepath.Join(paths["result"], "runner.stderr.log"), true, request.window)
		result.LogSummary = summarizeRunnerLogs(stdoutLog, stderrLog)
		if err := errors.Join(stdoutErr, stderrErr); err != nil {
			setProjectionFailure(&result, "OutputPublication", err)
			postErr = errors.Join(postErr, err)
		}
	}
	if canceled {
		setProjectionCanceled(&result)
		if !executionStarted {
			result.FailureDomain = failureDomainHelper
		}
	} else if timedOut {
		result.FailureDomain = failureDomainHelper
	}
	if executionStarted && !result.TimedOut && !result.Canceled {
		if streamErr != nil {
			class := "OutputPublication"
			result.FailureClass, result.FailureDomain = &class, failureDomainHelper
		}
	}
	if !executionStarted && !addPublished {
		return result, errors.Join(processErr, postErr, errors.New("payload exec failed"))
	}
	return completeProjection(request, paths, result, postErr)
}

func completeProjection(request projectionRequest, paths map[string]string, result projectionResult, postErr error) (projectionResult, error) {
	var productErr error
	updates, postimageErr := []snapshotUpdate{}, error(nil)
	deadlineErr := request.window.check()
	if deadlineErr != nil && !result.TimedOut && !result.Canceled {
		if errors.Is(deadlineErr, errOperationCanceled) {
			setProjectionCanceled(&result)
		} else {
			setProjectionTimeout(&result)
		}
		postErr = errors.Join(postErr, deadlineErr)
	}
	if result.ExecutionStarted && !result.TimedOut && !result.Canceled {
		manifest := filepath.Join(paths["result"], "snapshot-updates.json")
		var data []byte
		var manifestErr error
		if request.RunnerRequest != nil && request.CaptureSnapshots {
			data, manifestErr = readRegularNoFollow(manifest, request.window)
		}
		if manifestErr == nil && request.CaptureSnapshots {
			updates, manifestErr = completeSnapshotUpdates(request.Source, data)
		}
		postimageErr = manifestErr
		if postimageErr == nil {
			postimageErr = collectSnapshotUpdates(request, paths["upper"], updates)
		}
		postimageErr = errors.Join(postimageErr, removeOwnedPaths(manifest))
		if deadlineErr = request.window.check(); deadlineErr != nil {
			if errors.Is(deadlineErr, errOperationCanceled) {
				setProjectionCanceled(&result)
			} else {
				setProjectionTimeout(&result)
			}
			postErr = errors.Join(postErr, deadlineErr)
		}
	}
	if postimageErr != nil {
		result.GeneratedSnapshotUpdates = []snapshotUpdate{}
		var outputErr snapshotOutputError
		if errors.As(postimageErr, &outputErr) {
			class := "SourcePostimage"
			result.FailureClass = &class
			result.SourcePostimagePath = outputErr.Path
			result.FailureDomain = failureDomainProduct
			productErr = postimageErr
		} else {
			setProjectionHelperFailure(&result, "SourcePostimageInspection")
			postErr = errors.Join(postErr, postimageErr)
		}
	} else {
		result.GeneratedSnapshotUpdates = updates
	}
	if !result.ExecutionStarted || result.TimedOut || result.Canceled {
		request.window = newDeadline(core.GracePeriod(), nil)
	}
	resultFiles := append([]string{"runner.stdout.log", "runner.stderr.log"}, request.ResultFiles...)
	statisticsPath := filepath.Join(paths["result"], "sccache-stats.json")
	_, statisticsStatErr := os.Lstat(statisticsPath)
	statisticsRequired := result.ExecutionStarted && (!(result.TimedOut || result.Canceled) || !os.IsNotExist(statisticsStatErr))
	if statisticsRequired {
		resultFiles = append(resultFiles, "sccache-stats.json")
	}
	if err := publishProjectionFiles(request, paths["result"], resultFiles, &result); err != nil {
		setProjectionFailure(&result, "OutputPublication", err)
		return result, errors.Join(postErr, productErr, err)
	}
	if postErr != nil {
		setProjectionHelperFailure(&result, "Cleanup")
		return result, errors.Join(postErr, productErr)
	}
	if productErr != nil {
		return result, productErr
	}
	return result, nil
}

func reapedProjectionResult(request projectionRequest, projection sourceToolchainProjection, started time.Time, result projectionResult) projectionResult {
	result.Reaped = true
	result.DurationMilliseconds = time.Since(started).Milliseconds()
	if result.Canceled {
		setProjectionCanceled(&result)
		return result
	}
	if result.TimedOut {
		result.FailureDomain = failureDomainHelper
		return result
	}
	postToolchain, toolchainErr := resolveToolchain(request.ProjectWorkingRoot, request.window)
	if errors.Is(toolchainErr, errOperationCanceled) {
		setProjectionCanceled(&result)
		return result
	}
	action := "custody-source-projection"
	if filepath.Dir(request.Source) == filepath.Join(request.ProjectScratchRoot, "tmp") {
		action = "custody-detached-source-projection"
	}
	actual, projectionErr := nativeSourceWithin(request.RuntimeTemporaryRoot, request.Source, action, request.window.remaining(), request.Cancellation)
	postCommit := ""
	if request.SourceCommit != "" {
		data, headErr := repositoryGitOutput(request.Source, "rev-parse", "HEAD")
		postCommit = strings.TrimSuffix(string(data), "\n")
		projectionErr = errors.Join(projectionErr, headErr)
	}
	result.SourceUnchanged = toolchainErr == nil && projectionErr == nil && actual == projection.sourceProjection && postToolchain == projection.Toolchain && postCommit == request.SourceCommit
	if errors.Is(projectionErr, errOperationCanceled) {
		setProjectionCanceled(&result)
	} else if errors.Is(projectionErr, errNativeTimeout) || request.window.timeout() != nil {
		setProjectionTimeout(&result)
	}
	return result
}

func openProjectionLog(path string, appendOutput bool) (*os.File, error) {
	flags := os.O_WRONLY | os.O_CREATE | os.O_EXCL | syscall.O_NOFOLLOW
	if appendOutput {
		flags = os.O_WRONLY | os.O_APPEND | syscall.O_NOFOLLOW
	}
	return os.OpenFile(path, flags, 0o600)
}

func randomNonce() (string, error) {
	data := make([]byte, 32)
	if _, err := rand.Read(data); err != nil {
		return "", err
	}
	return hex.EncodeToString(data), nil
}

func terminalObservation(result projectionResult) *processTerminalObservation {
	class := result.FailureClass
	if class != nil && !listed("Capability ControllerCancellation", *class) {
		class = nil
	}
	return &processTerminalObservation{result.ExecutionStarted, result.ExitCode, result.TimedOut, result.Canceled, result.DurationMilliseconds, result.LogsTruncated, result.SourceUnchanged, class}
}

func validProcessTerminal(value *processTerminalObservation) bool {
	return value != nil && value.DurationMilliseconds >= 0 && !(value.TimedOut && value.Canceled) && (!value.Canceled || value.FailureClass != nil && *value.FailureClass == "ControllerCancellation") && (value.FailureClass == nil || listed("Capability ControllerCancellation", *value.FailureClass))
}

func completeReapedProjection(request projectionRequest, paths map[string]string, projection sourceToolchainProjection, observation *processTerminalObservation, postErr error) (projectionResult, error) {
	if !validProcessTerminal(observation) {
		return projectionResult{FailureDomain: failureDomainHelper}, errors.New("projection Terminal observation differs")
	}
	result := projectionResult{ExecutionStarted: observation.ExecutionStarted, Reaped: true, OwnedReap: true, ExitCode: observation.ExitCode, TimedOut: observation.TimedOut, Canceled: observation.Canceled, DurationMilliseconds: observation.DurationMilliseconds, LogsTruncated: observation.LogsTruncated, SourceUnchanged: observation.SourceUnchanged, FailureClass: observation.FailureClass, Observation: observation}
	if result.Canceled {
		result.FailureDomain = failureDomainProduct
	} else if result.TimedOut {
		result.FailureDomain = failureDomainHelper
	}
	return completeProjection(request, paths, result, postErr)
}

func validProjectionPhase(phase string) bool {
	return listed("CargoLock SourceProjection Toolchain PackageDiscovery PlanPublication Cargo Runner Compilation TestExecution Drain Cleanup Stage BaselinePromotion ResultPublication Terminal", phase)
}

func writeProjectionLockRecord(recordRoot, projectScratchRoot, attemptID string, phases ...string) error {
	record := map[string]any{
		"SchemaVersion": core.SchemaVersion, "RecordType": "LockRecord", "AttemptID": attemptID,
		"Lock": "CargoLock", "Path": filepath.Join(projectScratchRoot, "locks", "cargo-lock"),
	}
	path := filepath.Join(recordRoot, "process", attemptID, "lock.json")
	var current map[string]any
	_, readErr := readCanonicalValue(path, &current)
	if readErr == nil {
		phase, present := current["Phase"]
		if present {
			name, valid := phase.(string)
			if !valid || !validProjectionPhase(name) {
				return errors.New("process Phase differs")
			}
			delete(current, "Phase")
			record["Phase"] = phase
		}
		identity := map[string]any{}
		for key, value := range record {
			if key != "Phase" {
				identity[key] = value
			}
		}
		if !rustSuiteSameValue(current, identity) {
			return errors.New("process LockRecord differs")
		}
	} else if !os.IsNotExist(readErr) {
		return readErr
	}
	if len(phases) != 0 {
		if len(phases) != 1 || !validProjectionPhase(phases[0]) {
			return errors.New("process Phase differs")
		}
		record["Phase"] = phases[0]
	}
	data, err := core.CanonicalJSON(record)
	if err == nil {
		err = ensureDirectoryNoFollow(filepath.Dir(path), 0o700)
	}
	if err == nil {
		if readErr == nil && len(phases) != 0 {
			err = publishReplace(path, data, 0o600)
		} else {
			err = publishNoReplace(path, data, 0o600)
		}
	}
	return err
}

func writeProcessGroupRecord(recordRoot, attemptID string, processGroupID int, transition string, preparation *cargoPreparation, request *runnerRequestBinding, observation *processTerminalObservation) (bool, error) {
	record := processGroupRecord{SchemaVersion: core.SchemaVersion, RecordType: "LockGroupRecord", AttemptID: attemptID, ProcessGroupID: processGroupID, Transition: transition, Preparation: preparation, Request: request, Observation: observation}
	data, err := core.CanonicalJSON(record)
	if err != nil {
		return false, err
	}
	path := filepath.Join(recordRoot, "process", attemptID, strings.ToLower(transition)+".json")
	if transition == "Add" {
		err = publishReplace(path, data, 0o600)
	} else {
		err = publishNoReplace(path, data, 0o600)
	}
	published := equalRecord(path, data)
	return published == nil, errors.Join(err, published)
}

func internalRunnerChild(capabilityFD, execStatusFD, parentPID int, nonce string, payload []string) (err error) {
	if capabilityFD != 3 || execStatusFD != 4 || parentPID != os.Getppid() || !validCapabilityNonce(nonce) || len(payload) == 0 || !filepath.IsAbs(payload[0]) {
		return errors.New("invalid internal runner child")
	}
	execStatus := os.NewFile(uintptr(execStatusFD), "exec-status")
	if execStatus == nil {
		return errors.New("missing internal runner exec status")
	}
	syscall.CloseOnExec(execStatusFD)
	defer func() {
		if err != nil {
			_, _ = io.WriteString(execStatus, err.Error())
		}
		_ = execStatus.Close()
	}()
	selfInfo, selfErr := os.Stat("/proc/self/exe")
	parentInfo, parentErr := os.Stat(filepath.Join("/proc", strconv.Itoa(parentPID), "exe"))
	if selfErr != nil || parentErr != nil || !os.SameFile(selfInfo, parentInfo) {
		return errors.New("internal runner parent differs")
	}
	capability := os.NewFile(uintptr(capabilityFD), "capability")
	if capability == nil {
		return errors.New("missing internal runner capability")
	}
	defer capability.Close()
	data, err := io.ReadAll(io.LimitReader(capability, 65))
	if err != nil || string(data) != nonce {
		return errors.New("internal runner capability differs")
	}
	if err := capability.Close(); err != nil {
		return err
	}
	return syscall.Exec(payload[0], payload, os.Environ())
}

func validateRunnerHome(codexHome string) error {
	if !filepath.IsAbs(codexHome) || filepath.Clean(codexHome) != codexHome || codexHome == "/" || strings.ContainsAny(codexHome, "\x00\r\n") {
		return errors.New("CODEX_HOME must name a clean absolute sandbox destination")
	}
	for _, target := range []string{"/bin", "/cache", "/dev", "/etc", "/harness", "/lib", "/lib64", "/proc", "/result", "/run", "/runner", "/tmp", "/toolchain", "/usr", "/workspace", "/workspace-lower"} {
		if pathsOverlap(codexHome, target) {
			return fmt.Errorf("CODEX_HOME overlaps projected mount %s", target)
		}
	}
	if codexHome == "/home/codex" || pathsOverlap(codexHome, "/home/codex") && !containedBy("/home/codex", codexHome) {
		return errors.New("CODEX_HOME must not replace the projected HOME mount")
	}
	return nil
}

func runnerEnvironment(consumer []string, codexHome string) ([]string, error) {
	if err := validateRunnerHome(codexHome); err != nil {
		return nil, err
	}
	base, err := parseAssetLines("runner environment", assets.RunnerEnvironment(), true)
	if err != nil {
		return nil, err
	}
	base = append(base, "CODEX_HOME="+codexHome)
	seen := make(map[string]struct{}, len(base)+len(consumer)+1)
	for index, row := range append(append([]string{}, base...), append(consumer, "PROJECT_NAME="+core.ProjectName)...) {
		separator := strings.IndexByte(row, '=')
		if separator < 1 || !validEnvironmentName(row[:separator]) {
			return nil, errors.New("invalid runner environment assignment")
		}
		key := row[:separator]
		if _, exists := seen[key]; exists {
			return nil, fmt.Errorf("duplicate runner environment key %s", key)
		}
		seen[key] = struct{}{}
		if index >= len(base) && !listed("PROJECT_NAME CARGO_NET_OFFLINE CCUSAGE_VERSION STABLE_GIT_COMMIT CODEX_SANDBOX_NETWORK_DISABLED CODEX_SUITE_ROUND_BUDGET_MS INSTA_FORCE_PASS INSTA_UPDATE NEXTEST_EXPERIMENTAL_LIBTEST_JSON NEXTEST_TEST_THREADS", key) {
			return nil, fmt.Errorf("unowned runner environment key %s", key)
		}
	}
	result := append(append([]string{}, base...), consumer...)
	result = append(result, "PROJECT_NAME="+core.ProjectName)
	sort.Strings(result)
	return result, nil
}

func runnerSystemArguments(networkDisabled bool) ([]string, error) {
	arguments, err := parseAssetLines("runner bwrap arguments", assets.RunnerBwrapArguments(), false)
	if err != nil || !networkDisabled {
		return arguments, err
	}
	filtered := arguments[:0]
	for _, argument := range arguments {
		if argument != "--share-net" {
			filtered = append(filtered, argument)
		}
	}
	return filtered, nil
}

func runnerSandboxArguments(request projectionRequest, paths map[string]string, toolchain ToolchainProjection, cache CacheProjection, codexHome string, systemArguments, environment []string) []string {
	arguments := append([]string{}, systemArguments...)
	arguments = append(arguments, "--ro-bind", "/etc/hosts", "/etc/hosts")
	arguments = append(arguments, "--dir", "/workspace-lower", "--ro-bind", request.Source, "/workspace-lower", "--dir", "/workspace", "--overlay-src", request.Source, "--overlay", paths["upper"], paths["work"], "/workspace")
	if request.WritableSourceRoot != "" {
		arguments = append(arguments, "--bind", filepath.Join(request.Source, request.WritableSourceRoot), filepath.ToSlash(filepath.Join("/workspace", request.WritableSourceRoot)))
	}
	arguments = append(arguments,
		"--bind", paths["harness"], "/harness", "--overlay-src", paths["tmp-lower"], "--overlay", paths["tmp-upper"], paths["tmp-work"], "/tmp", "--bind", paths["run"], "/run", "--bind", paths["home"], "/home/codex",
		"--bind", paths["codex"], codexHome,
		"--bind", paths["result"], "/result", "--bind", paths["shm"], "/dev/shm",
		"--dir", "/workspace/codex-rs/target", "--bind", filepath.Join(paths["result"], "nextest"), "/workspace/codex-rs/target/nextest",
		"--dir", "/cache", "--bind", cache.CargoHome, "/cache/cargo", "--bind", cache.CargoBuild, "/cache/build", "--bind", cache.CargoTarget, "/cache/target", "--bind", cache.SccacheDirectory, "/cache/sccache",
		"--ro-bind", toolchain.Path, "/toolchain", "--ro-bind", paths["runner"], "/runner",
		"--chdir", filepath.ToSlash(filepath.Join("/workspace", request.WorkingDirectory)), "/usr/bin/env", "-i")
	arguments = append(arguments, environment...)
	arguments = append(arguments, "/runner/runner-stage.zsh")
	arguments = append(arguments, request.Arguments...)
	return arguments
}

func validateSnapshotTargetSpans(data []byte, target snapshotTarget) error {
	previousEnd := 0
	for index, span := range target.ExistingInlineSpans {
		if span.Start < previousEnd || span.End <= span.Start || span.End > len(data) || span.Line <= 0 || span.Preimage != string(data[span.Start:span.End]) {
			return fmt.Errorf("snapshot %s span %d differs", target.Path, index)
		}
		previousEnd = span.End
	}
	return nil
}

func rebuildSnapshotUpdate(source []byte, update snapshotUpdate) ([]byte, bool) {
	if update.Target.Kind != "InlineRust" || len(update.Target.ExistingInlineSpans) == 0 || string(source) != update.Target.Current || validateSnapshotTargetSpans(source, update.Target) != nil || len(update.GeneratedInlineSpans) == 0 {
		return nil, false
	}
	targets := make(map[[3]int]snapshotTargetSpan, len(update.Target.ExistingInlineSpans))
	for _, target := range update.Target.ExistingInlineSpans {
		targets[[3]int{target.Start, target.End, target.Line}] = target
	}
	previousEnd := 0
	for _, span := range update.GeneratedInlineSpans {
		_, exists := targets[[3]int{span.Start, span.End, span.Line}]
		if !exists || span.Start < previousEnd {
			return nil, false
		}
		previousEnd = span.End
	}
	postimage := append([]byte{}, source...)
	for index := len(update.GeneratedInlineSpans) - 1; index >= 0; index-- {
		span := update.GeneratedInlineSpans[index]
		targetSpan := targets[[3]int{span.Start, span.End, span.Line}]
		if string(postimage[targetSpan.Start:targetSpan.End]) != targetSpan.Preimage {
			return nil, false
		}
		next := make([]byte, 0, len(postimage)-targetSpan.End+targetSpan.Start+len(span.Postimage))
		next = append(next, postimage[:targetSpan.Start]...)
		next = append(next, span.Postimage...)
		next = append(next, postimage[targetSpan.End:]...)
		postimage = next
	}
	if string(postimage) != update.Generated {
		return nil, false
	}
	return postimage, true
}

func collectSnapshotUpdates(request projectionRequest, upper string, updates []snapshotUpdate) error {
	remaining := make(map[string]snapshotUpdate, len(updates))
	for _, update := range updates {
		remaining[update.Target.Path] = update
	}
	err := filepath.WalkDir(upper, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if err := request.window.check(); err != nil {
			return err
		}
		if path == upper || entry.IsDir() {
			return nil
		}
		relative, err := filepath.Rel(upper, path)
		if err != nil || !filepath.IsLocal(relative) {
			return errors.Join(err, errors.New("projection source postimage path differs"))
		}
		if !entry.Type().IsRegular() {
			return snapshotOutputError{Path: filepath.ToSlash(relative), error: errors.New("non-regular source output")}
		}
		if slices.Contains(request.Consumer.ephemeralSourcePaths, relative) {
			return nil
		}
		data, err := readRegularNoFollow(path, request.window)
		if err != nil {
			return err
		}
		name := filepath.Base(relative)
		inline := strings.HasPrefix(name, ".") && strings.HasSuffix(name, ".rs.pending-snap")
		target := relative
		if inline {
			target = filepath.Join(filepath.Dir(relative), strings.TrimSuffix(name[1:], ".pending-snap"))
		} else if strings.HasSuffix(relative, ".snap.new") {
			target = strings.TrimSuffix(relative, ".new")
		} else {
			before, readErr := readRegularNoFollow(filepath.Join(request.Source, relative), request.window)
			if readErr == nil && bytes.Equal(before, data) {
				return nil
			}
		}
		update, declared := remaining[target]
		if !declared || inline && update.Target.Kind != "InlineRust" || !inline && (update.Target.Kind != "ExternalSnapshot" || string(data) != update.Generated) {
			return snapshotOutputError{Path: filepath.ToSlash(target), error: errors.New("source output differs from native snapshot input")}
		}
		delete(remaining, target)
		return nil
	})
	if err == nil && len(remaining) != 0 {
		err = errors.New("snapshot manifest input is absent from overlay")
	}
	return err
}

func decodeSnapshotManifest(data []byte) ([]snapshotUpdate, error) {
	var manifest struct {
		Updates []snapshotUpdate
	}
	if err := validateJSONUniqueKeys(data); err != nil {
		return nil, err
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&manifest); err != nil {
		return nil, err
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return nil, errors.Join(err, errors.New("snapshot manifest has trailing content"))
	}
	wrapped, err := core.CanonicalJSON(struct{ Updates []snapshotUpdate }{Updates: manifest.Updates})
	if err != nil || !bytes.Equal(data, wrapped) {
		return nil, errors.Join(err, errors.New("snapshot manifest encoding differs"))
	}
	return manifest.Updates, nil
}

func completeSnapshotUpdates(source string, data []byte) ([]snapshotUpdate, error) {
	updates, err := decodeSnapshotManifest(data)
	if err != nil {
		return nil, err
	}
	previous := ""
	for _, update := range updates {
		target := update.Target
		pathValid := target.Path > previous && strings.HasPrefix(target.Path, "codex-rs/") && filepath.IsLocal(target.Path) && filepath.Clean(target.Path) == target.Path && !strings.ContainsAny(target.Path, "\\\r\n\x00")
		inline := target.Kind == "InlineRust" && strings.HasSuffix(target.Path, ".rs") && len(target.ExistingInlineSpans) != 0
		external := target.Kind == "ExternalSnapshot" && strings.Contains(target.Path, "/snapshots/") && strings.HasSuffix(target.Path, ".snap") && len(target.ExistingInlineSpans) == 0
		if !pathValid || !inline && !external || update.Generated == target.Current {
			return nil, fmt.Errorf("snapshot %s: generated target kind, order, or postimage differs", target.Path)
		}
		preimage, readErr := readRegularNoFollow(filepath.Join(source, target.Path))
		if external && os.IsNotExist(readErr) && target.Current == "" {
			readErr = nil
			preimage = nil
		}
		if readErr != nil || string(preimage) != target.Current {
			return nil, errors.Join(readErr, fmt.Errorf("snapshot %s: lower preimage differs", target.Path))
		}
		if inline {
			rebuilt, valid := rebuildSnapshotUpdate(preimage, update)
			if !valid || !bytes.Equal(rebuilt, []byte(update.Generated)) {
				return nil, fmt.Errorf("snapshot %s: inline reconstruction differs", target.Path)
			}
		} else if len(update.GeneratedInlineSpans) != 0 || update.Generated == "" {
			return nil, fmt.Errorf("snapshot %s: external postimage differs", target.Path)
		}
		previous = target.Path
	}
	return updates, nil
}

func validateCompleteSnapshotUpdates(source string, updates []snapshotUpdate) ([]snapshotUpdate, error) {
	data, err := core.CanonicalJSON(struct{ Updates []snapshotUpdate }{Updates: updates})
	if err != nil {
		return nil, err
	}
	return completeSnapshotUpdates(source, data)
}

func publishProjectionFiles(request projectionRequest, resultRoot string, names []string, result *projectionResult) error {
	if err := request.window.check(); err != nil {
		return err
	}
	seen := make(map[string]bool, len(names))
	result.Files = make(map[string]projectionFile, len(names))
	var outputErr error
	var outputRootErr error
	if request.OutputRoot != "" {
		expectedTemporary := projectionOutputRoot(request.ProjectWorkingRoot, request.AttemptIdentity)
		validRoot := request.OutputRoot == expectedTemporary || request.RunnerRequest == nil && !request.Consumer.rustSuite && request.OutputRoot == projectionPublishedOutputRoot(request.ProjectWorkingRoot, request.AttemptIdentity)
		if !validRoot || noFollowPath("projection output root", request.OutputRoot) != nil {
			outputRootErr = errors.New("projection output root differs")
		} else {
			outputRootErr = ensureDirectoryNoFollow(request.OutputRoot, 0o700)
		}
	}
	for _, name := range names {
		if name == "" || filepath.Base(name) != name || seen[name] {
			return fmt.Errorf("invalid declared projection output: %s", name)
		}
		seen[name] = true
		path := filepath.Join(resultRoot, name)
		info, err := os.Lstat(path)
		if os.IsNotExist(err) {
			result.NativeAbsentOutputs = append(result.NativeAbsentOutputs, name)
			if !result.ExecutionStarted || result.TimedOut || result.Canceled {
				continue
			}
			outputErr = errors.Join(outputErr, fmt.Errorf("declared projection output is absent: %s", name))
			continue
		}
		if err != nil || !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 {
			result.FailedOutputs = append(result.FailedOutputs, name)
			outputErr = errors.Join(outputErr, err, fmt.Errorf("projection result differs: %s", name))
			continue
		}
		if name == "runner.stdout.log" || name == "runner.stderr.log" {
			expected := request.stagePaths(request.AttemptRoot)["result"]
			if resultRoot != expected {
				result.FailedOutputs = append(result.FailedOutputs, name)
				outputErr = errors.Join(outputErr, errors.New("raw runner output is outside its temporary root"))
				continue
			}
			result.Files[name] = projectionFile{Path: path}
			continue
		}
		if request.OutputRoot != "" {
			if request.OutputRoot == resultRoot && outputRootErr == nil {
				result.Files[name] = projectionFile{Path: path}
				continue
			}
			data, err := readRegularNoFollow(path, request.window)
			if err != nil || outputRootErr != nil {
				result.FailedOutputs = append(result.FailedOutputs, name)
				outputErr = errors.Join(outputErr, err, outputRootErr)
				continue
			}
			published := filepath.Join(request.OutputRoot, name)
			if err := publishNoReplace(published, data, 0o600); err != nil {
				result.FailedOutputs = append(result.FailedOutputs, name)
				outputErr = errors.Join(outputErr, err)
				continue
			}
			result.Files[name] = projectionFile{Path: published}
			continue
		}
		result.Files[name] = projectionFile{Path: path}
	}
	directoryFile, err := os.OpenFile(resultRoot, os.O_RDONLY|syscall.O_NOFOLLOW, 0)
	if err != nil {
		result.FailedOutputs = append(result.FailedOutputs, "result-root")
		outputErr = errors.Join(outputErr, err)
	} else if syncErr := errors.Join(directoryFile.Sync(), directoryFile.Close()); syncErr != nil {
		result.FailedOutputs = append(result.FailedOutputs, "result-root")
		outputErr = errors.Join(outputErr, syncErr)
	}
	return errors.Join(outputErr, outputRootErr, request.window.check())
}

func (writer *boundedWriter) Write(data []byte) (int, error) {
	original := len(data)
	allowed := int64(original)
	if allowed > writer.remaining {
		allowed = writer.remaining
		writer.truncated = true
	}
	if allowed > 0 {
		written, err := writer.writer.Write(data[:allowed])
		writer.remaining -= int64(written)
		if err != nil {
			return written, err
		}
		if int64(written) != allowed {
			return written, io.ErrShortWrite
		}
	}
	return original, nil
}

func waitProcessGroup(command *exec.Cmd, deadline, grace time.Duration, cancellation <-chan os.Signal, roundDeadlines ...<-chan time.Time) (error, bool, bool) {
	wait := make(chan error, 1)
	go func() { wait <- command.Wait() }()
	var phases <-chan time.Time
	if len(roundDeadlines) != 0 {
		phases = roundDeadlines[0]
	}
	currentEnd := time.Now().Add(deadline)
	timer := time.NewTimer(deadline)
	defer timer.Stop()
	timedOut, canceled := false, false
	var err error
	for {
		select {
		case err = <-wait:
			goto complete
		case next := <-phases:
			if !time.Now().Before(currentEnd) || !timer.Stop() {
				timedOut = true
				goto complete
			}
			currentEnd = next
			timer.Reset(time.Until(next))
		case <-timer.C:
			timedOut = true
			goto complete
		case <-cancellation:
			canceled = true
			goto complete
		}
	}
complete:
	if timedOut || canceled {
		select {
		case err = <-wait:
			return err, timedOut, canceled
		default:
		}
		_ = syscall.Kill(-command.Process.Pid, syscall.SIGTERM)
		if grace <= 0 {
			_ = syscall.Kill(-command.Process.Pid, syscall.SIGKILL)
			return <-wait, timedOut, canceled
		}
		select {
		case err = <-wait:
		case <-time.After(grace):
			_ = syscall.Kill(-command.Process.Pid, syscall.SIGKILL)
			err = <-wait
		}
	}
	return err, timedOut, canceled
}

func cleanupSccacheSocket(path string) error {
	if removeErr := os.Remove(path); removeErr != nil && !os.IsNotExist(removeErr) {
		return fmt.Errorf("remove sccache socket: %w", removeErr)
	}
	if _, socketErr := os.Lstat(path); !os.IsNotExist(socketErr) {
		return errors.New("sccache socket remains after stage reap")
	}
	return nil
}

func parseAssetLines(name string, data []byte, assignments bool) ([]string, error) {
	if len(data) == 0 || data[len(data)-1] != '\n' {
		return nil, fmt.Errorf("%s must be nonempty with final LF", name)
	}
	lines := strings.Split(string(data[:len(data)-1]), "\n")
	seenAssignments := make(map[string]struct{})
	for _, line := range lines {
		if line == "" || strings.TrimSpace(line) != line || strings.ContainsRune(line, '\x00') {
			return nil, fmt.Errorf("%s contains an invalid line", name)
		}
		if assignments {
			separator := strings.IndexByte(line, '=')
			if separator < 1 || !validEnvironmentName(line[:separator]) {
				return nil, fmt.Errorf("%s contains an invalid assignment", name)
			}
			if _, exists := seenAssignments[line[:separator]]; exists {
				return nil, fmt.Errorf("%s contains a duplicate assignment", name)
			}
			seenAssignments[line[:separator]] = struct{}{}
		}
	}
	return lines, nil
}

func validEnvironmentName(name string) bool {
	for index, character := range name {
		if (character >= 'A' && character <= 'Z') || (character >= 'a' && character <= 'z') || character == '_' || (index > 0 && character >= '0' && character <= '9') {
			continue
		}
		return false
	}
	return name != ""
}
