import ast
import json
import os
from pathlib import Path
import re
import stat
import subprocess
import sys


WORKSPACE = Path("/workspace/codex-rs")
OUTPUT = Path("/result")
CARGO = "/toolchain/rust/bin/cargo"
PREPARATION = OUTPUT / "runner-test-preparation.json"
DIAGNOSTIC_LIMIT = 2048


def require(condition, message):
    if not condition:
        raise ValueError(message)


def bounded(value):
    return value.encode("utf-8")[:DIAGNOSTIC_LIMIT].decode("utf-8", errors="ignore")


def canonical(value):
    text = json.dumps(value, ensure_ascii=False, indent=2, sort_keys=True)
    for character in ("&", "<", ">", chr(0x2028), chr(0x2029)):
        text = text.replace(character, f"\\u{ord(character):04x}")
    return (text + "\n").encode("utf-8")


def publish_preparation(phase, domain, diagnostic):
    pending = PREPARATION.with_suffix(".pending")
    descriptor = os.open(pending, os.O_WRONLY | os.O_CREAT | os.O_EXCL | os.O_NOFOLLOW, 0o600)
    with os.fdopen(descriptor, "wb") as stream:
        stream.write(canonical({"Phase": phase, "FailureDomain": domain, "Diagnostic": bounded(diagnostic)}))
    os.replace(pending, PREPARATION)


class ProductFailure(Exception):
    def __init__(self, status, diagnostic):
        self.status = status
        super().__init__(diagnostic)


def output_tail(path):
    with path.open("rb") as stream:
        stream.seek(max(0, os.fstat(stream.fileno()).st_size - DIAGNOSTIC_LIMIT))
        return stream.read(DIAGNOSTIC_LIMIT).decode("utf-8", errors="replace")


def capture(command, name, environment=None):
    stdout_path, stderr_path = OUTPUT / (name + ".jsonl"), OUTPUT / (name + ".stderr.log")
    descriptors = []
    try:
        for path in (stdout_path, stderr_path):
            descriptors.append(os.open(path, os.O_WRONLY | os.O_CREAT | os.O_EXCL | os.O_NOFOLLOW, 0o600))
        result = subprocess.run(command, cwd=WORKSPACE, env=environment, stdout=descriptors[0], stderr=descriptors[1], check=False)
    finally:
        for descriptor in descriptors:
            os.close(descriptor)
    if result.returncode:
        diagnostic = ""
        with stdout_path.open("rb") as stream:
            for line in stream:
                try:
                    message = json.loads(line)
                except (ValueError, UnicodeDecodeError):
                    continue
                if isinstance(message, dict) and message.get("reason") == "compiler-message" and isinstance(message.get("message"), dict) and message["message"].get("level") == "error":
                    diagnostic = message["message"].get("rendered") or message["message"].get("message", "")
                    break
        failure = f"{name} exited {result.returncode}; output={stdout_path}\n{diagnostic}\n{output_tail(stderr_path)}"
        if isinstance(diagnostic, str) and diagnostic.strip():
            raise ProductFailure(result.returncode, failure)
        raise ValueError(failure)
    return stdout_path


def compilation_arguments(arguments):
    removed_flags = {"--no-run", "--fail-fast", "--ff", "--no-fail-fast", "--nff", "--cargo-quiet"}
    removed_values = {"--cargo-message-format", "--max-fail", "--debugger", "--tracer", "--no-tests"}
    selected, index = [], 2
    while index < len(arguments):
        argument = arguments[index]
        if argument == "--":
            names = arguments[index + 2:]
            if arguments[index + 1:index + 2] == ["--exact"] and names and all(not name.startswith("-") for name in names):
                return [CARGO, "nextest", "list", "--message-format=json", "--cargo-message-format=json", *selected], names
            selected.extend(arguments[index:])
            break
        option, separator, value = argument.partition("=")
        if option in removed_values:
            if not separator:
                index += 1
                require(index < len(arguments), f"missing preparation option value: {option}")
                value = arguments[index]
            require(value, f"empty preparation option value: {option}")
        elif argument not in removed_flags:
            selected.append(argument)
        index += 1
    return [CARGO, "nextest", "run", "--no-run", "--cargo-message-format=json", *selected], []


