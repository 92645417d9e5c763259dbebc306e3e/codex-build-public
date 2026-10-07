package runtime

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"time"

	"codex-build/internal/core"
)

func RunCLI(arguments []string, stdout, stderr io.Writer) int {
	if len(arguments) == 0 {
		fmt.Fprintln(stderr, "required")
		return 2
	}
	switch arguments[0] {
	case "sync":
		return runSync(arguments[1:], stdout, stderr)
	case "build-install":
		return runBuildInstall(arguments[1:], stdout, stderr)
	case "native-toolchain":
		return runNativeToolchain(arguments[1:], stdout, stderr)
	case "runner-exec":
		return runRunnerExec(arguments[1:], stdout, stderr)
	case "repository-format":
		return runRepositoryFormat(arguments[1:], stdout, stderr)
	case "repository-maintenance":
		return runRepositoryMaintenance(arguments[1:], stdout, stderr)
	case "runner-child-exec":
		return runRunnerChild(arguments[1:], stderr)
	case "patch-replay":
		return runPatchReplay(arguments[1:], stdout, stderr)
	case "release-baseline":
		return runRelease(arguments[1:], "release-baseline", "Baseline", RunReleaseBaseline, stdout, stderr)
	case "release-cleanup":
		return runRelease(arguments[1:], "release-cleanup", "Cleanup", RunReleaseCleanup, stdout, stderr)
	case "baseline-coverage":
		return runBaselineCoverage(arguments[1:], stdout, stderr)
	default:
		fmt.Fprintln(stderr, "unsupported")
		return 2
	}
}

func runRelease(arguments []string, command, route string, invoke func(ReleaseBaselineOptions) (ReleaseRouteResult, error), stdout, stderr io.Writer) int {
	names := []string{"project-working-root", "project-scratch-root", "runtime-tmp-root", "record-root", "tag"}
	endpoint, values, ok := parseEndpointStrings(command, arguments, stderr, names...)
	if !ok {
		return 2
	}
	options := ReleaseBaselineOptions{endpointOptions: endpoint, WorkingRoot: values[0], ScratchRoot: values[1], RuntimeRoot: values[2], RecordRoot: values[3], Tag: values[4]}
	return runReleaseOperation(&options, route, command, func() (ReleaseRouteResult, error) { return invoke(options) }, stdout, stderr)
}

func runBaselineCoverage(arguments []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("baseline-coverage", flag.ContinueOnError)
	flags.SetOutput(stderr)
	options := BaselineCoverageOptions{Proposal: rustSuiteBaselineProposal{CompileOnlyPackages: []string{}, CoveredPackages: []string{}, HelperBinaries: []string{}}}
	bindPublicRoots(flags, &options.endpointOptions, &options.WorkingRoot, &options.ScratchRoot, &options.RuntimeRoot, &options.RecordRoot)
	flags.StringVar(&options.Tag, "tag", "", "")
	flags.StringVar(&options.Unit, "unit", "", "")
	flags.StringVar(&options.FlatPatch, "flat-patch", "", "")
	flags.BoolVar(&options.Align, "align", false, "")
	flags.Var((*repeatedStrings)(&options.Proposal.CompileOnlyPackages), "compile-only-package", "")
	flags.Var((*repeatedStrings)(&options.Proposal.HelperBinaries), "helper-binary", "")
	flags.Var((*repeatedStrings)(&options.Proposal.CoveredPackages), "covered-package", "")
	if parsePublicFlags(flags, arguments) != nil || flags.NArg() != 0 {
		return 2
	}
	return runReleaseOperation(&options.ReleaseBaselineOptions, "RollCoverage", "baseline-coverage", func() (ReleaseRouteResult, error) { return RunBaselineCoverage(options) }, stdout, stderr)
}

type repeatedStrings []string

func (values *repeatedStrings) String() string { return strings.Join(*values, ",") }

func (values *repeatedStrings) Set(value string) error {
	*values = append(*values, value)
	return nil
}

func runReleaseOperation(options *ReleaseBaselineOptions, route, operation string, invoke func() (ReleaseRouteResult, error), stdout, stderr io.Writer) int {
	failure := releaseRouteResult(route, options.Tag).terminal("Fail", failureDomainHelper, "None")
	if err := admitPublicRoute(&options.endpointOptions, "ReleaseBaselineSupport"); err != nil {
		return writeResult(failure, failure.Terminal, err, stdout, stderr)
	}
	cancellation, releaseController, err := controllerSignals()
	if err != nil {
		return writeResult(failure, failure.Terminal, err, stdout, stderr)
	}
	defer releaseController()
	options.Cancellation = cancellation
	notice := newWaitNotice(stderr, operation)
	options.OnPhase = notice.Phase
	result, err := invoke()
	return writeResult(result, result.Terminal, err, stdout, stderr)
}

