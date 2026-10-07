#!/usr/bin/python3

import ctypes
import gzip
import json
import os
from pathlib import Path
import re
import shutil
import subprocess
import sys
import tarfile
import urllib.request


GSTREAMER_VERSION = "1.28.6"
GSTREAMER_SOURCE = "https://gitlab.freedesktop.org/gstreamer/gstreamer.git"
RUST_VERSION = "1.95.0"
RUSTUP_SOURCE = (
    "https://static.rust-lang.org/rustup/dist/x86_64-unknown-linux-gnu/rustup-init"
)
NEXTEST_VERSION = "0.9.131"
NEXTEST_SOURCE = (
    f"https://github.com/nextest-rs/nextest/releases/download/cargo-nextest-{NEXTEST_VERSION}/"
    f"cargo-nextest-{NEXTEST_VERSION}-x86_64-unknown-linux-gnu.tar.gz"
)
SCCACHE_VERSION = "0.15.0"
SCCACHE_SOURCE = (
    f"https://github.com/mozilla/sccache/releases/download/v{SCCACHE_VERSION}/"
    f"sccache-v{SCCACHE_VERSION}-x86_64-unknown-linux-musl.tar.gz"
)
V8_VERSION = "150.4.0"
V8_RELEASE = f"https://github.com/openai/codex/releases/download/rusty-v8-v{V8_VERSION}"
V8_ARCHIVE = "librusty_v8_ptrcomp_sandbox_release_x86_64-unknown-linux-gnu.a.gz"
V8_BINDING = "src_binding_ptrcomp_sandbox_release_x86_64-unknown-linux-gnu.rs"
BAZELISK_VERSION = "1.29.0"
BAZELISK_SOURCE = (
    f"https://github.com/bazelbuild/bazelisk/releases/download/v{BAZELISK_VERSION}/"
    "bazelisk-linux-amd64"
)
JUST_VERSION = "1.57.0"
JUST_SOURCE = f"https://github.com/casey/just/releases/download/{JUST_VERSION}/just-{JUST_VERSION}-x86_64-unknown-linux-musl.tar.gz"
DOTSLASH_VERSION = "0.5.9"
DOTSLASH_SOURCE = f"https://github.com/facebook/dotslash/releases/download/v{DOTSLASH_VERSION}/dotslash-ubuntu-22.04.x86_64.tar.gz"
UV_VERSION = "0.11.8"
UV_SOURCE = f"https://github.com/astral-sh/uv/releases/download/{UV_VERSION}/uv-x86_64-unknown-linux-gnu.tar.gz"
TARGET = "x86_64-unknown-linux-gnu"
PLUGINS = (
    "libgstapp.so",
    "libgstaudioconvert.so",
    "libgstaudioresample.so",
    "libgstcoreelements.so",
    "libgstopus.so",
    "libgstrtp.so",
    "libgstrtpmanager.so",
)


def run(arguments, *, cwd=None, environment=None):
    completed = subprocess.run(
        arguments,
        cwd=cwd,
        env=environment,
        stdout=subprocess.PIPE,
        stderr=subprocess.STDOUT,
        text=True,
    )
    if completed.returncode:
        sys.stderr.write(completed.stdout[-32768:])
        raise subprocess.CalledProcessError(completed.returncode, arguments)
    sys.stdout.write(completed.stdout)


def sync_directory(path):
    descriptor = os.open(path, os.O_RDONLY | os.O_DIRECTORY)
    try:
        os.fsync(descriptor)
    finally:
        os.close(descriptor)


def exchange_directories(left, right):
    renameat2 = ctypes.CDLL(None, use_errno=True).renameat2
    renameat2.argtypes = (
        ctypes.c_int,
        ctypes.c_char_p,
        ctypes.c_int,
        ctypes.c_char_p,
        ctypes.c_uint,
    )
    renameat2.restype = ctypes.c_int
    if renameat2(-100, os.fsencode(left), -100, os.fsencode(right), 2) != 0:
        error = ctypes.get_errno()
        raise OSError(error, os.strerror(error))


def write_json(path, value):
    pending = path.with_name(f".{path.name}.stage")
    pending.write_text(json.dumps(value, indent=2, sort_keys=True) + "\n")
    os.replace(pending, path)


