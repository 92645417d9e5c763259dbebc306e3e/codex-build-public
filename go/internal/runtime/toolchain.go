package runtime

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"codex-build/internal/core"
)

type ToolchainProjection struct {
	Root         string
	Path         string
	CargoVersion string
	Target       string
}

const nativeToolchainRoot = "build/toolchains/native/toolchain"
const nativeVoiceVersion = "1.28.6"
const nativeRustVersion = "1.95.0"
const nativeNextestVersion = "0.9.131"
const nativeSccacheVersion = "0.15.0"
const nativeV8Version = "150.4.0"
const nativeBazeliskVersion = "1.29.0"
const repositoryBazelVersion = "9.0.0"
const nativeTarget = "x86_64-unknown-linux-gnu"

type CacheProjection struct {
	CargoHome        string
	CargoBuild       string
	CargoTarget      string
	SccacheDirectory string
}

func resolveToolchainProjection(projectWorkingRoot string) (ToolchainProjection, error) {
	return resolveToolchain(projectWorkingRoot, newDeadline(core.StageDeadline(), nil))
}

func resolveToolchain(projectWorkingRoot string, window deadline) (ToolchainProjection, error) {
	path := filepath.Join(projectWorkingRoot, filepath.FromSlash(nativeToolchainRoot))
	if err := noFollowPath("toolchain", path); err != nil {
		return ToolchainProjection{}, err
	}
	projection := ToolchainProjection{Root: nativeToolchainRoot, Path: path}
	cargoVersion, err := verifyBuildToolchain(path, window)
	if err != nil {
		return ToolchainProjection{}, err
	}
	projection.CargoVersion = cargoVersion
	host, err := commandOutput(exec.Command(filepath.Join(path, "rust", "bin", "rustc"), "-vV"), window)
	if err != nil {
		return ToolchainProjection{}, err
	}
	for _, line := range strings.Split(string(host), "\n") {
		if strings.HasPrefix(line, "host: ") {
			projection.Target = strings.TrimPrefix(line, "host: ")
		}
	}
	if !validName(projection.Target, "-_") {
		return ToolchainProjection{}, errors.New("invalid Rust target")
	}
	if err := verifyJustToolchain(path, window); err != nil {
		return ToolchainProjection{}, err
	}
	if err := verifyRepositoryFormatToolchain(path, window); err != nil {
		return ToolchainProjection{}, err
	}
	if err := verifyNativeVoiceToolchain(path, projection.Target, window); err != nil {
		return ToolchainProjection{}, err
	}
	return projection, window.check()
}

func commandOutput(command *exec.Cmd, window deadline) ([]byte, error) {
	if err := window.check(); err != nil {
		return nil, err
	}
	output := bytes.Buffer{}
	command.Stdout = &output
	if err := window.run(command); err != nil {
		return nil, err
	}
	return output.Bytes(), window.check()
}

func verifyJustToolchain(root string, window deadline) error {
	manifest, err := readRegularNoFollow(filepath.Join(root, "native-toolchain.json"))
	version := struct {
		Just string `json:"just"`
	}{}
	just, justErr := readToolVersionAt(filepath.Join(root, "bin", "just"), "just", window)
	if err != nil || json.Unmarshal(manifest, &version) != nil || version.Just == "" || justErr != nil || version.Just != just.String() {
		return errors.Join(err, justErr, errors.New("native toolchain just version differs"))
	}
	return nil
}

func verifyRepositoryFormatToolchain(root string, window deadline) error {
	manifest, err := readRegularNoFollow(filepath.Join(root, "native-toolchain.json"))
	versions := struct {
		DotSlash string `json:"dotslash"`
		UV       string `json:"uv"`
	}{}
	dotslash, dotslashErr := readToolVersionAt(filepath.Join(root, "bin", "dotslash"), "DotSlash", window)
	uv, uvErr := readToolVersionAt(filepath.Join(root, "bin", "uv"), "uv", window)
	if err != nil || json.Unmarshal(manifest, &versions) != nil || versions.DotSlash == "" || versions.UV == "" || dotslashErr != nil || uvErr != nil || versions.DotSlash != dotslash.String() || versions.UV != uv.String() {
		return errors.Join(err, dotslashErr, uvErr, errors.New("native toolchain repository formatter versions differ"))
	}
	return nil
}