def build_context(arguments):
    values = {
        "--features": "--features", "-F": "--features", "--cargo-profile": "--profile",
        "--target": "--target", "--target-dir": "--target-dir", "--manifest-path": "--manifest-path",
        "--config": "--config", "-Z": "-Z", "--build-jobs": "--jobs",
    }
    flags = {"--release", "-r", "--all-features", "--no-default-features", "--ignore-rust-version", "--offline", "--frozen"}
    result, index = [], 2
    while index < len(arguments) and arguments[index] != "--":
        argument = arguments[index]
        option, separator, value = argument.partition("=")
        if option in values:
            if not separator:
                index += 1
                require(index < len(arguments), f"missing build option value: {option}")
                value = arguments[index]
            result.extend((values[option], value))
        elif argument in flags:
            result.append(argument)
        elif argument.startswith(("-F", "-Z")) and len(argument) > 2:
            result.extend((values[argument[:2]], argument[2:]))
        index += 1
    if not any(option in result for option in ("--profile", "--release", "-r")):
        result.extend(("--profile", "test"))
    return result


def executable(path):
    candidate = Path(path)
    require(candidate.is_absolute() and str(candidate) == path and os.path.realpath(path) == path, f"noncanonical executable: {path}")
    require(any(candidate.is_relative_to(root) for root in (Path("/cache/target"), Path("/cache/build"))), f"executable outside projected Cargo caches: {path}")
    require(stat.S_ISREG(os.lstat(path).st_mode) and os.access(path, os.X_OK), f"missing executable artifact: {path}")
    return path


def compiler_artifacts(path, packages):
    selected, programs, features, listing = set(), {}, {}, None
    with path.open("rb") as stream:
        for line in stream:
            artifact = json.loads(line)
            require(isinstance(artifact, dict), "Cargo artifact output is not an object")
            if "rust-suites" in artifact:
                require(listing is None and isinstance(artifact["rust-suites"], dict), "native test listing differs")
                listing = artifact["rust-suites"]
            if artifact.get("reason") != "compiler-artifact" or not artifact.get("executable"):
                continue
            target = artifact["target"]
            require(isinstance(target, dict) and isinstance(artifact.get("profile"), dict), "Cargo artifact target or profile is absent")
            test = artifact.get("profile", {}).get("test") or any(kind in target["kind"] for kind in ("test", "bench"))
            if not test and "bin" not in target["kind"]:
                continue
            identifier = artifact["package_id"]
            require(identifier in packages, f"non-workspace executable package: {identifier}")
            require(any(candidate["name"] == target["name"] and candidate["src_path"] == target["src_path"] and candidate["kind"] == target["kind"] for candidate in packages[identifier]["targets"]), f"artifact target differs: {target['name']}")
            program = executable(artifact["executable"])
            if test:
                selected.add(identifier)
                features.setdefault(identifier, set()).update(artifact.get("features", []))
            else:
                key = identifier, target["name"]
                require(key not in programs, f"duplicate binary artifact: {key}")
                programs[key] = program
    return selected, programs, features, listing


def required_binaries(packages, selected, features):
    labels, required = {}, set()
    for identifier, package in packages.items():
        directory = Path(package["manifest_path"]).parent
        require(directory.is_relative_to(WORKSPACE), f"package outside source workspace: {directory}")
        source_path = directory.relative_to(WORKSPACE.parent).as_posix()
        for target in package["targets"]:
            if "bin" not in target["kind"]:
                continue
            key = identifier, target["name"]
            label = f"//{source_path}:{target['name']}"
            require(label not in labels, f"ambiguous binary label: {label}")
            labels[label] = key
            if identifier in selected and set(target.get("required-features", [])) <= features.get(identifier, set()):
                required.add(key)
    for identifier in sorted(selected):
        path = Path(packages[identifier]["manifest_path"]).with_name("BUILD.bazel")
        require(stat.S_ISREG(os.lstat(path).st_mode), f"missing regular binary contract: {path}")
        tree = ast.parse(path.read_text())
        declarations = [node.value for node in tree.body if isinstance(node, ast.Expr) and isinstance(node.value, ast.Call) and isinstance(node.value.func, ast.Name) and node.value.func.id == "codex_rust_crate"]
        require(len(declarations) == 1, f"expected one codex_rust_crate binary contract: {path}")
        require(not declarations[0].args and all(keyword.arg is not None for keyword in declarations[0].keywords), f"binary contract has unresolved arguments: {path}")
        require(len({keyword.arg for keyword in declarations[0].keywords}) == len(declarations[0].keywords), f"binary contract repeats a field: {path}")
        for keyword in declarations[0].keywords:
            if keyword.arg not in ("extra_binaries", "extra_binaries_non_windows"):
                continue
            dependencies = ast.literal_eval(keyword.value)
            require(isinstance(dependencies, list) and all(isinstance(label, str) for label in dependencies), f"binary dependencies must be literal labels: {path}")
            for label in dependencies:
                require(label in labels, f"binary label has no unique Cargo target: {label}")
                required.add(labels[label])
    return required