def needed(path):
    output = subprocess.run(
        ["/usr/bin/readelf", "-d", path],
        check=True,
        stdout=subprocess.PIPE,
        text=True,
    ).stdout
    return sorted(set(re.findall(r"Shared library: \[([^]]+)\]", output)))


def copy_file(source, destination):
    destination.parent.mkdir(parents=True, exist_ok=True)
    shutil.copy2(source.resolve(strict=True), destination)


def prepare_runtime(sdk, runtime):
    library_directory = runtime / "lib"
    plugin_directory = library_directory / "gstreamer-1.0"
    plugin_directory.mkdir(parents=True)
    libraries = []
    for source in sorted((sdk / "lib").glob("*.so*")):
        if not source.is_file():
            continue
        destination = library_directory / source.name
        copy_file(source, destination)
        libraries.append({"path": f"lib/{source.name}", "imports": needed(destination)})
    plugins = []
    for name in PLUGINS:
        source = sdk / "lib" / "gstreamer-1.0" / name
        if not source.is_file():
            raise RuntimeError(f"required GStreamer plugin is absent: {name}")
        destination = plugin_directory / name
        copy_file(source, destination)
        path = f"lib/gstreamer-1.0/{name}"
        plugins.append(path)
        libraries.append({"path": path, "imports": needed(destination)})
    scanner_source = sdk / "libexec" / "gstreamer-1.0" / "gst-plugin-scanner"
    scanner = runtime / "libexec" / "gstreamer-1.0" / "gst-plugin-scanner"
    if not scanner_source.is_file():
        raise RuntimeError("native GStreamer plugin scanner is absent")
    copy_file(scanner_source, scanner)
    libraries.append(
        {
            "path": "libexec/gstreamer-1.0/gst-plugin-scanner",
            "imports": needed(scanner),
        }
    )
    if not libraries:
        raise RuntimeError("native voice runtime is empty")
    write_json(
        runtime / "runtime.json",
        {
            "libraries": sorted(libraries, key=lambda item: item["path"]),
            "plugins": plugins,
            "schemaVersion": 1,
            "target": TARGET,
        },
    )


def build_voice(attempt, stage):
    source = attempt / "source"
    build = attempt / "build"
    home = attempt / "voice-home"
    temporary = attempt / "voice-tmp"
    home.mkdir()
    temporary.mkdir()
    environment = {
        "CC": "/usr/bin/cc",
        "CXX": "/usr/bin/c++",
        "HOME": str(home),
        "LANG": "C.UTF-8",
        "LC_ALL": "C.UTF-8",
        "PATH": "/usr/bin:/bin",
        "PKG_CONFIG_PATH": "",
        "TMPDIR": str(temporary),
    }
    run(
        [
            "/usr/bin/git",
            "clone",
            "--depth",
            "1",
            "--branch",
            GSTREAMER_VERSION,
            GSTREAMER_SOURCE,
            source,
        ],
        environment=environment,
    )
    tag = subprocess.run(
        ["/usr/bin/git", "describe", "--tags", "--exact-match"],
        cwd=source,
        env=environment,
        check=True,
        stdout=subprocess.PIPE,
        text=True,
    ).stdout.strip()
    if tag != GSTREAMER_VERSION:
        raise RuntimeError("GStreamer source tag differs")
    sdk = stage / "voice" / "sdk"
    setup = [
        "/usr/bin/meson",
        "setup",
        build,
        source,
        f"--prefix={sdk}",
        "--libdir=lib",
        "--buildtype=release",
        "--default-library=shared",
        "--wrap-mode=nofallback",
        "-Dauto_features=disabled",
        "-Dbase=enabled",
        "-Dgood=enabled",
        "-Dugly=disabled",
        "-Dbad=disabled",
        "-Dlibav=disabled",
        "-Ddevtools=disabled",
        "-Dges=disabled",
        "-Drtsp_server=disabled",
        "-Dpython=disabled",
        "-Dgst-examples=disabled",
        "-Dtests=disabled",
        "-Dexamples=disabled",
        "-Dbenchmarks=disabled",
        "-Dtools=enabled",
        "-Dintrospection=disabled",
        "-Dnls=disabled",
        "-Ddoc=disabled",
        "-Dgtk_doc=disabled",
        "-Dorc=disabled",
        "-Dpkgconfig.relocatable=true",
        "-Dc_link_args=-Wl,-rpath,$ORIGIN:$ORIGIN/..",
        "-Dcpp_link_args=-Wl,-rpath,$ORIGIN:$ORIGIN/..",
        "-Dgstreamer:gst_parse=false",
        "-Dgstreamer:registry=true",
        "-Dgstreamer:tracer_hooks=false",
        "-Dgstreamer:check=enabled",
        "-Dgstreamer:tools=enabled",
        "-Dgstreamer:ptp-helper=disabled",
        "-Dgst-plugins-base:app=enabled",
        "-Dgst-plugins-base:audioconvert=enabled",
        "-Dgst-plugins-base:audioresample=enabled",
        "-Dgst-plugins-base:opus=enabled",
        "-Dgst-plugins-base:gl=disabled",
        "-Dgst-plugins-good:rtp=enabled",
        "-Dgst-plugins-good:rtpmanager=enabled",
    ]
    run(setup, environment=environment)
    run(["/usr/bin/meson", "compile", "-C", build, "-j", "16"], environment=environment)
    run(["/usr/bin/meson", "install", "-C", build, "--no-rebuild"], environment=environment)
    voice = stage / "voice"
    write_json(sdk / "sdk.json", {"schemaVersion": 1, "target": TARGET})
    prepare_runtime(sdk, voice / "runtime")