func runNativeToolchain(arguments []string, stdout, stderr io.Writer) int {
	options, values, ok := parseEndpointStrings("native-toolchain", arguments, stderr, "project-working-root", "project-scratch-root", "runtime-tmp-root")
	if !ok {
		return 2
	}
	request := NativeToolchainOptions{endpointOptions: options, WorkingRoot: values[0], ScratchRoot: values[1], RuntimeRoot: values[2]}
	result := NativeToolchainResult{SchemaVersion: core.SchemaVersion, RecordType: "NativeToolchainResult", Terminal: "Blocked", FailureDomain: failureDomainInput, ProductEffect: "None"}
	if err := admitPublicRoute(&request.endpointOptions, "NativeToolchainSupport"); err != nil {
		result.Terminal, result.FailureDomain = "Fail", failureDomainHelper
		return writeResult(result, result.Terminal, err, stdout, stderr)
	}
	newWaitNotice(stderr, "native-toolchain")
	result, err := prepareNativeToolchain(request)
	return writeResult(result, result.Terminal, err, stdout, stderr)
}

func runPatchReplay(arguments []string, stdout, stderr io.Writer) int {
	if len(arguments) == 1 && arguments[0] == "--help" {
		_, _ = parsePatchReplayOptions(arguments, stdout)
		return 0
	}
	options, err := parsePatchReplayOptions(arguments, stderr)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	result := PatchReplayResult{SchemaVersion: core.SchemaVersion, RecordType: "PatchReplayResult", Mode: options.Mode, Terminal: "Blocked", FailureDomain: failureDomainInput, ProductEffect: "None", AppliedRows: []PatchReplayRow{}, Rows: []PatchReplayRow{}}
	if err := admitPublicRoute(&options.endpointOptions, "PatchSupport"); err != nil {
		result.Terminal, result.FailureDomain = "Fail", failureDomainHelper
		return writeResult(result, result.Terminal, err, stdout, stderr)
	}
	notice := newWaitNotice(stderr, "patch-replay "+options.Mode)
	options.OnPhase = notice.Phase
	result, err = executePatchReplay(options)
	return writeResult(result, result.Terminal, err, stdout, stderr)
}

func parsePatchReplayOptions(arguments []string, stderr io.Writer) (PatchReplayOptions, error) {
	flags := flag.NewFlagSet("patch-replay", flag.ContinueOnError)
	flags.SetOutput(stderr)
	options := PatchReplayOptions{}
	bindPublicRoots(flags, &options.endpointOptions, &options.WorkingRoot, &options.ScratchRoot, &options.RuntimeRoot, &options.RecordRoot)
	bind := func(target *string, name string) { flags.StringVar(target, name, "", "") }
	bind(&options.Mode, "mode")
	bind(&options.Source, "source")
	bind(&options.Target, "target")
	bind(&options.Tag, "tag")
	bind(&options.StartUnit, "start-unit")
	bind(&options.EndUnit, "end-unit")
	bind(&options.StartFlatPatch, "start-flat-patch")
	bind(&options.EndFlatPatch, "end-flat-patch")
	bind(&options.StartTransform, "start-transform")
	bind(&options.EndTransform, "end-transform")
	bind(&options.Lane, "lane")
	bind(&options.Unit, "unit")
	bind(&options.ChildPatch, "child-patch")
	bind(&options.FlatPatch, "flat-patch")
	bind(&options.Form, "form")
	bind(&options.Segment, "segment")
	flags.BoolVar(&options.SkipLineLimit, "skip-line-limit", false, "")
	bind(&options.ParentFork, "parent-fork")
	bind(&options.ActiveTask, "active-task")
	flags.BoolVar(&options.Resume, "resume", false, "continue the current full execution without renewing its budget")
	common := "active-task mode project-scratch-root runtime-tmp-root skill-root tag"
	modes := map[string]string{
		"fork":          common + " parent-fork",
		"prepare":       common + " target",
		"list":          common + " project-working-root record-root source",
		"apply":         common + " end-flat-patch end-transform end-unit project-working-root record-root source start-flat-patch start-transform start-unit target",
		"revise":        common + " child-patch flat-patch form lane project-working-root record-root segment target unit",
		"finalize":      common + " lane project-working-root record-root resume target",
		"config-schema": common + " project-working-root record-root target",
		"commit":        common + " child-patch flat-patch form lane project-working-root record-root segment skip-line-limit target unit",
		"export":        common + " project-working-root record-root skip-line-limit",
	}
	flags.Usage = func() {
		fmt.Fprintln(stderr, "patch-replay --mode MODE --active-task TASK --tag TAG [flags]; roots default to cwd and the declared environment")
		fmt.Fprintln(stderr, "Mode-specific allowed flags:")
		for _, mode := range strings.Fields("apply commit config-schema export finalize fork list prepare revise") {
			fmt.Fprintln(stderr, mode+": --"+strings.ReplaceAll(strings.TrimPrefix(modes[mode], common+" "), " ", " --"))
		}
		flags.PrintDefaults()
		fmt.Fprintln(stderr, "Example: patch-replay --mode prepare --active-task TASK --tag TAG --target CURRENT")
	}
	if parsePublicFlags(flags, arguments) != nil || flags.NArg() != 0 {
		return options, errors.New("invalid patch replay arguments")
	}
	allowed := modes[options.Mode]
	if allowed == "" {
		return options, errors.New("invalid patch replay mode")
	}
	var forbidden string
	flags.Visit(func(option *flag.Flag) {
		if forbidden == "" && !listed(allowed, option.Name) {
			forbidden = option.Name
		}
	})
	if forbidden != "" {
		return options, errors.New(options.Mode + " forbids --" + forbidden)
	}
	if options.ActiveTask == "" {
		return options, errors.New("patch replay --active-task is required")
	}
	return options, validatePatchReplayOptions(options)
}