def executable_environment(required, programs):
    bindings, paths = {}, set()
    for key in sorted(required):
        require(key in programs, f"required binary artifact is absent: {key}")
        path = executable(programs[key])
        require(path not in paths, f"binary artifacts share one executable: {path}")
        paths.add(path)
        name = key[1]
        require(re.fullmatch(r"[A-Za-z0-9_-]+", name), f"binary name cannot form an environment key: {name}")
        for prefix in ("CARGO_BIN_EXE_", "NEXTEST_BIN_EXE_"):
            for spelling in sorted({name, name.replace("-", "_")}):
                environment = prefix + spelling
                require(environment not in bindings, f"binary environment collision: {environment}")
                bindings[environment] = path
    return bindings


def main(arguments):
    phase = "Metadata"
    try:
        require(len(arguments) >= 2 and arguments[:2] == ["/toolchain/bin/just", "test"] and all(arguments), "expected the admitted just test argv")
        publish_preparation(phase, "Helper", "binary preparation did not complete")
        metadata_path = capture([CARGO, "metadata", "--locked", "--no-deps", "--format-version=1"], "runner-workspace-metadata")
        metadata = json.loads(metadata_path.read_bytes())
        require(isinstance(metadata, dict), "Cargo metadata is not an object")
        require(metadata["workspace_root"] == str(WORKSPACE), "Cargo workspace differs from frozen source")
        members = set(metadata["workspace_members"])
        packages = {package["id"]: package for package in metadata["packages"] if package["id"] in members}
        require(set(packages) == members, "Cargo workspace package inventory is incomplete")
        phase = "SelectedTestBuild"
        publish_preparation(phase, "Helper", "selected test compilation did not complete")
        command, exact_names = compilation_arguments(arguments)
        compiled = capture(command, "runner-selected-tests", dict(os.environ, NEXTEST_PROFILE="local"))
        selected, programs, features, listing = compiler_artifacts(compiled, packages)
        require(selected, "native selection produced no test executable artifacts")
        if exact_names:
            require(listing is not None, "native test listing is absent")
            names = {name for suite in listing.values() for name, test in suite["testcases"].items() if test["filter-match"]["status"] == "matches"}
            for name in exact_names:
                if name not in names:
                    raise ProductFailure(4, f"no tests to run for exact native name: {name}")
        phase = "BinaryContract"
        publish_preparation(phase, "Helper", "source binary contracts did not complete")
        required = required_binaries(packages, selected, features)
        missing = required - programs.keys()
        if missing:
            phase = "BinaryBuild"
            publish_preparation(phase, "Helper", "required binary compilation did not complete")
            command = [CARGO, "build", "--locked", "--message-format=json", *build_context(arguments)]
            for identifier in sorted(selected | {identifier for identifier, _ in missing}):
                command.extend(("--package", identifier))
            for name in sorted({name for _, name in missing}):
                command.extend(("--bin", name))
            compiled = capture(command, "runner-required-binaries")
            tests, built, _, _ = compiler_artifacts(compiled, packages)
            require(not tests and set(built) == missing, "required binary build did not produce the exact target set")
            programs.update(built)
        phase = "BinaryBindings"
        publish_preparation(phase, "Helper", "executable bindings did not complete")
        environment = dict(os.environ)
        environment.update(executable_environment(required, programs))
        phase = "TestExecution"
        publish_preparation(phase, "None", "")
        os.execve(arguments[0], arguments, environment)
    except ProductFailure as error:
        publish_preparation(phase, "Product", str(error))
        print(f"RunnerTestPreparation/{phase}: {bounded(str(error))}", file=sys.stderr)
        return error.status if 0 < error.status < 256 else 1
    except (OSError, ValueError, TypeError, KeyError, SyntaxError) as error:
        publish_preparation(phase, "Helper", str(error))
        print(f"RunnerTestPreparation/{phase}: {bounded(str(error))}", file=sys.stderr)
        return 65


if __name__ == "__main__":
    raise SystemExit(main(sys.argv[1:]))