func verifyNativeVoiceToolchain(root, target string, window deadline) error {
	if target != "x86_64-unknown-linux-gnu" {
		return errors.New("native voice toolchain target differs")
	}
	voice := filepath.Join(root, "voice")
	sdk := filepath.Join(voice, "sdk")
	runtime := filepath.Join(voice, "runtime")
	for _, path := range []string{
		voice,
		sdk,
		filepath.Join(sdk, "include"),
		filepath.Join(sdk, "lib"),
		filepath.Join(sdk, "lib", "pkgconfig"),
		runtime,
		filepath.Join(runtime, "lib"),
		filepath.Join(runtime, "lib", "gstreamer-1.0"),
	} {
		info, err := os.Lstat(path)
		if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			return errors.Join(err, errors.New("native voice toolchain directory differs"))
		}
	}
	required := []string{
		filepath.Join(root, "native-toolchain.json"),
		filepath.Join(sdk, "bin", "gst-inspect-1.0"),
		filepath.Join(sdk, "sdk.json"),
		filepath.Join(runtime, "lib", "libgstreamer-1.0.so.0"),
		filepath.Join(runtime, "libexec", "gstreamer-1.0", "gst-plugin-scanner"),
		filepath.Join(runtime, "runtime.json"),
	}
	for _, plugin := range []string{"app", "audioconvert", "audioresample", "coreelements", "opus", "rtp", "rtpmanager"} {
		required = append(required, filepath.Join(runtime, "lib", "gstreamer-1.0", "libgst"+plugin+".so"))
	}
	for _, module := range []string{"gstreamer-1.0", "gstreamer-base-1.0", "gstreamer-app-1.0", "gstreamer-audio-1.0"} {
		required = append(required, filepath.Join(sdk, "lib", "pkgconfig", module+".pc"))
	}
	for _, path := range required {
		info, err := os.Lstat(path)
		if err != nil || !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 {
			return errors.Join(err, errors.New("native voice toolchain file differs"))
		}
	}
	manifest, err := readRegularNoFollow(filepath.Join(root, "native-toolchain.json"))
	manifestVersion := struct {
		GStreamer string `json:"gstreamer"`
	}{}
	if err != nil || json.Unmarshal(manifest, &manifestVersion) != nil || manifestVersion.GStreamer != nativeVoiceVersion {
		return errors.Join(err, errors.New("native voice toolchain version differs"))
	}
	pkgConfig := "/usr/bin/pkg-config"
	command := exec.Command(pkgConfig, "--modversion", "gstreamer-1.0", "gstreamer-base-1.0", "gstreamer-app-1.0", "gstreamer-audio-1.0")
	command.Env = []string{"PATH=/usr/bin", "PKG_CONFIG_LIBDIR=" + filepath.Join(sdk, "lib", "pkgconfig") + ":/usr/lib64/pkgconfig:/usr/share/pkgconfig", "PKG_CONFIG_PATH="}
	output, err := commandOutput(command, window)
	versions := strings.Fields(string(output))
	if err != nil || len(versions) != 4 {
		return errors.Join(err, errors.New("native voice toolchain version differs"))
	}
	for _, value := range versions {
		if value != nativeVoiceVersion {
			return errors.New("native voice toolchain version differs")
		}
	}
	alsaCommand := exec.Command(pkgConfig, "--modversion", "alsa")
	alsaCommand.Env = command.Env
	alsa, err := commandOutput(alsaCommand, window)
	alsaVersion := toolVersion{}
	_, parseErr := fmt.Sscanf(strings.TrimSpace(string(alsa)), "%d.%d.%d", &alsaVersion.Major, &alsaVersion.Minor, &alsaVersion.Patch)
	if err != nil || parseErr != nil || !alsaVersion.atLeast(1, 2, 9) {
		return errors.Join(err, parseErr, errors.New("native voice ALSA version differs"))
	}
	opusCommand := exec.Command(pkgConfig, "--modversion", "opus")
	opusCommand.Env = command.Env
	opus, err := commandOutput(opusCommand, window)
	if err != nil || strings.TrimSpace(string(opus)) != "1.6.1" {
		return errors.Join(err, errors.New("native voice Opus version differs"))
	}
	link := exec.Command(pkgConfig, "--libs", "alsa", "gstreamer-1.0", "gstreamer-base-1.0", "gstreamer-app-1.0", "gstreamer-audio-1.0")
	link.Env = command.Env
	linkOutput, err := commandOutput(link, window)
	if err != nil || !voiceLinkSearchPaths(string(linkOutput), filepath.Join(sdk, "lib")) {
		return errors.Join(err, errors.New("native voice SDK link paths differ"))
	}
	if err := validateNativeVoiceManifests(sdk, runtime, target); err != nil {
		return err
	}
	if err := verifyNativeVoiceRegistry(sdk, runtime, window); err != nil {
		return err
	}
	return nil
}

