package runtime

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"codex-build/internal/assets"
	"codex-build/internal/core"
)

type sourceProjection struct {
	SourceBranch string
	SourceClean  bool
}

type preflightHelperError struct{ err error }

func (err preflightHelperError) Error() string { return err.err.Error() }
func (err preflightHelperError) Unwrap() error { return err.err }

var errNativeTimeout = errors.New("native process timed out")
var errOperationCanceled = errors.New("operation canceled")

func validName(value, punctuation string) bool {
	if value == "" {
		return false
	}
	for _, character := range value {
		if (character >= 'a' && character <= 'z') || (character >= 'A' && character <= 'Z') || (character >= '0' && character <= '9') || strings.ContainsRune(punctuation, character) {
			continue
		}
		return false
	}
	return true
}

type deadline struct {
	end     time.Time
	signals <-chan os.Signal
}

func newDeadline(duration time.Duration, signals <-chan os.Signal) deadline {
	return deadline{time.Now().Add(duration), signals}
}

func (window deadline) remaining() time.Duration { return time.Until(window.end) }

func (window deadline) timeout() error {
	if window.remaining() <= 0 {
		return errNativeTimeout
	}
	return nil
}

func (window deadline) check() error {
	if err := window.timeout(); err != nil {
		return err
	}
	select {
	case <-window.signals:
		return errOperationCanceled
	default:
		return nil
	}
}

func runGitWithin(window deadline, environment []string, arguments ...string) error {
	command := exec.Command("git", arguments...)
	command.Env = append([]string{"HOME=/nonexistent", "PATH=/usr/bin:/bin", "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL=/dev/null"}, environment...)
	return window.run(command)
}

func validateGitTree(window deadline, source, reference string) error {
	commands := [][]string{
		{"show-ref", "--verify", "--quiet", reference},
		{"diff", "--quiet", "--no-ext-diff", "--no-textconv", "--no-renames", reference, "HEAD", "--"},
		{"merge-base", "--is-ancestor", reference, "HEAD"},
		{"merge-base", "--is-ancestor", "HEAD", reference},
	}
	for _, arguments := range commands {
		if err := runGitWithin(window, nil, append([]string{"-C", source}, arguments...)...); err != nil {
			return err
		}
	}
	return nil
}

func (window deadline) run(command *exec.Cmd) error {
	if err := window.check(); err != nil {
		return err
	}
	command.SysProcAttr = &syscall.SysProcAttr{Setpgid: true, Pdeathsig: syscall.SIGTERM}
	if err := command.Start(); err != nil {
		return err
	}
	runErr, timedOut, canceled := waitProcessGroup(command, window.remaining(), core.GracePeriod(), window.signals)
	if timedOut {
		runErr = errors.Join(runErr, errNativeTimeout)
	}
	if canceled {
		runErr = errors.Join(runErr, errOperationCanceled)
	}
	return runErr
}

func sourceProjectionAt(runtimeRoot, source string) (sourceProjection, error) {
	return nativeSourceProjectionAt(runtimeRoot, source, "custody-source-projection")
}

func detachedSourceProjectionAt(runtimeRoot, source string) (sourceProjection, error) {
	return nativeSourceProjectionAt(runtimeRoot, source, "custody-detached-source-projection")
}

func nativeSourceProjectionAt(runtimeRoot, source, action string) (sourceProjection, error) {
	return nativeSourceWithin(runtimeRoot, source, action, core.StageDeadline(), nil)
}

func nativeSourceWithin(runtimeRoot, source, action string, deadline time.Duration, cancellation <-chan os.Signal) (sourceProjection, error) {
	data, err := runNativeWithin(runtimeRoot, action, deadline, cancellation, source)
	if err != nil {
		var exitErr *exec.ExitError
		if !errors.As(err, &exitErr) || exitErr.ExitCode() != 75 {
			err = preflightHelperError{errors.Join(err, errors.New("native source projection failed"))}
		}
		return sourceProjection{}, err
	}
	if len(data) == 0 || data[len(data)-1] != '\n' {
		return sourceProjection{}, preflightHelperError{errors.New("native source projection is invalid")}
	}
	fields := strings.Split(strings.TrimSuffix(string(data), "\n"), "\n")
	if len(fields) != 2 || !listed("true false", fields[0]) {
		return sourceProjection{}, preflightHelperError{errors.New("native source projection fields are invalid")}
	}
	return sourceProjection{SourceClean: fields[0] == "true", SourceBranch: fields[1]}, nil
}