func runBuildInstall(arguments []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("build-install", flag.ContinueOnError)
	flags.SetOutput(stderr)
	options := BuildInstallOptions{}
	bindPublicRoots(flags, &options.endpointOptions, &options.WorkingRoot, &options.ScratchRoot, &options.RuntimeRoot, &options.RecordRoot)
	bind := func(target *string, name string) { flags.StringVar(target, name, "", "") }
	bind(&options.CodexHome, "codex-home")
	bind(&options.Tag, "tag")
	bind(&options.BuildVersion, "build-version")
	for index, name := range []string{"rg", "bwrap"} {
		bind(&options.ToolPaths[index], name+"-path")
	}
	flags.Uint64Var(&options.BuildSequence, "build-sequence", 0, "")
	flags.BoolVar(&options.InstallRequested, "install", false, "")
	flags.DurationVar(&options.Deadline, "deadline", 0, "")
	if err := parsePublicFlags(flags, arguments); errors.Is(err, flag.ErrHelp) {
		return 0
	} else if err != nil || flags.NArg() != 0 {
		return 2
	}
	if err := admitPublicRoute(&options.endpointOptions, "BuildSupport"); err != nil {
		result := BuildInstallResult{SchemaVersion: core.SchemaVersion, RecordType: "BuildInstallLocalResult", Tag: options.Tag, Terminal: "Fail", ProductEffect: "None", FailureDomain: failureDomainHelper, BuildSequence: options.BuildSequence, BuildVersion: options.BuildVersion}
		return writeResult(result, result.Terminal, err, stdout, stderr)
	}
	cancellation, releaseController, err := controllerSignals()
	if err != nil {
		result := BuildInstallResult{SchemaVersion: core.SchemaVersion, RecordType: "BuildInstallLocalResult", Tag: options.Tag, Terminal: "Fail", ProductEffect: "None", FailureDomain: failureDomainHelper, BuildSequence: options.BuildSequence, BuildVersion: options.BuildVersion}
		return writeResult(result, result.Terminal, err, stdout, stderr)
	}
	defer releaseController()
	options.Cancellation = cancellation
	newWaitNotice(stderr, "build-install")
	result, err := RunBuildInstall(options)
	return writeResult(result, result.Terminal, err, stdout, stderr)
}

type endpointOptions struct {
	SkillRoot string
	admission *publicAdmission
}