def tool_version(path):
    if not path.is_file() or path.is_symlink():
        return ""
    completed = subprocess.run(
        [path, "--version"],
        check=False,
        stdout=subprocess.PIPE,
        stderr=subprocess.DEVNULL,
        text=True,
    )
    return completed.stdout.strip() if completed.returncode == 0 else ""


def semantic_version_report(report, name):
    first_line = report.splitlines()[0] if report else ""
    match = re.fullmatch(
        rf"{re.escape(name)} ([0-9]+\.[0-9]+\.[0-9]+)(?: .*)?",
        first_line,
    )
    return match.group(1) if match else ""


def semantic_version(path, name):
    return semantic_version_report(tool_version(path), name)


def nextest_version(path):
    return semantic_version(path, "cargo-nextest")


def sccache_version(path):
    if not path.is_file() or path.is_symlink():
        return ""
    completed = subprocess.run(
        ["sccache", "--version"],
        executable=path,
        check=False,
        stdout=subprocess.PIPE,
        stderr=subprocess.PIPE,
        text=True,
    )
    if completed.returncode != 0:
        diagnostic = completed.stderr.strip() or completed.stdout.strip() or "no diagnostic output"
        raise RuntimeError(f"sccache release execution failed: {diagnostic[-4096:]}")
    return semantic_version_report(completed.stdout, "sccache")


def bazelisk_version(path):
    completed = subprocess.run(
        [path, "bazeliskVersion"],
        check=False,
        stdout=subprocess.PIPE,
        stderr=subprocess.DEVNULL,
        text=True,
    )
    prefix = "Bazelisk version: v"
    output = completed.stdout.strip()
    return output.removeprefix(prefix) if completed.returncode == 0 and output.startswith(prefix) else ""


def download(source_url, destination):
    request = urllib.request.Request(source_url, headers={"User-Agent": "codex-build"})
    with urllib.request.urlopen(request, timeout=120) as response:
        with destination.open("xb") as output:
            shutil.copyfileobj(response, output)


def install_tool(
    attempt,
    stage,
    name,
    source_url,
    member_name,
    expected,
    version_reader=tool_version,
):
    archive_path = attempt / f"{name}.tar.gz"
    download(source_url, archive_path)
    pending = stage / "bin" / f".{name}.stage"
    with tarfile.open(archive_path, "r:gz") as archive:
        member = archive.getmember(member_name)
        if not member.isfile() or member.issym() or member.islnk():
            raise RuntimeError(f"{name} release payload differs")
        source = archive.extractfile(member)
        if source is None:
            raise RuntimeError(f"{name} release payload differs")
        with source, pending.open("xb") as destination:
            shutil.copyfileobj(source, destination)
    pending.chmod(0o755)
    if version_reader(pending) != expected:
        raise RuntimeError(f"{name} release version differs")
    os.replace(pending, stage / "bin" / name)


