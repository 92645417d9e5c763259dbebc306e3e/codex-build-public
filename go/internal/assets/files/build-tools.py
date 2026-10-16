#!/usr/bin/python3

import hashlib
import json
import os
from pathlib import Path
import re
import subprocess
import sys
import tarfile
import tomllib
import urllib.request


TOOLS = {
    "shuck": ("ewhauser/shuck", ("workspace", "package", "version")),
    "tokei": ("XAMPPRocky/tokei", ("package", "version")),
}
SEMANTIC_RELEASE = re.compile(r"v(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)")


def request(url):
    return urllib.request.Request(url, headers={"Accept": "application/vnd.github+json", "User-Agent": "codex-build"})


def latest_release(repository):
    with urllib.request.urlopen(request(f"https://api.github.com/repos/{repository}/releases/latest"), timeout=120) as response:
        release = json.load(response)
    tag = release.get("tag_name") if isinstance(release, dict) else None
    if not isinstance(tag, str) or SEMANTIC_RELEASE.fullmatch(tag) is None:
        raise RuntimeError(f"latest release tag differs for {repository}")
    return tag, tag[1:]


def remote_source(repository, reference, name, sources_root, output):
    url = f"https://github.com/{repository}.git"
    clone = sources_root / f"{name}.git"
    if clone.is_symlink():
        raise RuntimeError("tool source clone is a symlink")
    if not clone.exists():
        subprocess.run(["/usr/bin/git", "clone", "--bare", "--depth=1", "--no-tags", url, clone], check=True)
    git = ["/usr/bin/git", "--git-dir", str(clone)]
    remote = subprocess.check_output(git + ["remote", "get-url", "origin"], text=True).strip()
    bare = subprocess.check_output(git + ["rev-parse", "--is-bare-repository"], text=True).strip()
    if remote != url or bare != "true":
        raise RuntimeError("tool source clone differs")
    subprocess.run(git + ["fetch", "--depth=1", "--no-tags", "origin", reference], check=True)
    archive = output / f"{name}-source.tar.gz"
    with archive.open("xb") as stream:
        subprocess.run(git + ["archive", "--format=tar.gz", f"--prefix={name}/", "FETCH_HEAD^{commit}"], stdout=stream, check=True)
    extracted = output / f"{name}-source"
    extracted.mkdir()
    with tarfile.open(archive, "r:gz") as source_archive:
        source_archive.extractall(extracted, filter="data")
    root = extracted / name
    if not root.is_dir() or root.is_symlink():
        raise RuntimeError("release source root differs")
    return root


def declared_version(source, fields):
    with (source / "Cargo.toml").open("rb") as manifest:
        value = tomllib.load(manifest)
    for field in fields:
        value = value.get(field) if isinstance(value, dict) else None
    return value


def fetch_locked(cargo, rustc, cargo_home, source):
    environment = {
        "CARGO_HOME": str(cargo_home),
        "HOME": os.environ["HOME"],
        "LANG": "C.UTF-8",
        "LC_ALL": "C.UTF-8",
        "PATH": f"{cargo.parent}:/usr/bin:/bin",
        "RUSTC": str(rustc),
        "RUSTC_BOOTSTRAP": "1",
        "TMPDIR": os.environ["TMPDIR"],
    }
    completed = subprocess.run(
        [cargo, "fetch", "--locked", "--manifest-path", source / "Cargo.toml"],
        env=environment,
        stdout=subprocess.PIPE,
        stderr=subprocess.STDOUT,
        text=True,
    )
    sys.stdout.write(completed.stdout)
    if completed.returncode != 0:
        raise subprocess.CalledProcessError(completed.returncode, completed.args)


def main():
    if len(sys.argv) != 7 or sys.argv[1] != "prepare":
        raise RuntimeError("build tool preparation arguments differ")
    cargo_path = Path(sys.argv[2])
    cargo = cargo_path.resolve(strict=True)
    rustc = cargo_path.with_name("rustc").resolve(strict=True)
    cargo_home = Path(sys.argv[3])
    output = Path(sys.argv[4])
    sources_root = Path(sys.argv[5])
    validate_v8(cargo_path.parents[2], Path(sys.argv[6]))
    if cargo_path.name != "cargo" or output.exists():
        raise RuntimeError("build tool preparation paths differ")
    if sources_root.is_symlink():
        raise RuntimeError("tool source root is a symlink")
    cargo_home.mkdir(parents=True, exist_ok=True)
    sources_root.mkdir(parents=True, exist_ok=True)
    output.mkdir()
    metadata = {}
    for name, (repository, version_fields) in TOOLS.items():
        tag, version = latest_release(repository)
        source = remote_source(repository, f"refs/tags/{tag}", name, sources_root, output)
        if declared_version(source, version_fields) != version or not (source / "Cargo.lock").is_file():
            raise RuntimeError(f"{name} release source version differs")
        fetch_locked(cargo, rustc, cargo_home, source)
        metadata[name.title()] = {"Name": name, "Version": version}
    (output / "metadata.json").write_text(json.dumps(metadata, indent=2, sort_keys=True) + "\n")


def validate_v8(toolchain, source):
    if declared_version(source / "codex-rs", ("workspace", "package", "version")) != "0.0.0":
        raise RuntimeError("BuildInstall requires unstamped workspace version 0.0.0")
    native = json.loads((toolchain / "native-toolchain.json").read_text())
    with (source / "codex-rs/Cargo.lock").open("rb") as stream:
        versions = [item["version"] for item in tomllib.load(stream)["package"] if item["name"] == "v8"]
    if versions != [native["rusty_v8"]] or native["target"] != "x86_64-unknown-linux-gnu":
        raise RuntimeError("BuildInstall V8 version or GNU target differs")
    version, target = versions[0], native["target"]
    stem = f"rusty_v8_ptrcomp_sandbox_release_{target}"
    manifest = stem + ".sha256"
    trusted = source / "third_party/v8" / f"rusty_v8_{version.replace('.', '_')}_release_manifests.sha256"
    pins = [line.split() for line in trusted.read_text().splitlines()]
    expected = [row[0] for row in pins if len(row) == 2 and row[1] == manifest]
    url = f"https://github.com/openai/codex/releases/download/rusty-v8-v{version}/{manifest}"
    with urllib.request.urlopen(request(url), timeout=120) as response:
        data = response.read()
    if expected != [hashlib.sha256(data).hexdigest()]:
        raise RuntimeError("BuildInstall V8 checksum manifest differs")
    files = {f"lib{stem}.a.gz", f"src_binding_ptrcomp_sandbox_release_{target}.rs"}
    rows = [line.split() for line in data.decode("ascii").splitlines()]
    if len(rows) != 2 or any(len(row) != 2 for row in rows) or {row[1] for row in rows} != files:
        raise RuntimeError("BuildInstall V8 checksum inventory differs")
    for checksum, name in rows:
        path = toolchain / "rusty_v8" / target / name
        if path.is_symlink() or not path.is_file():
            raise RuntimeError("BuildInstall V8 payload is not a regular file")
        with path.open("rb") as stream:
            if hashlib.file_digest(stream, "sha256").hexdigest() != checksum:
                raise RuntimeError(f"BuildInstall V8 payload differs: {name}")


if __name__ == "__main__":
    main()