func bindPublicRoots(flags *flag.FlagSet, options *endpointOptions, working, scratch, runtime, records *string) {
	cwd, _ := os.Getwd()
	for _, root := range []struct {
		name, value, usage string
		target             *string
	}{
		{"skill-root", filepath.Join(os.Getenv("SKILLS_ROOT"), "codex-build"), "installed skill root", &options.SkillRoot},
		{"project-working-root", cwd, "working root (cwd)", working},
		{"project-scratch-root", os.Getenv("PROJECT_SCRATCH_ROOT"), "project scratch root", scratch},
		{"runtime-tmp-root", os.Getenv("PROJECT_RUNTIME_ROOT"), "disposable runtime root", runtime},
		{"record-root", "", "defaults to effective project scratch root/records", records},
	} {
		if root.target != nil {
			flags.StringVar(root.target, root.name, root.value, root.usage)
		}
	}
}

func parsePublicFlags(flags *flag.FlagSet, arguments []string) error {
	if err := flags.Parse(arguments); err != nil {
		return err
	}
	if records := flags.Lookup("record-root"); records != nil {
		supplied := false
		flags.Visit(func(option *flag.Flag) { supplied = supplied || option.Name == "record-root" })
		if scratch := flags.Lookup("project-scratch-root").Value.String(); !supplied && scratch != "" {
			return records.Value.Set(filepath.Join(scratch, "records"))
		}
	}
	return nil
}

func parseEndpointStrings(command string, arguments []string, stderr io.Writer, names ...string) (endpointOptions, []string, bool) {
	flags, options, values := flag.NewFlagSet(command, flag.ContinueOnError), endpointOptions{}, make([]string, len(names))
	flags.SetOutput(stderr)
	var records *string
	if len(names) > 3 {
		records = &values[3]
		flags.StringVar(&values[4], names[4], "", "")
	}
	bindPublicRoots(flags, &options, &values[0], &values[1], &values[2], records)
	return options, values, parsePublicFlags(flags, arguments) == nil && flags.NArg() == 0
}

func runRunnerExec(arguments []string, stdout, stderr io.Writer) int {
	if len(arguments) == 1 && arguments[0] == "--help" {
		_, _ = parseRunnerExecOptions(arguments, stdout)
		return 0
	}
	options, err := parseRunnerExecOptions(arguments, io.Discard)
	if err == nil {
		err = validateRunnerExecOptions(options)
	}
	if err != nil {
		options.RecordRoot = ""
		return writeRunnerFailure(options, failureDomainInput, err, stdout, stderr)
	}
	if err := admitPublicRoute(&options.endpointOptions, "RunnerSupport"); err != nil {
		return writeRunnerFailure(options, failureDomainHelper, err, stdout, stderr)
	}
	cancellation, releaseController, err := controllerSignals()
	if err != nil {
		return writeRunnerFailure(options, failureDomainHelper, err, stdout, stderr)
	}
	defer releaseController()
	options.Cancellation = cancellation
	projection, projectionErr := freezeSourceToolchainProjection(options.Source, options.ProjectWorkingRoot, options.ProjectScratchRoot, options.RuntimeTemporaryRoot, options.Release)
	if projectionErr != nil {
		return writeRunnerFailure(options, runnerFailureDomain(projectionErr), projectionErr, stdout, stderr)
	}
	if err := bindRunnerTestSelection(&options, projection.Toolchain); err != nil {
		return writeRunnerFailure(options, runnerFailureDomain(err), err, stdout, stderr)
	}
	if err := validateRunnerExecOptions(options); err != nil {
		return writeRunnerFailure(options, failureDomainInput, err, stdout, stderr)
	}
	if repositoryTestArguments(options.Arguments) {
		head, err := repositoryGitOutput(options.Source, "rev-parse", "HEAD")
		options.sourceCommit = strings.TrimSpace(string(head))
		if err != nil || !validGitCommit(options.sourceCommit) {
			return writeRunnerFailure(options, failureDomainHelper, errors.Join(err, errors.New("test source commit differs")), stdout, stderr)
		}
	}
	newWaitNotice(stderr, "runner-exec")
	options.replaceCurrent = true
	result, err := executeRunner(options)
	return writeRunnerResult(options, result, err, stdout, stderr)
}

func writeRunnerFailure(options RunnerExecOptions, domain string, err error, stdout, stderr io.Writer) int {
	return writeRunnerResult(options, RunnerExecResult{FailureDomain: domain, PreExecutionFailure: err.Error(), Terminal: "Blocked"}, nil, stdout, stderr)
}