def install_executable(
    attempt,
    stage,
    name,
    source_url,
    expected,
    version_reader=tool_version,
):
    downloaded = attempt / f"{name}.download"
    download(source_url, downloaded)
    pending = stage / "bin" / f".{name}.stage"
    with downloaded.open("rb") as source, pending.open("xb") as destination:
        shutil.copyfileobj(source, destination)
    pending.chmod(0o755)
    if version_reader(pending) != expected:
        raise RuntimeError(f"{name} release version differs")
    os.replace(pending, stage / "bin" / name)


def bootstrap_rust(attempt, stage):
    initializer = attempt / "rustup-init"
    rustup_home = attempt / "rustup"
    cargo_home = attempt / "cargo"
    home = attempt / "rust-home"
    temporary = attempt / "rust-tmp"
    for path in (rustup_home, cargo_home, home, temporary):
        path.mkdir()
    download(RUSTUP_SOURCE, initializer)
    initializer.chmod(0o700)
    environment = {
        "CARGO_HOME": str(cargo_home),
        "HOME": str(home),
        "LANG": "C.UTF-8",
        "LC_ALL": "C.UTF-8",
        "PATH": "/usr/bin:/bin",
        "RUSTUP_HOME": str(rustup_home),
        "TMPDIR": str(temporary),
    }
    run(
        [
            initializer,
            "-y",
            "--no-modify-path",
            "--profile",
            "minimal",
            "--default-host",
            TARGET,
            "--default-toolchain",
            RUST_VERSION,
            "--component",
            "clippy",
            "--component",
            "rustfmt",
            "--component",
            "rust-src",
        ],
        environment=environment,
    )
    toolchains = [
        path
        for path in (rustup_home / "toolchains").iterdir()
        if path.is_dir() and not path.is_symlink()
    ]
    if len(toolchains) != 1:
        raise RuntimeError("Rust toolchain selection differs")
    source = toolchains[0]
    expected = {
        "cargo": f"cargo {RUST_VERSION}",
        "rustc": f"rustc {RUST_VERSION}",
    }
    for name, prefix in expected.items():
        if not tool_version(source / "bin" / name).startswith(prefix):
            raise RuntimeError(f"Rust {name} version differs")
    if (
        not (source / "bin" / "clippy-driver").is_file()
        or not (source / "bin" / "rustfmt").is_file()
        or not (source / "lib" / "rustlib" / "src" / "rust" / "library").is_dir()
    ):
        raise RuntimeError("Rust component payload differs")
    shutil.copytree(source, stage / "rust", symlinks=True)


def install_v8(attempt, stage):
    root = stage / "rusty_v8" / TARGET
    root.mkdir(parents=True)
    archive = root / V8_ARCHIVE
    binding = root / V8_BINDING
    download(f"{V8_RELEASE}/{V8_ARCHIVE}", archive)
    download(f"{V8_RELEASE}/{V8_BINDING}", binding)
    try:
        with gzip.open(archive, "rb") as source:
            if not source.read(8):
                raise RuntimeError("rusty_v8 archive differs")
    except OSError as error:
        raise RuntimeError("rusty_v8 archive differs") from error
    if binding.stat().st_size == 0:
        raise RuntimeError("rusty_v8 binding differs")


def expected_manifest():
    return {
        "bazelisk": BAZELISK_VERSION,
        "cargo": RUST_VERSION,
        "cargo-nextest": NEXTEST_VERSION,
        "dotslash": DOTSLASH_VERSION,
        "gstreamer": GSTREAMER_VERSION,
        "just": JUST_VERSION,
        "rust": RUST_VERSION,
        "rusty_v8": V8_VERSION,
        "sccache": SCCACHE_VERSION,
        "schemaVersion": 1,
        "target": TARGET,
        "uv": UV_VERSION,
    }


def voice_ready(root, native_state):
    try:
        sdk_state = json.loads((root / "voice" / "sdk" / "sdk.json").read_text())
    except (OSError, ValueError):
        sdk_state = {}
    return (
        native_state.get("gstreamer") == GSTREAMER_VERSION
        and sdk_state == {"schemaVersion": 1, "target": TARGET}
        and (root / "voice" / "runtime" / "runtime.json").is_file()
        and (
            root
            / "voice"
            / "runtime"
            / "libexec"
            / "gstreamer-1.0"
            / "gst-plugin-scanner"
        ).is_file()
    )