func runNativeCustody(runtimeRoot, action string, arguments ...string) ([]byte, error) {
	return runNativeWithin(runtimeRoot, action, core.StageDeadline(), nil, arguments...)
}

func runNativeWithin(runtimeRoot, action string, duration time.Duration, cancellation <-chan os.Signal, arguments ...string) (data []byte, err error) {
	window := newDeadline(duration, cancellation)
	root, script, err := prepareNativeAttempt(runtimeRoot, action)
	if err != nil {
		return nil, err
	}
	defer func() { err = errors.Join(err, removePrivateAttemptWithin(root, newDeadline(core.GracePeriod(), nil))) }()
	if err := window.check(); err != nil {
		return nil, err
	}
	output := filepath.Join(root, "result")
	arguments = append(arguments, output)
	var diagnostic string
	data, diagnostic, err = runNativeProcess(root, script, action, arguments, nativeEnvironment(root), 1<<20, output, window.remaining(), cancellation, nil, nil)
	if message := strings.TrimSpace(diagnostic); err != nil && message != "" {
		err = errors.Join(err, errors.New(message))
	}
	return data, err
}

func prepareNativeAttempt(runtimeRoot, kind string) (string, string, error) {
	id, err := randomID()
	if err != nil {
		return "", "", err
	}
	root := filepath.Join(runtimeRoot, kind, id)
	if _, err := os.Lstat(root); !os.IsNotExist(err) {
		return "", "", errors.Join(err, errors.New("native attempt already exists"))
	}
	for _, path := range []string{root, filepath.Join(root, "home")} {
		if err := ensureDirectoryNoFollow(path, 0o700); err != nil {
			return "", "", errors.Join(err, removeOwnedPaths(root))
		}
	}
	script := filepath.Join(root, "release.py")
	if err := writeRuntimeAsset(script, assets.ReleaseRoll(), 0o700); err != nil {
		return "", "", errors.Join(err, removeOwnedPaths(root))
	}
	return root, script, nil
}

func nativeEnvironment(root string) []string {
	return []string{"PATH=/usr/bin:/bin", "HOME=" + filepath.Join(root, "home"), "TMPDIR=" + root, "LC_ALL=C.UTF-8", "TZ=UTC"}
}

const nativeDiagnosticLimit = 4 << 10
const nativeDiagnosticMarker = "\n... stderr truncated ...\n"

type diagnosticWriter struct {
	data      []byte
	truncated bool
	onPhase   func(string)
	line      []byte
	longLine  bool
}

func (writer *diagnosticWriter) Write(data []byte) (int, error) {
	written := len(data)
	if writer.onPhase == nil {
		writer.appendDiagnostic(data)
		return written, nil
	}
	for len(data) != 0 {
		newline := bytes.IndexByte(data, '\n')
		fragmentSize := len(data)
		if newline >= 0 {
			fragmentSize = newline + 1
		}
		fragment := data[:fragmentSize]
		data = data[fragmentSize:]
		if writer.longLine || len(writer.line)+len(fragment) > nativeDiagnosticLimit {
			writer.appendDiagnostic(writer.line)
			writer.line = writer.line[:0]
			writer.appendDiagnostic(fragment)
			writer.longLine = newline < 0
			continue
		}
		writer.line = append(writer.line, fragment...)
		if newline < 0 {
			continue
		}
		phase, notice := strings.CutPrefix(string(writer.line), "codex-build phase: repository-format ")
		phase, _, complete := strings.Cut(phase, " elapsed-ms=")
		if notice && complete && phase != "" {
			writer.onPhase("RepositoryFormatting " + phase)
		} else {
			writer.appendDiagnostic(writer.line)
		}
		writer.line = writer.line[:0]
	}
	return written, nil
}