func writeRunnerResult(options RunnerExecOptions, result RunnerExecResult, runErr error, stdout, stderr io.Writer) int {
	diagnostic := result.PreExecutionFailure
	if diagnostic == "" && runErr != nil {
		diagnostic = runErr.Error()
	}
	bounded := func(value string) string { return value[:min(len(value), 4096)] }
	evidence := []string{}
	if result.PreExecutionFailure == "" {
		evidence = []string{bounded(runnerTerminalPath(options)), bounded(filepath.Join(options.RecordRoot, "process", options.AttemptID))}
	}
	if info, err := os.Lstat(runnerOutputRoot(options)); err == nil && info.IsDir() {
		evidence = append(evidence, bounded(runnerOutputRoot(options)))
	}
	public := map[string]any{"Terminal": result.Terminal, "FailureDomain": result.FailureDomain, "FailureClass": result.FailureClass, "FailureSummary": result.FailureSummary, "Diagnostic": bounded(diagnostic), "EvidencePaths": evidence}
	return writeResult(public, result.Terminal, nil, stdout, stderr)
}

func runRepositoryFormat(arguments []string, stdout, stderr io.Writer) int {
	if len(arguments) == 1 && arguments[0] == "--help" {
		_, _ = parseRepositoryWorktreeOptions("repository-format", arguments, stdout, nil)
		return 0
	}
	common, ok := parseRepositoryWorktreeOptions("repository-format", arguments, stderr, nil)
	if !ok {
		return 2
	}
	options := RepositoryFormatOptions{repositoryWorktreeOptions: common}
	result := repositoryFormatResult(options)
	if err := validateRepositoryFormatOptions(options); err != nil {
		return writeResult(result, result.Terminal, err, stdout, stderr)
	}
	if err := admitPublicRoute(&options.endpointOptions, "RepositoryFormattingSupport"); err != nil {
		result.FailureDomain = failureDomainHelper
		return writeResult(result, result.Terminal, err, stdout, stderr)
	}
	cancellation, releaseController, err := controllerSignals()
	if err != nil {
		result.FailureDomain = failureDomainHelper
		return writeResult(result, result.Terminal, err, stdout, stderr)
	}
	defer releaseController()
	options.Cancellation = cancellation
	newWaitNotice(stderr, "repository-format")
	result, err = RunRepositoryFormat(options)
	return writeResult(result, result.Terminal, err, stdout, stderr)
}

func runRepositoryMaintenance(arguments []string, stdout, stderr io.Writer) int {
	updateCargoLock := false
	common, ok := parseRepositoryWorktreeOptions("repository-maintenance", arguments, stderr, &updateCargoLock)
	if !ok {
		return 2
	}
	options := RepositoryMaintenanceOptions{repositoryWorktreeOptions: common, UpdateCargoLock: updateCargoLock}
	result := repositoryMaintenanceResult(options)
	if err := validateRepositoryWorktreeOptions(common); err != nil {
		return writeResult(result, result.Terminal, err, stdout, stderr)
	}
	if err := admitPublicRoute(&options.endpointOptions, "RepositoryMaintenanceSupport"); err != nil {
		result.FailureDomain = failureDomainHelper
		return writeResult(result, result.Terminal, err, stdout, stderr)
	}
	cancellation, releaseController, err := controllerSignals()
	if err != nil {
		result.FailureDomain = failureDomainHelper
		return writeResult(result, result.Terminal, err, stdout, stderr)
	}
	defer releaseController()
	options.Cancellation = cancellation
	newWaitNotice(stderr, "repository-maintenance")
	result, err = RunRepositoryMaintenance(options)
	return writeResult(result, result.Terminal, err, stdout, stderr)
}

func parseRepositoryWorktreeOptions(command string, arguments []string, stderr io.Writer, updateCargoLock *bool) (repositoryWorktreeOptions, bool) {
	flags := flag.NewFlagSet(command, flag.ContinueOnError)
	flags.SetOutput(stderr)
	if updateCargoLock == nil {
		flags.Usage = func() {
			fmt.Fprintln(stderr, "repository-format [required flags]: run the repository's fmt and fmt-check recipes")
			flags.PrintDefaults()
			fmt.Fprintln(stderr, "Example: repository-format --source WORKTREE --release TAG --attempt-id ID --deadline 25m")
		}
	}
	options := repositoryWorktreeOptions{}
	bindPublicRoots(flags, &options.endpointOptions, &options.ProjectWorkingRoot, &options.ProjectScratchRoot, &options.RuntimeTemporaryRoot, nil)
	flags.StringVar(&options.Source, "source", "", "selected worktree")
	flags.StringVar(&options.Release, "release", "", "release tag")
	flags.StringVar(&options.AttemptID, "attempt-id", "", "attempt identity")
	flags.DurationVar(&options.Deadline, "deadline", 0, "execution deadline")
	if updateCargoLock != nil {
		flags.BoolVar(updateCargoLock, "cargo-lock", false, "")
	}
	return options, parsePublicFlags(flags, arguments) == nil && flags.NArg() == 0
}