func verifyNativeVoiceRegistry(sdk, runtime string, window deadline) error {
	environment := []string{
		"GST_PLUGIN_PATH_1_0=" + filepath.Join(runtime, "lib", "gstreamer-1.0"),
		"GST_PLUGIN_SCANNER=" + filepath.Join(runtime, "libexec", "gstreamer-1.0", "gst-plugin-scanner"),
		"GST_PLUGIN_SYSTEM_PATH_1_0=",
		"GST_REGISTRY_1_0=/dev/null",
		"LD_LIBRARY_PATH=" + filepath.Join(runtime, "lib"),
		"PATH=/usr/bin",
	}
	inspector := filepath.Join(sdk, "bin", "gst-inspect-1.0")
	for _, plugin := range []string{"app", "audioconvert", "audioresample", "coreelements", "opus", "rtp", "rtpmanager"} {
		command := exec.Command(inspector, "--plugin", plugin)
		command.Env = environment
		if err := window.run(command); err != nil {
			return errors.Join(err, errors.New("native voice registry discovery differs"))
		}
	}
	return nil
}

func voiceLinkSearchPaths(output, sdkLibrary string) bool {
	found := false
	for _, argument := range strings.Fields(output) {
		if strings.HasPrefix(argument, "-L") {
			path := filepath.Clean(strings.TrimPrefix(argument, "-L"))
			if path != sdkLibrary && path != "/usr/lib64" {
				return false
			}
			found = true
		}
	}
	return found
}

type nativeVoiceRuntimeManifest struct {
	SchemaVersion int `json:"schemaVersion"`
	Target        string
	Libraries     []struct {
		Path    string
		Imports []string
	}
	Plugins []string
}

func validateNativeVoiceManifests(sdk, runtime, target string) error {
	if value, err := nativeVoiceManifestTarget(filepath.Join(sdk, "sdk.json")); err != nil || value != target {
		return errors.Join(err, errors.New("native voice SDK target differs"))
	}
	data, err := readRegularNoFollow(filepath.Join(runtime, "runtime.json"))
	runtimeManifest := nativeVoiceRuntimeManifest{}
	if err != nil || json.Unmarshal(data, &runtimeManifest) != nil || runtimeManifest.SchemaVersion != 1 || runtimeManifest.Target != target {
		return errors.Join(err, errors.New("native voice runtime manifest differs"))
	}
	return validateNativeVoiceRuntimeClosure(runtime, runtimeManifest)
}

func nativeVoiceManifestTarget(path string) (string, error) {
	data, err := readRegularNoFollow(path)
	if err != nil {
		return "", err
	}
	value := struct {
		SchemaVersion int `json:"schemaVersion"`
		Target        string
	}{}
	if json.Unmarshal(data, &value) != nil || value.SchemaVersion != 1 {
		return "", errors.New("native voice manifest differs")
	}
	return value.Target, nil
}