func (writer *diagnosticWriter) appendDiagnostic(data []byte) {
	if !writer.truncated && len(writer.data)+len(data) <= nativeDiagnosticLimit {
		writer.data = append(writer.data, data...)
		return
	}
	headLimit := (nativeDiagnosticLimit - len(nativeDiagnosticMarker)) / 2
	tailLimit := nativeDiagnosticLimit - len(nativeDiagnosticMarker) - headLimit
	if !writer.truncated {
		head := append([]byte(nil), writer.data[:min(len(writer.data), headLimit)]...)
		if len(head) < headLimit {
			head = append(head, data[:headLimit-len(head)]...)
		}
		needed := tailLimit - min(len(data), tailLimit)
		tail := append([]byte(nil), writer.data[len(writer.data)-needed:]...)
		tail = append(tail, data[max(0, len(data)-tailLimit):]...)
		writer.data = append(append(head, nativeDiagnosticMarker...), tail...)
		writer.truncated = true
		return
	}
	tail := writer.data[headLimit+len(nativeDiagnosticMarker):]
	if len(data) >= tailLimit {
		copy(tail, data[len(data)-tailLimit:])
	} else {
		copy(tail, tail[len(data):])
		copy(tail[tailLimit-len(data):], data)
	}
}

func (writer *diagnosticWriter) String() string {
	writer.appendDiagnostic(writer.line)
	writer.line = nil
	return string(writer.data)
}

func runNativeProcess(attempt, script, action string, arguments, environment []string, limit int64, resultPath string, deadline time.Duration, cancellation <-chan os.Signal, onLive func(), onPhase func(string)) ([]byte, string, error) {
	window := newDeadline(deadline, cancellation)
	if err := window.check(); err != nil {
		return nil, "", err
	}
	logPath := filepath.Join(attempt, "native.log")
	log, err := os.OpenFile(logPath, os.O_WRONLY|os.O_CREATE|os.O_APPEND|syscall.O_NOFOLLOW, 0o600)
	if err != nil {
		return nil, "", err
	}
	used, seekErr := log.Seek(0, io.SeekEnd)
	if seekErr != nil || used > limit {
		return nil, "", errors.Join(seekErr, syncClose(log), errors.New("native log exceeded its bound"))
	}
	writer := &boundedWriter{writer: log, remaining: limit - used}
	stderr := &diagnosticWriter{onPhase: onPhase}
	command := exec.Command(script, append([]string{action}, arguments...)...)
	command.Env = environment
	command.Stdout, command.Stderr = writer, io.MultiWriter(writer, stderr)
	command.SysProcAttr = &syscall.SysProcAttr{Setpgid: true, Pdeathsig: syscall.SIGTERM}
	if err := command.Start(); err != nil {
		_ = syncClose(log)
		return nil, "", err
	}
	if onLive != nil {
		onLive()
	}
	grace := core.GracePeriod()
	if action == "patch-replay-apply" {
		grace *= 2
	}
	runErr, timedOut, canceled := waitProcessGroup(command, window.remaining(), grace, cancellation)
	closeErr := syncClose(log)
	if writer.truncated {
		runErr = errors.Join(runErr, errors.New("native log exceeded its bound"))
	}
	if timedOut {
		runErr = errors.Join(runErr, errNativeTimeout)
	}
	if canceled {
		runErr = errors.Join(runErr, errOperationCanceled)
	}
	var data []byte
	var readErr error
	if resultPath != "" {
		data, readErr = readRegularNoFollow(resultPath)
	}
	return data, stderr.String(), errors.Join(runErr, closeErr, readErr)
}

func randomID() (string, error) {
	data := make([]byte, 4)
	if _, err := rand.Read(data); err != nil {
		return "", err
	}
	return hex.EncodeToString(data), nil
}

func awaitRead(window deadline, reader io.Reader, stop func()) ([]byte, error) {
	done := make(chan []byte, 1)
	var readErr error
	go func() {
		data, err := io.ReadAll(reader)
		readErr = err
		done <- data
	}()
	timer := time.NewTimer(window.remaining())
	defer timer.Stop()
	cause := errNativeTimeout
	select {
	case data := <-done:
		return data, errors.Join(readErr, window.timeout())
	case <-timer.C:
	case <-window.signals:
		cause = errOperationCanceled
	}
	stop()
	data := <-done
	return data, errors.Join(cause, readErr)
}

func readRegularNoFollow(path string, windows ...deadline) ([]byte, error) {
	file, err := os.OpenFile(path, os.O_RDONLY|syscall.O_NOFOLLOW, 0)
	if err != nil {
		return nil, err
	}
	info, statErr := file.Stat()
	if statErr != nil || !info.Mode().IsRegular() {
		_ = file.Close()
		return nil, errors.Join(statErr, fmt.Errorf("not a regular file: %s", path))
	}
	var data []byte
	var readErr error
	if len(windows) != 0 {
		data, readErr = awaitRead(windows[0], file, func() { _ = file.Close() })
	} else {
		data, readErr = io.ReadAll(file)
	}
	closeErr := file.Close()
	return data, errors.Join(readErr, closeErr)
}