func parseRunnerExecOptions(arguments []string, output io.Writer) (RunnerExecOptions, error) {
	separator := argumentSeparator(arguments)
	if separator == len(arguments)-1 {
		return RunnerExecOptions{}, fmt.Errorf("missing runner argv")
	}
	flagEnd := separator
	if flagEnd < 0 {
		flagEnd = len(arguments)
	}
	flags := flag.NewFlagSet("runner-exec", flag.ContinueOnError)
	flags.SetOutput(output)
	flags.Usage = func() {
		fmt.Fprintln(output, "runner-exec [roots, source, release, deadline] (--test-source FILE --test-name NAME [--test-name NAME ...] | -- ARGV...); replaces the current focused evidence")
		flags.PrintDefaults()
	}
	options := RunnerExecOptions{AttemptID: "focused"}
	bindPublicRoots(flags, &options.endpointOptions, &options.ProjectWorkingRoot, &options.ProjectScratchRoot, &options.RuntimeTemporaryRoot, &options.RecordRoot)
	flags.StringVar(&options.Source, "source", "", "")
	flags.StringVar(&options.Release, "release", "", "")
	flags.StringVar(&options.WorkingDirectory, "working-directory", "", "")
	flags.StringVar(&options.TestSource, "test-source", "", "source-relative Cargo target root")
	flags.Func("test-name", "exact qualified native test name; repeat for the same target", func(name string) error {
		options.TestNames = append(options.TestNames, name)
		return nil
	})
	packageGate := flags.Bool("package-gate", false, "explicit package test")
	flags.BoolVar(&options.CompleteSuite, "complete-suite", false, "explicit complete test")
	flags.DurationVar(&options.Deadline, "deadline", 0, "")
	if err := parsePublicFlags(flags, arguments[:flagEnd]); err != nil {
		return options, err
	}
	if flags.NArg() != 0 {
		return options, fmt.Errorf("misplaced runner flag")
	}
	if separator >= 0 {
		options.Arguments = append([]string{}, arguments[separator+1:]...)
	}
	options.WorkingDirectory = normalizeWorkingDirectory(options.WorkingDirectory)
	if options.CompleteSuite {
		options.AttemptID = "full"
	}
	if err := validateRunnerSelection(options); err != nil {
		return options, err
	}
	if *packageGate && (!repositoryTestArguments(options.Arguments) || options.CompleteSuite) || !*packageGate && repositoryTestArguments(options.Arguments) && !options.CompleteSuite {
		return options, errors.New("focused repository tests require an exact source and names; package argv requires explicit package-gate")
	}
	return options, nil
}

func argumentSeparator(arguments []string) int {
	for index, argument := range arguments {
		if argument == "--" {
			return index
		}
	}
	return -1
}

func runRunnerChild(arguments []string, stderr io.Writer) int {
	separator := argumentSeparator(arguments)
	if separator < 0 || separator == len(arguments)-1 {
		return 2
	}
	flags := flag.NewFlagSet("runner-child-exec", flag.ContinueOnError)
	flags.SetOutput(stderr)
	capabilityFD := flags.Int("capability-fd", -1, "")
	execStatusFD := flags.Int("exec-status-fd", -1, "")
	parentPID := flags.Int("parent-pid", -1, "")
	nonce := flags.String("nonce", "", "")
	if flags.Parse(arguments[:separator]) != nil || flags.NArg() != 0 {
		return 2
	}
	if err := internalRunnerChild(*capabilityFD, *execStatusFD, *parentPID, *nonce, arguments[separator+1:]); err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	return 0
}