func validateNativeVoiceRuntimeClosure(root string, manifest nativeVoiceRuntimeManifest) error {
	system := map[string]bool{
		"ld-linux-x86-64.so.2": true, "libasound.so.2": true, "libblkid.so.1": true, "libc.so.6": true,
		"libdl.so.2": true, "libffi.so.8": true, "libgcc_s.so.1": true, "libgio-2.0.so.0": true,
		"libglib-2.0.so.0": true, "libgmodule-2.0.so.0": true, "libgobject-2.0.so.0": true,
		"libm.so.6": true, "libmount.so.1": true, "libpcre2-8.so.0": true, "libpthread.so.0": true,
		"libopus.so.0": true, "libresolv.so.2": true, "librt.so.1": true, "libstdc++.so.6": true, "libz.so.1": true,
	}
	libraries := map[string]bool{}
	for _, library := range manifest.Libraries {
		if !filepath.IsLocal(library.Path) || filepath.Clean(library.Path) != library.Path || libraries[filepath.Base(library.Path)] {
			return errors.New("native voice runtime inventory differs")
		}
		libraries[filepath.Base(library.Path)] = true
		info, err := os.Lstat(filepath.Join(root, library.Path))
		if err != nil || !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 {
			return errors.Join(err, errors.New("native voice runtime inventory differs"))
		}
	}
	for _, library := range manifest.Libraries {
		for _, dependency := range library.Imports {
			if !system[dependency] && !libraries[dependency] {
				return errors.New("native voice runtime dependency differs")
			}
		}
	}
	expectedPlugins := []string{"lib/gstreamer-1.0/libgstapp.so", "lib/gstreamer-1.0/libgstaudioconvert.so", "lib/gstreamer-1.0/libgstaudioresample.so", "lib/gstreamer-1.0/libgstcoreelements.so", "lib/gstreamer-1.0/libgstopus.so", "lib/gstreamer-1.0/libgstrtp.so", "lib/gstreamer-1.0/libgstrtpmanager.so"}
	if !sameStrings(manifest.Plugins, expectedPlugins) {
		return errors.New("native voice runtime plugins differ")
	}
	for _, plugin := range expectedPlugins {
		if !libraries[filepath.Base(plugin)] {
			return errors.New("native voice runtime plugin inventory differs")
		}
	}
	return nil
}

func cacheProjection(projectWorkingRoot string) CacheProjection {
	cacheRoot := filepath.Join(projectWorkingRoot, "build", "cache")
	return CacheProjection{
		CargoHome:        filepath.Join(cacheRoot, "cargo-home"),
		CargoBuild:       filepath.Join(cacheRoot, "build", "runner"),
		CargoTarget:      filepath.Join(cacheRoot, "target", "runner"),
		SccacheDirectory: filepath.Join(cacheRoot, "sccache"),
	}
}

func regularFileBytes(window deadline, paths ...string) (int64, error) {
	var total int64
	for _, root := range paths {
		err := filepath.WalkDir(root, func(_ string, entry os.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if err := window.check(); err != nil {
				return err
			}
			info, err := entry.Info()
			if err != nil {
				return err
			}
			if info.Mode().IsRegular() {
				total += info.Size()
			}
			return nil
		})
		if err != nil {
			return 0, err
		}
	}
	return total, nil
}

type toolVersion struct {
	Major int
	Minor int
	Patch int
}