func cargoLock(root string) (*os.File, error) { return lockCargo(root, syscall.LOCK_EX) }

func tryCargoLock(root string) (*os.File, error) {
	return lockCargo(root, syscall.LOCK_EX|syscall.LOCK_NB)
}

func lockCargo(root string, op int) (*os.File, error) {
	if err := noFollowPath("project scratch root", root); err != nil {
		return nil, err
	}
	locks := filepath.Join(root, "locks")
	if err := ensureDirectoryNoFollow(locks, 0o700); err != nil {
		return nil, err
	}
	file, err := os.OpenFile(filepath.Join(locks, "cargo-lock"), os.O_RDWR|os.O_CREATE|syscall.O_NOFOLLOW, 0o600)
	if err != nil {
		return nil, err
	}
	if err := syscall.Flock(int(file.Fd()), op); err != nil {
		_ = file.Close()
		return nil, err
	}
	return file, nil
}

func releaseCargoLock(file *os.File) {
	_ = syscall.Flock(int(file.Fd()), syscall.LOCK_UN)
	_ = file.Close()
}

func noFollowPath(name, path string) error {
	if !filepath.IsAbs(path) || filepath.Clean(path) != path {
		return fmt.Errorf("%s must be a clean absolute path", name)
	}
	current := path
	for {
		info, err := os.Lstat(current)
		if err == nil {
			if info.Mode()&os.ModeSymlink != 0 {
				return fmt.Errorf("%s contains a symlink: %s", name, current)
			}
		} else if !os.IsNotExist(err) {
			return err
		}
		parent := filepath.Dir(current)
		if parent == current {
			return nil
		}
		current = parent
	}
}

func containedBy(root, path string) bool {
	relative, err := filepath.Rel(root, path)
	return err == nil && relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator))
}

func pathsOverlap(left, right string) bool {
	return containedBy(left, right) || containedBy(right, left)
}

func removeOwnedPaths(paths ...string) error {
	for _, path := range paths {
		if path == "" {
			continue
		}
		if err := os.RemoveAll(path); err != nil {
			return err
		}
		if _, err := os.Lstat(path); !os.IsNotExist(err) {
			return fmt.Errorf("owned path remains after cleanup: %s", path)
		}
	}
	return nil
}

func removeOwnedPathWithin(path string, window deadline) error {
	if err := window.run(exec.Command("/usr/bin/rm", "-rf", "--", path)); err != nil {
		return err
	}
	if _, err := os.Lstat(path); !os.IsNotExist(err) {
		return errors.Join(err, errors.New("path remains"))
	}
	return nil
}

func removePrivateAttempt(root string) error {
	return removePrivateAttemptWithin(root, newDeadline(core.StageDeadline(), nil))
}

func removePrivateAttemptWithin(root string, window deadline) error {
	if _, err := os.Lstat(root); os.IsNotExist(err) {
		return nil
	} else if err != nil {
		return err
	}
	const atFDCWD = -100
	const atSymlinkNoFollow = 0x100
	if err := filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil || !entry.IsDir() {
			return walkErr
		}
		if err := window.check(); err != nil {
			return err
		}
		return syscall.Fchmodat(atFDCWD, path, 0o700, atSymlinkNoFollow)
	}); err != nil {
		return err
	}
	return removeOwnedPathWithin(root, window)
}

func writeRuntimeAsset(path string, data []byte, mode os.FileMode) (err error) {
	return publishNoReplace(path, data, mode)
}

func syncDirectory(path string) error {
	directory, err := os.OpenFile(path, os.O_RDONLY|syscall.O_NOFOLLOW|syscall.O_DIRECTORY, 0)
	if err != nil {
		return err
	}
	return errors.Join(directory.Sync(), directory.Close())
}

func syncClose(file *os.File) error {
	if file == nil {
		return nil
	}
	return errors.Join(file.Sync(), file.Close())
}

func closeFiles(files ...*os.File) {
	for _, file := range files {
		if file != nil {
			_ = file.Close()
		}
	}
}