func controllerSignals() (<-chan os.Signal, func(), error) {
	const setParentDeathSignal = 1
	runtime.LockOSThread()
	parent := os.Getppid()
	cancellation := make(chan os.Signal, 1)
	signal.Notify(cancellation, os.Interrupt, syscall.SIGHUP, syscall.SIGTERM)
	_, _, errno := syscall.Syscall6(syscall.SYS_PRCTL, setParentDeathSignal, uintptr(syscall.SIGTERM), 0, 0, 0, 0)
	if errno != 0 {
		signal.Stop(cancellation)
		runtime.UnlockOSThread()
		return nil, nil, errno
	}
	if os.Getppid() != parent {
		cancellation <- syscall.SIGTERM
	}
	release := func() {
		_, _, _ = syscall.Syscall6(syscall.SYS_PRCTL, setParentDeathSignal, 0, 0, 0, 0, 0)
		signal.Stop(cancellation)
		runtime.UnlockOSThread()
	}
	return cancellation, release, nil
}

func normalizeWorkingDirectory(value string) string {
	if value == "/workspace" {
		return "."
	}
	return strings.TrimPrefix(value, "/workspace/")
}

func runnerTerminalData(result RunnerExecResult) []byte {
	data, _ := core.CanonicalJSON(result)
	return data
}

func runnerTerminalPath(options RunnerExecOptions) string {
	return filepath.Join(options.RecordRoot, "runner-terminal", options.AttemptID+".json")
}

func writeResult(result any, terminal string, runErr error, stdout, stderr io.Writer) int {
	if runErr != nil {
		fmt.Fprintln(stderr, runErr)
	}
	data, encodeErr := core.CanonicalJSON(result)
	if encodeErr != nil {
		fmt.Fprintln(stderr, encodeErr)
		return 2
	}
	written, writeErr := stdout.Write(data)
	if writeErr == nil && written != len(data) {
		writeErr = io.ErrShortWrite
	}
	if writeErr != nil {
		fmt.Fprintln(stderr, writeErr)
		return 2
	}
	if runErr != nil || terminal != "Pass" {
		return 1
	}
	return 0
}

type waitNotice struct {
	stderr    io.Writer
	operation string
	mutex     sync.Mutex
	started   time.Time
}

func newWaitNotice(stderr io.Writer, operation string) *waitNotice {
	notice := &waitNotice{stderr: stderr, operation: operation, started: time.Now()}
	notice.Phase("Starting")
	return notice
}

func (notice *waitNotice) Phase(phase string) {
	notice.mutex.Lock()
	defer notice.mutex.Unlock()
	fmt.Fprintf(notice.stderr, "codex-build phase: %s %s elapsed-ms=%d; nonterminal: wait on the same running handle at the longest permitted native interval capped at 25 minutes, without commentary; only terminal JSON completes\n", notice.operation, phase, time.Since(notice.started).Milliseconds())
}

type publicAdmission struct{}

type loadGraphNode struct {
	Depth int
	Name  string
}

func admitPublicRoute(options *endpointOptions, fixedPoint string) error {
	root, endpointPath, endpointInfo, err := publicAdmissionRoot(*options)
	if err != nil {
		return err
	}
	if noFollowPath("executing endpoint", endpointPath) != nil {
		return errors.New("executing endpoint contains a symlink")
	}
	currentEndpointInfo, currentInfoErr := os.Lstat(endpointPath)
	if currentInfoErr != nil || !os.SameFile(endpointInfo, currentEndpointInfo) {
		return errors.Join(currentInfoErr, errors.New("executing endpoint changed during admission"))
	}
	graphPath := filepath.Join(root, "contracts", "load-graph.md")
	if !containedBy(root, graphPath) || noFollowPath("LoadGraph", graphPath) != nil {
		return errors.New("LoadGraph is outside or contains a symlink")
	}
	graphBytes, err := readRegularNoFollow(graphPath)
	if err != nil {
		return errors.Join(err, errors.New("LoadGraph is unreadable"))
	}
	groups, err := parseDocumentedLoadGraph(graphBytes)
	if err != nil {
		return err
	}
	relativePaths, err := expandLoadGraph(groups, fixedPoint)
	if err != nil {
		return err
	}
	for _, relative := range relativePaths {
		path := filepath.Join(root, filepath.FromSlash(relative))
		if relative == "bin" {
			path = endpointPath
		} else if relative == "contracts/load-graph.md" {
			continue
		} else {
			if !containedBy(root, path) || noFollowPath("LoadGraph entry", path) != nil {
				return errors.New("LoadGraph entry is outside or contains a symlink")
			}
			if _, readErr := readRegularNoFollow(path); readErr != nil {
				return errors.Join(readErr, errors.New("LoadGraph entry is unreadable"))
			}
		}
	}
	options.admission = &publicAdmission{}
	return nil
}