func verifyBuildToolchain(toolchain string, window deadline) (string, error) {
	manifest, err := readRegularNoFollow(filepath.Join(toolchain, "native-toolchain.json"))
	versions := struct {
		Bazelisk      string `json:"bazelisk"`
		Cargo         string `json:"cargo"`
		CargoNextest  string `json:"cargo-nextest"`
		Rust          string `json:"rust"`
		RustyV8       string `json:"rusty_v8"`
		Sccache       string `json:"sccache"`
		SchemaVersion int    `json:"schemaVersion"`
		Target        string `json:"target"`
	}{}
	fields := map[string]json.RawMessage{}
	if err != nil || json.Unmarshal(manifest, &versions) != nil || json.Unmarshal(manifest, &fields) != nil || len(fields) != 12 || versions.Bazelisk != nativeBazeliskVersion || versions.Cargo != nativeRustVersion || versions.CargoNextest != nativeNextestVersion || versions.Rust != nativeRustVersion || versions.RustyV8 != nativeV8Version || versions.Sccache != nativeSccacheVersion || versions.SchemaVersion != 1 || versions.Target != nativeTarget {
		return "", errors.Join(err, errors.New("native build toolchain manifest differs"))
	}
	for _, name := range []string{"bazelisk", "cargo", "cargo-nextest", "dotslash", "gstreamer", "just", "rust", "rusty_v8", "sccache", "schemaVersion", "target", "uv"} {
		if _, ok := fields[name]; !ok {
			return "", errors.New("native build toolchain manifest differs")
		}
	}
	cargo, err := readToolVersionAt(filepath.Join(toolchain, "rust", "bin", "cargo"), "cargo", window)
	if err != nil {
		return "", err
	}
	if cargo.String() != nativeRustVersion {
		return "", fmt.Errorf("cargo %s lacks build-directory support", cargo)
	}
	nextest, err := readToolVersionAt(filepath.Join(toolchain, "bin", "cargo-nextest"), "cargo-nextest", window)
	if err != nil {
		return "", err
	}
	if nextest.String() != nativeNextestVersion {
		return "", fmt.Errorf("cargo-nextest %s lacks build-directory support", nextest)
	}
	bazelisk := exec.Command(filepath.Join(toolchain, "bin", "bazel"), "bazeliskVersion")
	bazelisk.Env = []string{"HOME=/nonexistent", "PATH=/usr/bin"}
	bazeliskOutput, err := commandOutput(bazelisk, window)
	if err != nil || strings.TrimSpace(string(bazeliskOutput)) != "Bazelisk version: v"+nativeBazeliskVersion {
		return "", errors.Join(err, errors.New("native Bazelisk version differs"))
	}
	sccache, err := readToolVersionAt(filepath.Join(toolchain, "bin", "sccache"), "sccache", window)
	if err != nil || sccache.String() != nativeSccacheVersion {
		return "", errors.Join(err, errors.New("native sccache version differs"))
	}
	rustc := exec.Command(filepath.Join(toolchain, "rust", "bin", "rustc"), "--version")
	rustc.Env = []string{"PATH=/usr/bin"}
	rustcOutput, err := commandOutput(rustc, window)
	if err != nil || !strings.HasPrefix(string(rustcOutput), "rustc "+nativeRustVersion+" ") {
		return "", errors.Join(err, errors.New("native Rust version differs"))
	}
	for _, path := range []string{
		filepath.Join(toolchain, "rust", "bin", "clippy-driver"),
		filepath.Join(toolchain, "rust", "bin", "rustfmt"),
		filepath.Join(toolchain, "rusty_v8", nativeTarget, "librusty_v8_ptrcomp_sandbox_release_x86_64-unknown-linux-gnu.a.gz"),
		filepath.Join(toolchain, "rusty_v8", nativeTarget, "src_binding_ptrcomp_sandbox_release_x86_64-unknown-linux-gnu.rs"),
	} {
		info, fileErr := os.Lstat(path)
		if fileErr != nil || !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 || info.Size() == 0 {
			return "", errors.Join(fileErr, errors.New("native build toolchain payload differs"))
		}
	}
	rustSource := filepath.Join(toolchain, "rust", "lib", "rustlib", "src", "rust", "library")
	info, err := os.Lstat(rustSource)
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return "", errors.Join(err, errors.New("native Rust source component differs"))
	}
	return cargo.String(), nil
}

func readToolVersionAt(path, name string, window deadline) (toolVersion, error) {
	command := exec.Command(path, "--version")
	command.Env = []string{"PATH=/usr/bin"}
	return readToolVersion(command, name, window)
}

func readToolVersion(command *exec.Cmd, name string, window deadline) (toolVersion, error) {
	output, err := commandOutput(command, window)
	if err != nil {
		return toolVersion{}, fmt.Errorf("read %s version: %w", name, err)
	}
	version := toolVersion{}
	if _, err := fmt.Sscanf(string(output), name+" %d.%d.%d", &version.Major, &version.Minor, &version.Patch); err != nil {
		return toolVersion{}, fmt.Errorf("invalid %s version: %w", name, err)
	}
	return version, nil
}

func (version toolVersion) atLeast(major, minor, patch int) bool {
	return version.Major > major || version.Major == major && (version.Minor > minor || version.Minor == minor && version.Patch >= patch)
}

func (version toolVersion) String() string {
	return fmt.Sprintf("%d.%d.%d", version.Major, version.Minor, version.Patch)
}