def composite_ready(root, native_state):
    expected = expected_manifest()
    if (
        set(native_state) != set(expected)
        or any(
            native_state.get(name) != value for name, value in expected.items()
        )
        or not voice_ready(root, native_state)
    ):
        return False
    expected_tools = {
        "dotslash": f"DotSlash {DOTSLASH_VERSION}",
        "just": f"just {JUST_VERSION}",
        "sccache": f"sccache {SCCACHE_VERSION}",
        "uv": f"uv {UV_VERSION} ({TARGET})",
    }
    if any(
        tool_version(root / "bin" / name) != version
        for name, version in expected_tools.items()
    ):
        return False
    if nextest_version(root / "bin" / "cargo-nextest") != NEXTEST_VERSION:
        return False
    if bazelisk_version(root / "bin" / "bazel") != BAZELISK_VERSION:
        return False
    if not tool_version(root / "rust" / "bin" / "rustc").startswith(f"rustc {RUST_VERSION}"):
        return False
    if not tool_version(root / "rust" / "bin" / "cargo").startswith(f"cargo {RUST_VERSION}"):
        return False
    for path in (
        root / "rust" / "bin" / "clippy-driver",
        root / "rust" / "bin" / "rustfmt",
        root / "rusty_v8" / TARGET / V8_ARCHIVE,
        root / "rusty_v8" / TARGET / V8_BINDING,
    ):
        if not path.is_file() or path.is_symlink() or path.stat().st_size == 0:
            return False
    return (root / "rust" / "lib" / "rustlib" / "src" / "rust" / "library").is_dir()


def main():
    if len(sys.argv) != 6 or sys.argv[1] != "prepare":
        raise SystemExit(2)
    working, scratch, attempt, result = map(Path, sys.argv[2:])
    for path in (working, scratch, attempt):
        if not path.is_absolute() or not path.is_dir() or path.is_symlink():
            raise RuntimeError("native toolchain root differs")
    native = working / "build" / "toolchains" / "native"
    final = native / "toolchain"
    stage = native / ".toolchain.stage"
    native.mkdir(parents=True, exist_ok=True)
    if final.exists() and (not final.is_dir() or final.is_symlink()):
        raise RuntimeError("native toolchain differs")
    if stage.exists() or stage.is_symlink():
        raise RuntimeError("native toolchain publication stage exists")
    try:
        native_manifest = final / "native-toolchain.json"
        try:
            native_state = json.loads(native_manifest.read_text())
        except (OSError, ValueError):
            native_state = {}
        if not isinstance(native_state, dict):
            native_state = {}
        if not composite_ready(final, native_state):
            stage.mkdir()
            (stage / "bin").mkdir()
            bootstrap_rust(attempt, stage)
            install_tool(
                attempt,
                stage,
                "cargo-nextest",
                NEXTEST_SOURCE,
                "cargo-nextest",
                NEXTEST_VERSION,
                nextest_version,
            )
            install_executable(
                attempt,
                stage,
                "bazel",
                BAZELISK_SOURCE,
                BAZELISK_VERSION,
                bazelisk_version,
            )
            install_tool(
                attempt,
                stage,
                "sccache",
                SCCACHE_SOURCE,
                f"sccache-v{SCCACHE_VERSION}-x86_64-unknown-linux-musl/sccache",
                SCCACHE_VERSION,
                sccache_version,
            )
            install_v8(attempt, stage)
            install_tool(
                attempt,
                stage,
                "just",
                JUST_SOURCE,
                "just",
                f"just {JUST_VERSION}",
            )
            install_tool(
                attempt,
                stage,
                "dotslash",
                DOTSLASH_SOURCE,
                "dotslash",
                f"DotSlash {DOTSLASH_VERSION}",
            )
            install_tool(
                attempt,
                stage,
                "uv",
                UV_SOURCE,
                "uv-x86_64-unknown-linux-gnu/uv",
                f"uv {UV_VERSION} ({TARGET})",
            )
            build_voice(attempt, stage)
            write_json(stage / "native-toolchain.json", expected_manifest())
            if final.exists():
                exchange_directories(stage, final)
                shutil.rmtree(stage)
            else:
                os.rename(stage, final)
            sync_directory(native)
        write_json(result, {"Output": str(final), "Result": "Prepared"})
    except BaseException as error:
        if stage.exists() and not stage.is_symlink():
            shutil.rmtree(stage)
        write_json(result, {"Diagnostic": str(error), "Output": "", "Result": "Failed"})


if __name__ == "__main__":
    main()