func publicAdmissionRoot(options endpointOptions) (string, string, os.FileInfo, error) {
	if noFollowPath("skill root", options.SkillRoot) != nil {
		return "", "", nil, errors.New("invalid skill root")
	}
	self, err := os.Executable()
	if err != nil {
		return "", "", nil, err
	}
	selfInfo, selfErr := os.Lstat(self)
	installed := filepath.Join(options.SkillRoot, "bin")
	installedInfo, installedErr := os.Lstat(installed)
	if selfErr == nil && installedErr == nil && selfInfo.Mode().IsRegular() && installedInfo.Mode().IsRegular() && os.SameFile(selfInfo, installedInfo) {
		return options.SkillRoot, installed, installedInfo, nil
	}
	return "", "", nil, errors.Join(selfErr, installedErr, errors.New("current process is not the installed endpoint"))
}

func parseDocumentedLoadGraph(data []byte) (map[string][]loadGraphNode, error) {
	lines := strings.Split(string(data), "\n")
	start, end := -1, -1
	for index := 0; index+1 < len(lines); index++ {
		if lines[index] != "```" || lines[index+1] != "LoadGraph" {
			continue
		}
		if start >= 0 {
			return nil, errors.New("LoadGraph fence is duplicated")
		}
		start = index + 2
		for cursor := start; cursor < len(lines); cursor++ {
			if lines[cursor] == "```" {
				end = cursor
				break
			}
		}
	}
	if start < 0 || end < start {
		return nil, errors.New("LoadGraph fence is absent")
	}
	groups := map[string][]loadGraphNode{}
	current := ""
	for _, line := range lines[start:end] {
		depth, name, ok := parseLoadGraphLine(line)
		if !ok {
			return nil, errors.New("LoadGraph line differs")
		}
		if depth == 0 {
			if !validName(name, "") || groups[name] != nil {
				return nil, errors.New("LoadGraph group differs")
			}
			current, groups[name] = name, []loadGraphNode{}
			continue
		}
		if current == "" {
			return nil, errors.New("LoadGraph entry has no group")
		}
		groups[current] = append(groups[current], loadGraphNode{Depth: depth - 1, Name: name})
	}
	return groups, nil
}

func parseLoadGraphLine(line string) (int, string, bool) {
	depth, rest := 0, line
	for strings.HasPrefix(rest, "|   ") || strings.HasPrefix(rest, "    ") {
		depth++
		rest = rest[4:]
	}
	if !strings.HasPrefix(rest, "+-- ") && !strings.HasPrefix(rest, "`-- ") {
		return 0, "", false
	}
	name := rest[4:]
	return depth, name, name != ""
}

func expandLoadGraph(groups map[string][]loadGraphNode, fixedPoint string) ([]string, error) {
	paths := []string{}
	seenPaths, activeGroups := map[string]bool{}, map[string]bool{}
	var expand func(string) error
	expand = func(group string) error {
		nodes, exists := groups[group]
		if !exists || activeGroups[group] {
			return errors.New("LoadGraph reference differs")
		}
		activeGroups[group] = true
		defer delete(activeGroups, group)
		directories := []string{}
		for _, node := range nodes {
			if node.Depth > len(directories) {
				return errors.New("LoadGraph depth differs")
			}
			directories = directories[:node.Depth]
			if _, reference := groups[node.Name]; reference {
				if node.Depth != 0 || len(directories) != 0 {
					return errors.New("LoadGraph reference is nested")
				}
				if err := expand(node.Name); err != nil {
					return err
				}
				continue
			}
			if strings.HasSuffix(node.Name, "/") {
				directory := strings.TrimSuffix(node.Name, "/")
				if !validLoadGraphPath(directory) {
					return errors.New("LoadGraph directory differs")
				}
				directories = append(directories, directory)
				continue
			}
			relative := filepath.ToSlash(filepath.Join(append(append([]string{}, directories...), node.Name)...))
			if !validLoadGraphPath(relative) || seenPaths[relative] {
				return errors.New("LoadGraph path is invalid or duplicated")
			}
			seenPaths[relative] = true
			paths = append(paths, relative)
		}
		return nil
	}
	if err := expand(fixedPoint); err != nil {
		return nil, err
	}
	return paths, nil
}

func validLoadGraphPath(value string) bool {
	if value == "" || value == "." || filepath.IsAbs(value) || filepath.Clean(value) != value || strings.ContainsAny(value, "*?[]{}$\\\x00\n") {
		return false
	}
	return filepath.IsLocal(value)
}
