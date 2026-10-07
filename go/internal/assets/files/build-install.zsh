#!/usr/bin/env zsh
emulate -LR zsh
setopt NO_ALIASES ERR_EXIT NO_UNSET PIPE_FAIL

exec > /result/build.stdout.log 2> /result/build.stderr.log
export CARGO_BUILD_TARGET=x86_64-unknown-linux-gnu
export CARGO_PROFILE_RELEASE_LTO=off
export RUSTFLAGS="$RUSTFLAGS -C target-cpu=native"
build_jobs=$(( $(/usr/bin/nproc) / 2 ))
(( build_jobs > 0 )) || build_jobs=1
export CARGO_BUILD_JOBS=$build_jobs

trap 'cp -- /workspace-lower/codex-rs/Cargo.toml /workspace-lower/codex-rs/Cargo.lock . || exit 1' EXIT
trap 'exit 129' HUP
trap 'exit 130' INT
trap 'exit 143' TERM
sed -i '/^\[workspace.package\]$/,/^\[/ s/^version = "0.0.0"$/version = "'"$1"'"/' Cargo.toml
cargo update --workspace --offline
cargo build --release --locked --verbose -p codex-cli -p codex-code-mode-host --bin codex --bin codex-code-mode-host
RUSTFLAGS="$RUSTFLAGS -C link-arg=-Wl,-rpath,\$ORIGIN/../lib" cargo build --release --locked --verbose -p codex-voice-host --bin codex-voice-host
build_tools_root=/cache/build/tools
for build_tool in shuck tokei ccusage; do
    build_source=$build_tools_root/$build_tool/source
    rm -rf -- "$build_source"
    mkdir -p "$build_source"
    tar -xzf /runner/$build_tool-source.tar.gz --strip-components=1 -C "$build_source"
    build_package=$build_tool
    [[ $build_tool != shuck ]] || build_package=shuck-cli
    if [[ $build_tool == ccusage ]]; then
        build_source+=/rust
        export CCUSAGE_PRICING_JSON_PATH=$build_tools_root/ccusage/pricing.json
        if [[ ! -e $CCUSAGE_PRICING_JSON_PATH ]]; then
            cp -- /runner/ccusage-pricing.json "$CCUSAGE_PRICING_JSON_PATH"
        elif cmp -s -- /runner/ccusage-pricing.json "$CCUSAGE_PRICING_JSON_PATH"; then
            :
        else
            (( $? == 1 )) || exit 1
            cp -- /runner/ccusage-pricing.json "$CCUSAGE_PRICING_JSON_PATH"
        fi
    fi
    CARGO_BUILD_BUILD_DIR="$build_tools_root/$build_tool/build" CARGO_TARGET_DIR="/cache/target/tools/$build_tool" cargo build --release --locked --offline --verbose --manifest-path "$build_source/Cargo.toml" -p "$build_package" --bin "$build_tool"
    install -m 0755 "/cache/target/tools/$build_tool/$CARGO_BUILD_TARGET/release/$build_tool" /result/$build_tool
done
for build_binary in codex codex-code-mode-host codex-voice-host; do
    install -m 0755 /cache/target/$CARGO_BUILD_TARGET/release/$build_binary /result/$build_binary
done
install -m 0755 /runner/rg /result/rg
install -m 0755 /runner/bwrap /result/bwrap
