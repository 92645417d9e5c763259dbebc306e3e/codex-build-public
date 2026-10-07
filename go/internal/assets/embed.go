package assets

import (
	"bytes"
	_ "embed"
)

//go:embed files/normalize-tui-terminal-chrome.py
var terminalChromeTransform []byte

//go:embed files/regenerate-app-server-schema.py
var appServerSchemaTransform []byte

//go:embed files/config-schema.py
var configSchemaGenerator []byte

//go:embed files/release-roll.py
var releaseRoll []byte

//go:embed files/build-install.zsh
var buildInstall []byte

//go:embed files/build-select.zsh
var buildSelect []byte

//go:embed files/sync.zsh
var publicSync []byte

//go:embed files/build-tools.py
var buildTools []byte

//go:embed files/native-toolchain.py
var nativeToolchain []byte

//go:embed files/runner.env
var runnerEnvironment []byte

//go:embed files/runner-bwrap.args
var runnerBwrapArguments []byte

//go:embed files/runner-stage.zsh
var runnerStage []byte

//go:embed files/runner-binaries.py
var runnerBinaries []byte

func TerminalChromeTransform() []byte  { return bytes.Clone(terminalChromeTransform) }
func AppServerSchemaTransform() []byte { return bytes.Clone(appServerSchemaTransform) }
func ConfigSchemaGenerator() []byte    { return bytes.Clone(configSchemaGenerator) }
func ReleaseRoll() []byte              { return bytes.Clone(releaseRoll) }
func BuildInstall() []byte             { return bytes.Clone(buildInstall) }
func BuildSelect() []byte              { return bytes.Clone(buildSelect) }
func PublicSync() []byte               { return bytes.Clone(publicSync) }
func BuildTools() []byte               { return bytes.Clone(buildTools) }
func NativeToolchain() []byte          { return bytes.Clone(nativeToolchain) }
func RunnerEnvironment() []byte        { return bytes.Clone(runnerEnvironment) }
func RunnerBwrapArguments() []byte     { return bytes.Clone(runnerBwrapArguments) }
func RunnerStage() []byte              { return bytes.Clone(runnerStage) }
func RunnerBinaries() []byte           { return bytes.Clone(runnerBinaries) }
