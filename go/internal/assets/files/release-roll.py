#!/usr/bin/python3

import base64
import ctypes
import json
import os
import re
import shutil
import signal
import stat
import subprocess
import sys
import time
import tomllib
from itertools import groupby
from operator import itemgetter


LANES = "bugfix normalization backend mixed frontend draft".split()
join = os.path.join
dir_name = os.path.dirname
base_name = os.path.basename
REGULAR = stat.S_IFREG
DIRECTORY = stat.S_IFDIR
SYMLINK = stat.S_IFLNK
TRANSFORM = "ExecutableTransform"
SEGMENTS = ("none", "non-schema", "schema", "snapshot")
RESPONSIBILITY_ORDER = {
    "backend": (
        ("Unit", "none"),
        ("Unit", "non-schema"),
        ("Unit", "schema"),
        ("FlatPatch", "schema"),
        ("FlatPatch", "non-schema"),
        ("FlatPatch", "none"),
    ),
    "mixed": (("Unit", "none"), ("Unit", "non-schema"), ("Unit", "schema")),
}
SCHEMA_ROOT = "codex-rs/app-server-protocol/schema"
PROTOCOL_ROOT = "codex-rs/app-server-protocol/src"
SCHEMA_OUTPUTS = ("json", "typescript", "precomputed")
NORMALIZATION_TRANSFORM = ("normalization", "Transform", "normalize-terminal-chrome-for-powerline", "normalize-tui-terminal-chrome", TRANSFORM, "none")
SCHEMA_TRANSFORMS = {
    lane: (lane, "Transform", "regenerate-app-server-schema", "regenerate-app-server-schema", TRANSFORM, "schema")
    for lane in LANES
}
SNAPSHOT_TRANSFORMS = {
    lane: (lane, "Transform", "regenerate-snapshots", "regenerate-snapshots", TRANSFORM, "snapshot")
    for lane in LANES
}
TRANSFORM_METADATA = (NORMALIZATION_TRANSFORM, *SCHEMA_TRANSFORMS.values(), *SNAPSHOT_TRANSFORMS.values())
TRANSFORM_ASSETS = {
    NORMALIZATION_TRANSFORM: ("normalize-tui-terminal-chrome.py", ".", ("codex-rs/tui", "codex-rs/mermaid/src")),
}
MAXIMUM_DIFF_LINES = 1500
DIFF_ARGUMENTS = "--no-ext-diff --no-textconv --no-renames --no-color".split()
INVALID_DIFF_PREFIXES = (b"GIT binary patch", b"Binary files ", b"Subproject commit ", b"old mode 160000", b"new mode 160000", b"new file mode 160000", b"deleted file mode 160000")
ENV = {
    "PATH": "/usr/bin:/bin",
    "HOME": os.environ["HOME"],
    "TMPDIR": os.environ["TMPDIR"],
    "LC_ALL": "C.UTF-8",
    "TZ": "UTC",
    "GIT_CONFIG_NOSYSTEM": "1",
    "GIT_CONFIG_GLOBAL": "/dev/null",
    "GIT_ATTR_NOSYSTEM": "1",
    "GIT_NO_REPLACE_OBJECTS": "1",
    "PYTHONDONTWRITEBYTECODE": "1",
    "PYTHONNOUSERSITE": "1",
    "SOURCE_DATE_EPOCH": "0",
    "GIT_OPTIONAL_LOCKS": "0",
}
for role in ("AUTHOR", "COMMITTER"):
    ENV[f"GIT_{role}_NAME"] = "nobody"
    ENV[f"GIT_{role}_EMAIL"] = "nobody@nobody"
    ENV[f"GIT_{role}_DATE"] = "@0 +0000"
CONFIG = (
    "-c", "core.hooksPath=/dev/null",
    "-c", "commit.gpgSign=false",
    "-c", "core.attributesFile=/dev/null",
    "-c", "core.fsmonitor=false",
    "-c", "core.autocrlf=false",
    "-c", "core.sparseCheckout=false",
    "-c", "diff.algorithm=myers",
    "-c", "diff.renames=false",
    "-c", "diff.orderFile=/dev/null",
)
exists = os.path.lexists


class CommitMetadataError(RuntimeError):
    def __init__(self, commit, field, problem):
        super().__init__(f"Commit={commit} Field={field} Problem={problem}")
        self.commit = commit
        self.field = field
        self.problem = problem

class RollInputError(RuntimeError):
    def __init__(self, scope, field, expected, actual):
        self.scope, self.field, self.expected, self.actual = (roll_input_text(value) for value in (scope, field, expected, actual))
        super().__init__(f"Scope={self.scope} Field={self.field} Expected={self.expected} Actual={self.actual}")

class LaneStructureError(RollInputError):
    pass

def roll_input_text(value):
    return str(value)[:256]

def must(value):
    if not value:
        raise RuntimeError("predicate failed")
    return value

def roll_require(value, unit, field, expected, actual):
    if not value:
        raise RollInputError(unit, field, expected, actual)
    return value

def lane_segments(lane):
    if lane in ("backend", "mixed"):
        return ("none", "non-schema", "schema")
    if lane == "draft":
        return ("none", "schema")
    return ("none",)

def generated_schema_path(path):
    return any(path.startswith(f"{SCHEMA_ROOT}/{output}/") for output in SCHEMA_OUTPUTS)

def kind(path, expected):
    try:
        return stat.S_IFMT(os.lstat(path).st_mode) == expected
    except OSError:
        return False

def remove(path):
    if kind(path, DIRECTORY):
        shutil.rmtree(path)
    elif exists(path):
        os.unlink(path)

def write(path, data, mode=0o600):
    created = False
    try:
        with open(path, "xb") as target:
            created = True
            os.chmod(path, mode)
            target.write(data)
            target.flush()
            os.fsync(target.fileno())
        sync_parent(path)
    except BaseException:
        if created:
            remove(path)
        raise

def sync_parent(path):
    descriptor = os.open(dir_name(path), os.O_RDONLY | os.O_DIRECTORY)
    os.fsync(descriptor)
    os.close(descriptor)

def canonical(value):
    data = json.dumps(value, ensure_ascii=False, indent=2, sort_keys=True)
    for character in "&<>\u2028\u2029":
        data = data.replace(character, f"\\u{ord(character):04x}")
    return (data + "\n").encode()

def unique_json_object(pairs):
    value = {}
    for name, item in pairs:
        must(name not in value)
        value[name] = item
    return value

class Repo:
    def __init__(self, path):
        self.path = os.path.abspath(path)

    def policy(self):
        query = git_process(("-C", self.path, "config", "--name-only", "--get-regexp", r"^(filter\.|diff\..*\.(textconv|command|external)$)"), fixed=False, check=False)
        must(query.returncode in (0, 1) and not query.stdout)
        common = git_process(("-C", self.path, "rev-parse", "--path-format=absolute", "--git-common-dir"), fixed=False).stdout.rstrip(b"\n").decode()
        attributes = git_process(("-C", self.path, "rev-parse", "--path-format=absolute", "--git-path", "info/attributes"), fixed=False).stdout.rstrip(b"\n").decode()
        must(not exists(join(common, "info/grafts")) and not exists(attributes))
        must(not git_process(("-C", self.path, "for-each-ref", "--format=%(refname)", "refs/replace/"), fixed=False).stdout)

    def run(self, *arguments, check=True, data=None, attributes=None, index=None):
        return git_process(("-C", self.path, *arguments), check=check, data=data, attributes=attributes, index=index)

    def bytes(self, *arguments, attributes=None, index=None):
        return self.run(*arguments, attributes=attributes, index=index).stdout

    def text(self, *arguments, attributes=None, index=None):
        return self.bytes(*arguments, attributes=attributes, index=index).decode().removesuffix("\n")

    def ok(self, *arguments, attributes=None, index=None):
        return self.run(*arguments, check=False, attributes=attributes, index=index).returncode == 0

    def branch(self):
        return self.text("branch", "--show-current")

    def sequencer(self):
        names = ("rebase-merge", "rebase-apply", "sequencer", "MERGE_HEAD", "CHERRY_PICK_HEAD", "REVERT_HEAD", "BISECT_LOG")
        return any(exists(self.text("rev-parse", "--path-format=absolute", "--git-path", name)) for name in names)

    def clean(self):
        return not self.sequencer() and not self.bytes("status", "--porcelain=v1", "--untracked-files=all")

    def has_ref(self, reference):
        return self.ok("show-ref", "--verify", "--quiet", reference)


def git_process(arguments, *, fixed=True, check=True, data=None, attributes=None, index=None, environment=ENV):
    environment = dict(environment)
    if attributes:
        environment["GIT_ATTR_SOURCE"] = attributes
    if index:
        environment["GIT_INDEX_FILE"] = index
    command = ["/usr/bin/git", *(CONFIG if fixed else ()), *map(str, arguments)]
    return subprocess.run(command, input=data, stdout=subprocess.PIPE, stderr=subprocess.PIPE, env=environment, check=check)

def git_path(path):
    must(isinstance(path, str) and path and not path.startswith("/") and "\0" not in path and ".." not in path.split("/"))
    return path

def formal(value, lane="", patch_kind=""):
    return re.fullmatch(r"[a-z][a-z0-9]*" if lane == "draft" and patch_kind == "Unit" else r"(?![0-9]+-)[a-z0-9]+(?:-[a-z0-9]+)+", value) and all((lane, value) != (metadata[0], metadata[2]) for metadata in TRANSFORM_METADATA)

def lane_path(lane):
    return f"{LANES.index(lane)}-{lane}"

def commit_identity(repository, commit):
    identity = repository.text("show", "-s", "--format=%an%x00%ae%x00%at%x00%cn%x00%ce%x00%ct", commit).split("\0")
    return repository.text("rev-list", "--merges", "--count", f"{commit}^!") == "0" and identity == ["nobody", "nobody@nobody", "0"] * 2

def valid_export_commit(repository, tag):
    commit = repository.text("log", "-1", "--format=%H", "--", ":(literal)" + tag)
    if not commit:
        return False
    paths = changed(repository, commit + "^", commit)
    tracked = tuple(sorted(path.removeprefix(tag + "/") for path in repository.text("ls-tree", "-r", "--name-only", commit, "--", tag).splitlines()))
    return repository.text("show", "-s", "--format=%s", commit) == "update " + tag and commit_identity(repository, commit) and all(path.startswith(tag + "/") for path in paths) and repository.ok("diff", "--quiet", commit, "--", ":(literal)" + tag) and root_inventory(join(repository.path, tag), repository.path) == tracked

def fields(repository, commit):
    if not commit_identity(repository, commit):
        raise CommitMetadataError(commit, "Commit", "malformed")
    return metadata_fields(repository.bytes("show", "-s", "--format=%B", commit).removesuffix(b"\n"), commit)

def metadata_fields(message, identity):
    try:
        lines = message.decode().splitlines(keepends=True)
    except UnicodeError as error:
        raise CommitMetadataError(identity, "Message", "malformed") from error
    variants = {
        "FlatPatch": ("Lane", "FlatPatch", "Form", "Segment"),
        "Unit": ("Lane", "Unit", "ChildPatch", "Form", "Segment"),
        "Transform": ("Lane", "Transform", "TransformPatch", "Form", "Segment"),
    }
    labels = [line.split(": ", 1)[0] for line in lines[1:] if line.endswith("\n") and ": " in line]
    kind = next((name for name, names in variants.items() if labels == list(names)), "")
    if not kind:
        raise CommitMetadataError(identity, "Message", "malformed discriminant")
    names = variants[kind]
    if len(lines) != 2 * len(names) + 1 or lines[1::2] != ["\n"] * len(names) or not all(line.endswith("\n") for line in lines):
        raise CommitMetadataError(identity, "Message", "malformed")
    values = []
    for line, name in zip(lines[2::2], names):
        prefix = name + ": "
        if not line.startswith(prefix) or line == prefix + "\n":
            raise CommitMetadataError(identity, name, "malformed")
        values.append(line[len(prefix):-1])
    lane = values[0]
    group, patch = (values[1], values[1]) if kind == "FlatPatch" else values[1:3]
    form, segment = values[-2:]
    metadata = (lane, kind, group, patch, form, segment)
    git_diff = form == "GitDiff" and segment in lane_segments(lane)
    valid = lane in LANES and formal(patch) and (kind != "Transform" and git_diff and formal(group, lane, kind) and (kind == "FlatPatch" or group != patch) or metadata in TRANSFORM_METADATA)
    if not valid or lines[0] != patch + "\n":
        field = "Subject" if lines[0] != patch + "\n" else "Message"
        raise CommitMetadataError(identity, field, "malformed")
    return metadata

def root_inventory(path, parent, missing=False):
    path, parent = os.path.abspath(path), os.path.abspath(parent)
    must(dir_name(path) == parent and base_name(path).startswith("rust-v"))
    must(kind(parent, DIRECTORY) and os.path.realpath(parent) == parent)
    if not exists(path):
        must(missing)
        return ()
    must(kind(path, DIRECTORY) and os.path.realpath(path) == path)
    files = []
    for current, directories, names in os.walk(path, followlinks=False):
        must(kind(current, DIRECTORY))
        directories.sort()
        names.sort()
        for name in directories:
            node = join(current, name)
            must(kind(node, DIRECTORY))
        for name in names:
            node = join(current, name)
            must(kind(node, REGULAR))
            files.append(os.path.relpath(node, path))
    files.sort()
    return tuple(files)

def release_notes(root, lanes=LANES):
    notes = {}
    for relative in ("notes.md", *(join(lane_path(lane), "notes.md") for lane in lanes)):
        path = join(root, relative)
        if not os.path.lexists(path):
            continue
        must(kind(path, REGULAR) and os.path.realpath(path) == path)
        data = regular_bytes(path)
        text = data.decode("utf-8")
        must(not data or text.endswith("\n"))
        must("\r" not in text and "\x00" not in text)
        must(all(re.fullmatch(r"- \S.*", line) for line in text.splitlines()))
        notes[relative] = data
    return notes

def working_release_notes(root):
    if not os.path.lexists(root):
        return {}
    must(kind(root, DIRECTORY) and os.path.realpath(root) == root)
    for name in os.listdir(root):
        if name == "notes.md":
            continue
        path = join(root, name)
        must(name in {lane_path(lane) for lane in LANES})
        must(kind(path, DIRECTORY) and os.path.realpath(path) == path)
        must(set(os.listdir(path)) <= {"notes.md"})
    return release_notes(root)

def changed(repository, start=None, end=None, *, attributes=None, index=None):
    return [os.fsdecode(item) for item in repository.bytes("diff", *(("--cached",) if start is None else (start, end)), "--name-only", "-z", "--no-renames", "--no-ext-diff", "--no-textconv", attributes=attributes, index=index).split(b"\0") if item]

def canonical_patch_bytes(data):
    lines = data.splitlines(keepends=True)
    must(not any(invalid_diff_line(line) for line in lines))
    return b"".join(line for line in lines if not line.startswith(b"index "))

def patch_bytes(repository, start, end, attributes=None):
    return canonical_patch_bytes(repository.bytes("diff", *DIFF_ARGUMENTS, start, end, attributes=attributes))

def staged_patch_bytes(repository, parent):
    return canonical_patch_bytes(repository.bytes("diff", *DIFF_ARGUMENTS, "--cached", parent, attributes=parent))

def invalid_diff_line(line):
    return line.startswith(INVALID_DIFF_PREFIXES) or line[:1] in (b"+", b"-") and line[1:].startswith(b"Subproject commit ")

def source_change_class(paths, unit):
    roll_require(paths and all(git_path(path) for path in paths), unit, "ChangedPaths", "nonempty canonical changed paths", paths)
    snapshots = [path.endswith(".snap") for path in paths]
    return "snapshot" if all(snapshots) else "mixed" if any(snapshots) else "source"

def validate_protocol_segment(paths, lane, segment, unit):
    protocol = any(path == PROTOCOL_ROOT or path.startswith(PROTOCOL_ROOT + "/") for path in paths)
    roll_require(not protocol or lane in ("backend", "mixed", "draft") and segment == "schema", unit, "Segment", "schema for protocol source in backend, mixed, or draft", f"{lane}/{segment}")

def change_class(repository, lane, form, segment, start=None, end=None, *, attributes=None, index=None):
    paths = set(changed(repository, start, end, attributes=attributes, index=index))
    generated = {path for path in paths if generated_schema_path(path)}
    if form == TRANSFORM and segment == "schema" and not paths:
        return "schema-transform"
    if form == TRANSFORM and segment == "snapshot":
        must(all(path.endswith(".snap") and "/snapshots/" in path or path.endswith(".rs") for path in paths))
        return "snapshot-transform"
    if form == TRANSFORM:
        roll_require(paths and all(git_path(path) for path in paths), lane, "ChangedPaths", "nonempty canonical changed paths", paths)
        if segment == "schema":
            must(segment == "schema" and paths == generated)
            return "schema-transform"
        must(lane == "normalization" and segment == "none" and not generated)
        return "normalization-transform"
    roll_require(form == "GitDiff" and not generated and segment in lane_segments(lane), lane, "Segment", lane_segments(lane), segment)
    validate_protocol_segment(paths, lane, segment, lane)
    classification = source_change_class(paths, lane)
    must(classification == "source")
    return classification

def changed_paths(source):
    tracked = source.bytes("diff", "HEAD", "--name-only", "-z", "--no-renames").split(b"\0")[:-1]
    untracked = source.bytes("ls-files", "--others", "--exclude-standard", "-z").split(b"\0")[:-1]
    return tuple(sorted({os.fsdecode(path) for path in tracked + untracked}))

def rust_format_paths(source, paths):
    return tuple(path for path in paths if path.endswith(".rs") and kind(join(source.path, path), REGULAR))

def native_commit(repository, *arguments):
    repository.run("commit", "--quiet", "--no-verify", *arguments)
    must(repository.clean() and commit_identity(repository, "HEAD"))

def metadata_commit(repository, lane, kind, group, patch, form, segment, title="", amend=False, allow_empty=False):
    labels = {"FlatPatch": (("FlatPatch", group),), "Unit": (("Unit", group), ("ChildPatch", patch)), "Transform": (("Transform", group), ("TransformPatch", patch))}[kind]
    paragraphs = [("Lane", lane), *labels, ("Form", form), ("Segment", segment)]
    arguments = ("--amend", "--no-edit") if amend else tuple(item for pair in (("-m", title or patch), *( ("-m", f"{name}: {value}") for name, value in paragraphs)) for item in pair)
    if allow_empty:
        arguments = ("--allow-empty", *arguments)
    native_commit(repository, *arguments)
    must(fields(repository, "HEAD") == (lane, kind, group, patch, form, segment))

def first_parent(repository, base, head="HEAD"):
    must(repository.ok("merge-base", "--is-ancestor", base, head) and repository.text("rev-list", "--merges", "--count", f"{base}..{head}") == "0")
    count = int(repository.text("rev-list", "--count", "--first-parent", f"{base}..{head}"))
    return [head if distance == 0 else f"{head}~{distance}" for distance in reversed(range(count))]

def validate_history(rows, complete=False, partial=False):
    must(rows and rows[0][2:8] == NORMALIZATION_TRANSFORM)
    must(rows[0][8] == "normalization-transform")
    ordinary = [row for row in rows if row[6] == "GitDiff"]
    last, structures, lane_orders = -1, set(), {}
    for (lane, kind, group), grouped in groupby(ordinary, itemgetter(2, 3, 4)):
        members = list(grouped)
        identity, rank = (lane, kind, group), LANES.index(lane)
        must(identity not in structures and rank >= last)
        structures.add(identity)
        last = rank
        if lane == "bugfix" and kind != "FlatPatch":
            raise LaneStructureError(f"{lane}/{group}", "PatchKind", "FlatPatch", kind)
        trailing_partial = partial and members[-1] == rows[-1]
        roll_require(transport_complete((lane, kind, group, members), trailing_partial), f"{lane}/{group}", "PatchStructure", "one Flat Patch or a Unit with at least two Child Patches", [member[5] for member in members])
        segment = members[0][7]
        must(segment in lane_segments(lane))
        if lane in RESPONSIBILITY_ORDER:
            order = RESPONSIBILITY_ORDER[lane]
            category = (kind, segment)
            if category not in order:
                raise LaneStructureError(f"{lane}/{group}", "PatchKind", "Unit", kind)
            position = order.index(category)
            expected = order
        else:
            position = 0 if lane in ("bugfix", "draft") else int(kind == "FlatPatch")
            expected = "Units before Flat Patches"
        if position < lane_orders.get(lane, 0):
            raise LaneStructureError(f"{lane}/{group}", "LaneOrder", expected, (kind, segment))
        lane_orders[lane] = position
    tail = [row for row in rows[1:] if row[6] == TRANSFORM]
    must(not tail or ordinary and rows[-len(tail):] == tail)
    segments = [row[7] for row in tail]
    must(segments in ([], ["schema"], ["snapshot"], ["schema", "snapshot"]))
    for row in tail:
        metadata = SCHEMA_TRANSFORMS if row[7] == "schema" else SNAPSHOT_TRANSFORMS
        must(row[2] == ordinary[-1][2] and row[2:8] == metadata[row[2]] and row[8] == row[7] + "-transform")
    required = schema_required(rows)
    must("schema" not in segments or required)
    must("snapshot" not in segments or ("schema" in segments) == required)
    must(not complete or "snapshot" in segments)
    return rows

def schema_required(rows):
    return any(row[6] == "GitDiff" and row[7] == "schema" for row in rows)

def history_rows(source, base, head="HEAD"):
    rows = []
    for commit in first_parent(source, base, head):
        try:
            lane, kind, group, patch, form, segment = fields(source, commit)
        except CommitMetadataError as error:
            raise RollInputError(error.commit, error.field, "canonical commit metadata", error.problem) from error
        parent = f"{commit}^"
        classification = change_class(source, lane, form, segment, parent, commit, attributes=parent)
        rows.append((commit, parent, lane, kind, group, patch, form, segment, classification))
    return rows

def projected_patch_rows(rows):
    projected, indexes = [], {}
    for (lane, kind, group), grouped in groupby(rows, itemgetter(2, 3, 4)):
        members = list(grouped)
        indexes[lane] = indexes.get(lane, 0) + 1
        structure_path = f"{indexes[lane]:02d}-{group}"
        for patch_index, row in enumerate(members, 1):
            patch = row[5]
            name = f"{lane_path(lane)}/{structure_path}/{patch_index:03d}-{patch}.diff" if kind == "Unit" else f"{lane_path(lane)}/{structure_path}.diff"
            projected.append((row, name))
    return tuple(projected)

def git_diff_finding(row, artifact, data, enforce_limit=True):
    _, _, lane, patch_kind, group, patch, _, _, _ = row
    lines = len(data.splitlines())
    output = "EmptyGitDiff" if not data else "OversizeGitDiff" if enforce_limit and lines > MAXIMUM_DIFF_LINES else ""
    if not output:
        return None
    return {**transport_result_identity(patch_kind, lane, group, patch), "Result": "Conflict", "Output": output, "ArtifactPath": artifact, "ActualLines": lines, "MaximumLines": MAXIMUM_DIFF_LINES}

def project(source, base, head="HEAD", publication=True, skip_line_limit=False):
    rows = [row for row in validate_history(history_rows(source, base, head)) if row[6] == "GitDiff"]
    projected, findings = [], []
    for row, name in projected_patch_rows(rows):
        commit_id, parent = row[:2]
        data = canonical_patch_bytes(source.bytes("format-patch", *DIFF_ARGUMENTS, "--stdout", "--no-stat", "--no-signature", "--zero-commit", "--format=%B", "-1", commit_id, attributes=parent))
        patch_start = data.index(b"\n\ndiff --git ") + 2
        finding = git_diff_finding(row, name, data[patch_start:], publication and not skip_line_limit)
        if finding:
            findings.append(finding)
        projected.append((name, data))
    return (() if findings else tuple(projected)), tuple(findings)

def roll_format_patch(source, rustfmt):
    paths = changed_paths(source)
    rust = rust_format_paths(source, paths)
    try:
        for path in rust:
            subprocess.run([rustfmt, "--config", "skip_children=true", join(source.path, path)], env=ENV, check=True)
    except (OSError, subprocess.CalledProcessError):
        return False
    return changed_paths(source) == paths

def write_project(root, projected):
    expected = tuple(name for name, _ in projected)
    must(expected == tuple(sorted(expected)) and len(expected) == len(set(expected)) and all(git_path(name) == name for name in expected))
    expected_directories = set()
    for name in expected:
        directory = dir_name(name)
        while directory not in ("", "."):
            expected_directories.add(directory)
            directory = dir_name(directory)
    if not exists(root):
        os.mkdir(root)
        sync_parent(root)
    actual = root_inventory(root, dir_name(root))
    must(set(actual) <= set(expected))
    actual_directories = set()
    for current, directories, names in os.walk(root, followlinks=False):
        relative = os.path.relpath(current, root)
        if relative != ".":
            actual_directories.add(relative)
        must(kind(current, DIRECTORY))
        must(all(kind(join(current, name), DIRECTORY) for name in directories))
        must(all(kind(join(current, name), REGULAR) for name in names))
    must(actual_directories <= expected_directories)
    by_name = dict(projected)
    for name in actual:
        must(regular_bytes(join(root, name)) == by_name[name])
    for name in expected:
        if name not in actual:
            target = join(root, name)
            os.makedirs(dir_name(target), exist_ok=True)
            write(target, by_name[name], 0o644)
    must(root_inventory(root, dir_name(root)) == expected)
    actual_directories = {os.path.relpath(current, root) for current, _, _ in os.walk(root, followlinks=False) if current != root}
    must(actual_directories == expected_directories)
    sync_parent(root)
    return expected

def project_matches_export(source, base, export, head="HEAD"):
    projected, findings = project(source, base, head, publication=False)
    must(not findings)
    lanes = {name.split("/", 1)[0] for name, _ in projected}
    notes = release_notes(export, [lane for lane in LANES if lane_path(lane) in lanes])
    expected = tuple(sorted([name for name, _ in projected] + list(notes)))
    must(root_inventory(export, dir_name(export)) == expected)
    for name, data in projected:
        must(regular_bytes(join(export, name)) == data)

def snapshot_source_path(path):
    path = git_path(path)
    inline = path.endswith(".rs") and "/snapshots/" not in path
    roll_require(path.startswith("codex-rs/") and (inline or "/snapshots/" in path and path.endswith(".snap")), path, "SnapshotTarget", "codex-rs Rust inline or external snapshot", path)
    return path, inline

def rust_tokens(source):
    tokens, index, length = [], 0, len(source)
    while index < length:
        char = source[index]
        if char.isspace():
            index += 1
            continue
        if source.startswith("//", index):
            index = source.find("\n", index)
            index = length if index < 0 else index + 1
            continue
        if source.startswith("/*", index):
            cursor, depth = index + 2, 1
            while cursor < length and depth:
                if source.startswith("/*", cursor):
                    depth += 1
                    cursor += 2
                elif source.startswith("*/", cursor):
                    depth -= 1
                    cursor += 2
                else:
                    cursor += 1
            must(not depth)
            index = cursor
            continue
        start, prefix = index, 0
        if source.startswith(("br", "cr"), index):
            prefix = 2
        elif char in "rbc":
            prefix = 1
        raw = index + prefix < length and source[index + prefix] == "#"
        quote = index + prefix < length and source[index + prefix] == '"'
        if raw or quote:
            cursor, fences = index + prefix, 0
            while cursor < length and source[cursor] == "#":
                fences += 1
                cursor += 1
            if cursor < length and source[cursor] == '"':
                cursor += 1
                if fences or "r" in source[start:start + prefix]:
                    end = source.find('"' + '#' * fences, cursor)
                    must(end >= 0)
                    index = end + fences + 1
                else:
                    escaped = False
                    while cursor < length:
                        current = source[cursor]
                        cursor += 1
                        if current == '"' and not escaped:
                            break
                        escaped = current == "\\" and not escaped
                        if current != "\\":
                            escaped = False
                    must(cursor <= length and source[cursor - 1] == '"')
                    index = cursor
                tokens.append(("literal", start, index, source[start:index]))
                continue
        if char == "'":
            cursor = index + 1
            if cursor < length and source[cursor] == "\\":
                cursor += 2
                if cursor <= length and source[cursor - 1] == "u" and cursor < length and source[cursor] == "{":
                    closing = source.find("}", cursor)
                    must(closing >= 0)
                    cursor = closing + 1
                elif cursor <= length and source[cursor - 1] == "x":
                    cursor += 2
            else:
                cursor += 1
            if 0 < cursor < length and source[cursor] == "'":
                index = cursor + 1
                tokens.append(("literal", start, index, source[start:index]))
                continue
        if char == "_" or char.isalpha():
            index += 1
            while index < length and (source[index] == "_" or source[index].isalnum()):
                index += 1
            tokens.append(("ident", start, index, source[start:index]))
            continue
        index += 1
        tokens.append(("punct", start, index, char))
    return tokens

def rust_string(literal):
    marker = literal.find('"')
    must(marker >= 0 and set(literal[:marker]).issubset(set("r#")))
    prefix = literal[:marker]
    if "r" in prefix:
        fences = prefix.count("#")
        must(literal.endswith('"' + '#' * fences))
        return literal[marker + 1:len(literal) - fences - 1]
    must(prefix == "" and literal.endswith('"'))
    value, index, end = [], marker + 1, len(literal) - 1
    while index < end:
        char = literal[index]
        index += 1
        if char != "\\":
            value.append(char)
            continue
        must(index < end)
        escape = literal[index]
        index += 1
        simple = {'n': '\n', 'r': '\r', 't': '\t', '0': '\0', '\\': '\\', '"': '"', "'": "'"}
        if escape in simple:
            value.append(simple[escape])
        elif escape == "x":
            digits = literal[index:index + 2]
            must(re.fullmatch(r"[0-9A-Fa-f]{2}", digits))
            value.append(chr(int(digits, 16)))
            index += 2
        elif escape == "u":
            match = re.match(r"\{([0-9A-Fa-f_]+)\}", literal[index:])
            must(match is not None)
            value.append(chr(int(match.group(1).replace("_", ""), 16)))
            index += match.end()
        elif escape == "\n":
            while index < end and literal[index] in " \t\r\n":
                index += 1
        else:
            must(False)
    return "".join(value)

def inline_escaped(value):
    return any(ord(char) < 32 and char not in "\n\t\x1b" or 127 <= ord(char) < 160 for char in value)

def normalized_inline(value):
    value = re.sub(r"[^\S\x1c-\x1f]+$", "", value)
    lines = [line[:-1] if line.endswith("\r") else line for line in value.split("\n")]
    if not inline_escaped(value) and lines[0] == "":
        lines = lines[1:]
    indentation = min((len(line) - len(line.lstrip(" \t")) for line in lines if line), default=0) if len(lines) > 1 else 0
    return "\n".join(line[indentation:] for line in lines)

def rendered_inline(value, indentation):
    value = re.sub(r"[^\S\x1c-\x1f]+$", "", value)
    if inline_escaped(value):
        replacements = {"\\": "\\\\", '"': '\\"', "\n": "\\n", "\r": "\\r", "\t": "\\t", "\0": "\\0"}
        body = "".join(replacements.get(char, char if 32 <= ord(char) < 127 or ord(char) >= 160 else f"\\u{{{ord(char):x}}}") for char in value)
        return '"' + body + '"'
    raw = "r" if "\\" in value or '"' in value else ""
    fences = max((len(part) - len(part.lstrip("#")) + 1 for part in value.split('"')[1:]), default=0)
    delimiter = "#" * fences
    body = "".join("\n" + (indentation if line else "") + line for line in value.split("\n")) + "\n" + indentation if "\n" in value else value
    return raw + delimiter + '"' + body + '"' + delimiter

def inline_spans(source):
    tokens = rust_tokens(source)
    pairs, stack, lines = {}, [], {}
    cursor, line = 0, 1
    for position, token in enumerate(tokens):
        line += source.count("\n", cursor, token[1])
        cursor, lines[position] = token[1], line
        if token[3] in "({[":
            stack.append((token[3], position))
        elif token[3] in ")}]":
            must(stack and stack[-1][0] == {")": "(", "}": "{", "]": "["}[token[3]])
            _, opening = stack.pop()
            pairs[opening] = position
    must(not stack)
    spans = []
    for position, token in enumerate(tokens[:-2]):
        if token[0] != "ident" or tokens[position + 1][3] != "!" or tokens[position + 2][3] not in "({[":
            continue
        path_position = position
        while path_position >= 3 and tokens[path_position - 1][3] == ":" and tokens[path_position - 2][3] == ":" and tokens[path_position - 3][0] == "ident":
            path_position -= 3
        if not token[3].startswith("assert_") or not token[3].endswith("_snapshot") or path_position < position and tokens[path_position][3] != "insta":
            continue
        opening, closing = position + 2, pairs.get(position + 2)
        if closing is None:
            continue
        depth, arguments = 0, []
        for inner in tokens[opening + 1:closing]:
            if inner[3] in "({[":
                depth += 1
            if depth == 0:
                arguments.append(inner)
            if inner[3] in ")}]":
                depth -= 1
        while arguments and arguments[-1][3] == ",":
            arguments.pop()
        if len(arguments) >= 2 and arguments[-2][3] == "@" and arguments[-1][0] == "literal":
            line_start = source.rfind("\n", 0, tokens[path_position][1]) + 1
            indentation = source[line_start:tokens[path_position][1]]
            if indentation.strip():
                continue
            literal = arguments[-1]
            spans.append((literal[1], literal[2], lines[position], normalized_inline(rust_string(literal[3])), literal[3], indentation))
    offsets, cursor, total = {}, 0, 0
    for point in sorted({point for span in spans for point in span[:2]}):
        total += len(source[cursor:point].encode())
        cursor, offsets[point] = point, total
    result = [(start, end, offsets[start], offsets[end], line, old, preimage, indentation) for start, end, line, old, preimage, indentation in spans]
    must(all(left[3] <= right[2] for left, right in zip(result, result[1:])))
    return result

def inline_postimage(source, changes):
    for start, end, postimage in sorted(changes, reverse=True):
        source = source[:start] + postimage + source[end:]
    return source

def inline_records(lower, overlay, planned):
    records = {}
    for source_path in sorted(planned):
        pending_name = join(dir_name(source_path), "." + base_name(source_path) + ".pending-snap")
        pending_path = join(overlay, pending_name)
        if not exists(pending_path):
            continue
        must(not os.path.lexists(join(lower, pending_name)) and kind(pending_path, REGULAR))
        batch = [json.loads(line) for line in regular_bytes(pending_path).decode().splitlines()]
        must(batch and all(isinstance(row, dict) and set(row) == {"run_id", "line", "new", "old"} for row in batch))
        latest = {}
        for row in batch:
            must(type(row["line"]) is int and row["line"] > 0 and (row["new"] is None and row["old"] is None or isinstance(row["old"], dict) and isinstance(row["new"], dict)))
            latest[row["line"]] = row
        for row in latest.values():
            if row["new"] is not None:
                old, new = row["old"].get("snapshot"), row["new"].get("snapshot")
                must(isinstance(old, str) and isinstance(new, str) and old != new)
                key, entry = (source_path, row["line"]), (old, new)
                must(key not in records or records[key] == entry)
                records[key] = entry
    grouped, result = {}, []
    for (path, line), entry in sorted(records.items()):
        grouped.setdefault(path, []).append((line, *entry))
    for path, rows in grouped.items():
        lower_name, overlay_name = join(lower, path), join(overlay, path)
        must(kind(lower_name, REGULAR) and kind(overlay_name, REGULAR) and regular_bytes(overlay_name) == regular_bytes(lower_name))
        contents = regular_bytes(lower_name).decode()
        parsed = inline_spans(contents)
        spans = {(span[4], span[5]): span for span in parsed}
        must(len(spans) == len(parsed))
        for line, old, new in rows:
            span = spans.get((line, old))
            if span is None:
                raise RollInputError(path, "InlineSnapshotLine", (line, old), [(point[4], point[5]) for point in parsed if point[4] == line])
            start, end, byte_start, byte_end, _, _, preimage, indentation = span
            postimage = rendered_inline(new, indentation)
            result.append({"End": byte_end, "Line": line, "Path": path, "Postimage": postimage, "Preimage": preimage, "Start": byte_start})
    return result

def inline_target_spans(data):
    return [(span[2], span[3], span[4], span[6]) for span in inline_spans(data.decode())]

def complete_snapshot_targets(lower, overlay):
    selected = set()
    for root, directories, files in os.walk(overlay):
        directories.sort()
        for name in sorted(files):
            relative = os.path.relpath(join(root, name), overlay)
            if relative.endswith(".snap.new") and "/snapshots/" in relative:
                selected.add(relative.removesuffix(".new"))
            elif relative.endswith(".snap") and "/snapshots/" in relative:
                selected.add(relative)
            elif name.startswith(".") and name.endswith(".rs.pending-snap"):
                selected.add(join(dir_name(relative), name[1:].removesuffix(".pending-snap")))
    targets = []
    for relative in sorted(selected):
        _, inline = snapshot_source_path(relative)
        path = join(lower, relative)
        if inline:
            roll_require(kind(path, REGULAR), relative, "SnapshotPreimage", "regular lower Rust source", path)
            data = regular_bytes(path)
            spans = [{"Start": start, "End": end, "Line": line, "Preimage": preimage} for start, end, line, preimage in inline_target_spans(data)]
            roll_require(spans, relative, "InlineSpans", "existing inline snapshot literals", "absent")
            targets.append({"Path": relative, "Kind": "InlineRust", "Current": data.decode(), "ExistingInlineSpans": spans})
        else:
            data = regular_bytes(path) if exists(path) else b""
            targets.append({"Path": relative, "Kind": "ExternalSnapshot", "Current": data.decode(), "ExistingInlineSpans": []})
    return targets

def replace_regular(path, data, mode=0o600):
    pending = path + ".release-roll-pending"
    must(kind(path, REGULAR) and not exists(pending))
    write(pending, data, mode)
    os.replace(pending, path)
    sync_parent(path)

def rust_suite_selection(transient_exclusions):
    return ("--workspace", *(value for name in transient_exclusions for value in ("--exclude", name)))

def rust_suite_package_list(value):
    must(isinstance(value, list) and value == sorted(set(value)) and all(isinstance(name, str) and re.fullmatch(r"[A-Za-z0-9_-]+", name) for name in value))
    return value

def rust_workspace_metadata(source, cargo, cargo_home):
    must(kind(source, DIRECTORY) and kind(cargo, REGULAR) and kind(cargo_home, DIRECTORY))
    environment = {**ENV, "CARGO_HOME": cargo_home, "CARGO_NET_OFFLINE": "true", "CARGO_TERM_COLOR": "never", "HOME": "/nonexistent"}
    result = subprocess.run((cargo, "metadata", "--locked", "--offline", "--no-deps", "--format-version", "1"), cwd=join(source, "codex-rs"), env=environment, stdout=subprocess.PIPE, stderr=sys.stderr.buffer, check=False)
    must(result.returncode == 0)
    return json.loads(result.stdout, object_pairs_hook=unique_json_object)

def rust_suite_packages(arguments):
    must(len(arguments) == 4)
    source, cargo, cargo_home, output = map(os.path.abspath, arguments)
    must(not exists(output))
    metadata = rust_workspace_metadata(source, cargo, cargo_home)
    members = set(metadata.get("workspace_members", ()))
    packages = sorted(package["name"] for package in metadata.get("packages", ()) if package.get("id") in members)
    must(members and len(packages) == len(members) == len(set(packages)) and all(re.fullmatch(r"[A-Za-z0-9_-]+", name) for name in packages))
    transport_write_result(output, packages)

def runner_test_selection(arguments):
    must(len(arguments) >= 6)
    source, cargo, cargo_home, test_source, *test_names, output = arguments
    must(not exists(output))
    target_source = join(source, test_source)
    must(kind(target_source, REGULAR) and os.path.realpath(target_source) == target_source)
    metadata = rust_workspace_metadata(source, cargo, cargo_home)
    members = set(metadata.get("workspace_members", ()))
    candidates = [(package, target) for package in metadata.get("packages", ()) if package.get("id") in members for target in package.get("targets", ()) if target.get("src_path") == target_source]
    roots = []
    if not candidates:
        owners = [package for package in metadata.get("packages", ()) if package.get("id") in members and target_source.startswith(dir_name(package["manifest_path"]) + "/")]
        if len(owners) == 1:
            roots = sorted({
                os.path.relpath(target["src_path"], source)
                for target in owners[0].get("targets", ())
                if target.get("test") is True
                and ("/tests/" not in test_source or "test" in target["kind"])
                and target["src_path"].startswith(source + "/")
                and kind(target["src_path"], REGULAR)
            })[:8]
    selected = []
    if len(candidates) == 1:
        package, target = candidates[0]
        kinds = target.get("kind")
        must(isinstance(kinds, list) and kinds)
        library_kinds = {"lib", "rlib", "dylib", "staticlib", "cdylib", "proc-macro"}
        target_kind = "lib" if set(kinds) <= library_kinds else kinds[0] if len(kinds) == 1 else ""
        manifest_path = package["manifest_path"]
        must(manifest_path.startswith(source + "/") and os.path.realpath(manifest_path) == manifest_path)
        manifest = tomllib.loads(regular_bytes(manifest_path).decode("utf-8"))
        declarations = [manifest.get("lib", {})] if target_kind == "lib" else [item for item in manifest.get(target_kind, []) if item.get("name") == target["name"] or item.get("path") and os.path.abspath(join(dir_name(manifest_path), item["path"])) == target_source]
        must(len(declarations) <= 1)
        harness = not declarations or declarations[0].get("harness", True) is True
        if target_kind in ("lib", "bin", "test") and target.get("test") is True and harness:
            selected = ["/toolchain/bin/just", "test", "--locked", "--package", package["name"]]
            selected.extend(["--lib"] if target_kind == "lib" else ["--" + target_kind, target["name"]])
            features = target.get("required-features", [])
            must(isinstance(features, list) and all(isinstance(feature, str) and feature and "," not in feature for feature in features))
            if features:
                selected.extend(("--features", ",".join(sorted(set(features)))))
            selected.extend(("--no-tests=fail", "--", "--exact", *test_names))
    transport_write_result(output, {"Arguments": selected, "TargetRoots": roots})

def rust_suite_preparation(plan, terminal, diagnostics, bindings=(), packages=(), tests=()):
    return {
        "CompileOnlyPackages": plan["CompileOnlyPackages"],
        "Diagnostics": list(diagnostics),
        "HelperBinaries": plan["HelperBinaries"],
        "BinaryBindings": list(bindings),
        "SelectedPackages": list(packages),
        "SelectedTests": list(tests),
        "Terminal": terminal,
    }

def rust_suite_discovery(data, omitted):
    value = json.loads(data)
    must(isinstance(value, dict) and isinstance(value.get("rust-suites"), dict))
    packages, tests = set(), []
    for suite in value["rust-suites"].values():
        must(isinstance(suite, dict) and re.fullmatch(r"[A-Za-z0-9_-]+", suite.get("package-name", "")) and isinstance(suite.get("binary-id"), str) and suite["binary-id"] and not any(character in suite["binary-id"] for character in "\x00\r\n") and isinstance(suite.get("binary-name"), str) and suite["binary-name"] and not any(character in suite["binary-name"] for character in "\x00\r\n") and isinstance(suite.get("testcases"), dict))
        package, binary_id, binary_name = suite["package-name"], suite["binary-id"], suite["binary-name"]
        for test, testcase in suite["testcases"].items():
            must(isinstance(test, str) and test and "\x00" not in test and "\n" not in test and "\r" not in test and isinstance(testcase, dict))
            if package + " " + test in omitted:
                continue
            match = testcase.get("filter-match")
            ignored = testcase.get("ignored")
            must(isinstance(ignored, bool))
            if not ignored and (not isinstance(match, dict) or match.get("status") != "matches"):
                continue
            packages.add(package)
            tests.append({
                "BinaryID": binary_id,
                "BinaryName": binary_name,
                "ExpectationIdentity": package + " " + test,
                "Identity": binary_id + " " + test,
                "Ignored": ignored,
                "Package": package,
                "ResultName": package + "::" + binary_name + "$" + test,
                "TestName": test,
            })
    tests.sort(key=itemgetter("Identity"))
    must(len({test["Identity"] for test in tests}) == len(tests) and len({test["ResultName"] for test in tests}) == len(tests))
    must(packages and tests or omitted and not tests)
    return sorted(packages), tests

def rust_suite_execute(arguments):
    must(len(arguments) == 3)
    lower, overlay, plan_data = os.path.abspath(arguments[0]), os.path.abspath(arguments[1]), arguments[2]
    plan = json.loads(plan_data, object_pairs_hook=unique_json_object)
    tag, scope = plan["Tag"], plan["Scope"]
    must(kind(lower, DIRECTORY) and kind(overlay, DIRECTORY) and os.path.realpath(lower) == lower and os.path.realpath(overlay) == overlay and re.fullmatch(r"[A-Za-z0-9._-]+", tag) and scope in ("baseline", "full"))
    must(plan.get("SchemaVersion") == 3 and plan.get("RecordType") == "RustSuitePlanRecord")
    compile_only = rust_suite_package_list(plan.get("CompileOnlyPackages"))
    helpers = rust_suite_package_list(plan.get("HelperBinaries"))
    excluded = rust_suite_package_list(plan.get("ExcludedPackages"))
    build_jobs = str(plan["NextestConcurrency"])
    test_timeout = plan["TestTimeoutMilliseconds"]
    must(type(test_timeout) is int and 0 < test_timeout <= 120000 and not set(compile_only).intersection(helpers) and set(compile_only + helpers).issubset(excluded))
    selected = rust_suite_selection(excluded)
    def exact(value):
        return "".join(f"\\u{{{ord(character):x}}}" if character.isspace() or character in "\\()," else character for character in value)

    omitted = set(plan["ExpectedFailures"] + plan["ExpectedTimeouts"]) if scope != "baseline" else set()
    if omitted:
        predicates = []
        for identity in sorted(omitted):
            package, test = identity.split(" ", 1)
            predicates.append(f"(package(={package}) & test(={exact(test)}))")
        selected += ("--filterset", "not (" + " | ".join(predicates) + ")")
    preparation_path, nextest_path = "/result/rust-suite-preparation.json", "/result/nextest.jsonl"
    events = [json.loads(line) for line in regular_bytes(nextest_path).splitlines() if line.strip()]
    round_state = [event for event in events if event.get("type") == "codex-build-round"][-1]
    round_budget = int(os.environ["CODEX_SUITE_ROUND_BUDGET_MS"])
    if round_state["event"] == "completed":
        if round_state["exit"]:
            raise SystemExit(round_state["exit"])
        return
    if not exists(preparation_path):
        write(preparation_path, canonical(rust_suite_preparation(plan, "Fail", [{"Field": "preparation", "Expected": "complete", "Actual": "interrupted"}])))
    for name in ("rust-suite-binaries.jsonl", "rust-suite-discovery.json", "rust-suite-nextest.toml", "nextest-user.toml"):
        remove(join("/result", name))

    def fail(field, expected, actual, status=65):
        replace_regular(preparation_path, canonical(rust_suite_preparation(plan, "Fail", [{"Field": field, "Expected": expected, "Actual": str(actual)[:256]}])))
        raise SystemExit(status if status else 65)

    def phase(name):
        with open("/result/.phase", "wb", buffering=0) as stream:
            stream.write(name.encode("ascii") + b"\n")

    phase("Compilation")
    if compile_only:
        command = ["/toolchain/rust/bin/cargo", "check", "--locked", "--jobs", build_jobs, "--message-format=json"]
        for package in compile_only:
            command.extend(("--package", package))
        result = subprocess.run(command, cwd=join(overlay, "codex-rs"), stdout=sys.stderr.buffer, stderr=sys.stderr.buffer, check=False)
        if result.returncode:
            fail("compile-only-packages", "successful Cargo check", result.returncode, result.returncode)
    cargo = "/toolchain/rust/bin/cargo"
    metadata_result = subprocess.run((cargo, "metadata", "--locked", "--no-deps", "--format-version", "1"), cwd=join(overlay, "codex-rs"), stdout=subprocess.PIPE, stderr=sys.stderr.buffer, check=False)
    if metadata_result.returncode:
        fail("cargo-metadata", "successful Cargo workspace metadata", metadata_result.returncode, metadata_result.returncode)
    try:
        metadata = json.loads(metadata_result.stdout)
        workspace = set(metadata["workspace_members"])
        packages = {package["id"]: package for package in metadata["packages"] if package["id"] in workspace}
        expected = {(identifier, target["name"]) for identifier, package in packages.items() for target in package["targets"] if "bin" in target["kind"] and (package["name"] not in excluded or package["name"] in helpers)}
        must(set(helpers) <= {packages[identifier]["name"] for identifier, _ in expected})
    except (KeyError, TypeError, RuntimeError, json.JSONDecodeError) as error:
        fail("binary-binding", "complete admitted Cargo bin target inventory", error)
    artifact_data = b""
    if expected:
        command = [cargo, "build", "--workspace", "--bins", "--locked", "--jobs", build_jobs, "--message-format=json"]
        for package in excluded:
            if package not in helpers:
                command.extend(("--exclude", package))
        with open("/result/rust-suite-binaries.jsonl", "xb") as output:
            result = subprocess.run(command, cwd=join(overlay, "codex-rs"), stdout=output, stderr=sys.stderr.buffer, check=False)
        if result.returncode:
            fail("cargo-bin-build", "successful admitted Cargo bin targets", result.returncode, result.returncode)
        artifact_data = regular_bytes("/result/rust-suite-binaries.jsonl")
    artifacts = set()
    for line in artifact_data.splitlines():
        try:
            artifact = json.loads(line)
        except json.JSONDecodeError as error:
            fail("binary-artifact", "Cargo JSON", error)
        target, executable = artifact.get("target", {}), artifact.get("executable")
        if artifact.get("reason") != "compiler-artifact" or not executable or "bin" not in target.get("kind", []):
            continue
        identifier, name = artifact.get("package_id"), target.get("name", "")
        path, normalized = "/cache/target/debug/" + name, name.replace("-", "_")
        key = (identifier, name)
        if key not in expected or key in artifacts or executable != path or not kind(executable, REGULAR):
            fail("binary-binding", "unique exact CargoTarget artifact", executable)
        artifacts.add(key)
    if artifacts != expected:
        fail("binary-binding", "every admitted Cargo bin target artifact", sorted(expected - artifacts))
    bindings, environment_names = [], set()
    for identifier, name in sorted(artifacts):
        package, path = packages[identifier]["name"], "/cache/target/debug/" + name
        normalized = name.replace("-", "_")
        environments = sorted(set(("CARGO_BIN_EXE_" + name, "CARGO_BIN_EXE_" + normalized, "NEXTEST_BIN_EXE_" + normalized)))
        if environment_names.intersection(environments):
            fail("binary-binding", "unique Cargo and nextest environment names", name)
        environment_names.update(environments)
        bindings.append({"Environments": environments, "Package": package, "Path": path})
    bindings.sort(key=itemgetter("Package", "Path"))
    if {package["name"] for package in packages.values()} <= set(excluded):
        must(compile_only or helpers)
        replace_regular(preparation_path, canonical(rust_suite_preparation(plan, "Complete", [], bindings)))
        return
    nextest_configuration_path = "/result/rust-suite-nextest.toml"
    repository_nextest_configuration = regular_bytes(join(overlay, "codex-rs", ".config", "nextest.toml"))
    remaining_filter = ""

    def nextest_configuration(timeout):
        overlay_configuration = f'''[profile.codex-build]
inherits = "local"
test-threads = {build_jobs}
{remaining_filter}

[[profile.codex-build.overrides]]
filter = "all()"
slow-timeout = {{ period = "{timeout}ms", terminate-after = 1, grace-period = "0s" }}
'''.encode()
        return repository_nextest_configuration + b"\n" + overlay_configuration

    write(nextest_configuration_path, nextest_configuration(test_timeout))
    list_command = ("/toolchain/rust/bin/cargo", "nextest", "list", *selected, "--locked", "--build-jobs", build_jobs, "--profile", "codex-build", "--config-file", nextest_configuration_path, "--message-format", "json")
    with open("/result/rust-suite-discovery.json", "xb") as output:
        listing = subprocess.run(list_command, cwd=join(overlay, "codex-rs"), stdout=output, stderr=sys.stderr.buffer, check=False)
    if listing.returncode:
        fail("current-test-discovery", "successful nextest list", listing.returncode, listing.returncode)
    try:
        selected_packages, selected_tests = rust_suite_discovery(regular_bytes("/result/rust-suite-discovery.json"), omitted)
    except (RuntimeError, ValueError, TypeError, json.JSONDecodeError) as error:
        fail("current-test-discovery", "nonempty canonical identities", error)
    if any(event.get("type") == "test" for event in events):
        names = {test["ResultName"] for test in selected_tests}
        passed = set()
        for event in events:
            if event.get("type") == "test" and event.get("event") == "ok":
                name = event["name"]
                if name not in names and name.endswith(("#2", "#3")):
                    name = name[:-2]
                must(name in names)
                passed.add(name)
        remaining = [f"(binary_id(={exact(test['BinaryID'])}) & test(={exact(test['TestName'])}))" for test in selected_tests if not test["Ignored"] and test["ResultName"] not in passed]
        remaining_filter = "default-filter = " + json.dumps(" | ".join(remaining) or "none()", ensure_ascii=False)
        replace_regular(nextest_configuration_path, nextest_configuration(test_timeout))
    replace_regular(preparation_path, canonical(rust_suite_preparation(plan, "Complete", [], bindings, selected_packages, selected_tests)))
    environment = dict(os.environ)
    environment.update({name: binding["Path"] for binding in bindings for name in binding["Environments"]})
    environment["NEXTEST_STATE_DIR"] = "/result/nextest-records"
    nextest_user_configuration = "/result/nextest-user.toml"
    write(nextest_user_configuration, b"[experimental]\nrecord = true\n\n[record]\nenabled = true\n")
    command = (
        "/toolchain/rust/bin/cargo", "nextest", "run", *selected, "--locked", "--build-jobs", build_jobs,
        "--retries", "2", "--no-fail-fast", "--flaky-result", "pass", "--no-tests=pass", "--profile", "codex-build", "--config-file", nextest_configuration_path,
        "--user-config-file", nextest_user_configuration,
        "--message-format", "libtest-json-plus", "--message-format-version", "0.1",
    )
    phase("TestExecution")
    step = max(1, (sum(not test["Ignored"] for test in selected_tests) + 19) // 20)
    with open(nextest_path, "ab", buffering=0) as stream:
        def boundary(event, ordinal, deadline, status=0):
            stream.write(json.dumps({"type": "codex-build-round", "event": event, "round": ordinal, "deadline_ms": deadline, "exit": status}).encode() + b"\n")
            os.fsync(stream.fileno())

        for round_index in range(round_state["round"] - 1, 3):
            rerun = ("--rerun", "latest") if round_index >= round_state["round"] else ()
            starts = 0
            with subprocess.Popen(
                (*command, *rerun), cwd=join(overlay, "codex-rs"), env=environment,
                stdout=subprocess.PIPE, stderr=sys.stderr.buffer,
            ) as process:
                try:
                    for line in process.stdout:
                        stream.write(line)
                        try:
                            event = json.loads(line)
                        except ValueError:
                            continue
                        if isinstance(event, dict) and event.get("type") == "test" and event.get("event") == "started":
                            starts += 1
                            if starts % step == 0 and starts <= 20 * step:
                                phase(f"TestExecution started={starts} round={round_index + 1}/3")
                    status = process.wait()
                except BaseException:
                    process.kill()
                    raise
            os.fsync(stream.fileno())
            boundary("completed", round_index + 1, round_state["deadline_ms"], status)
            next_timeout = test_timeout if status == 100 and round_index < 2 else 0
            next_deadline = 0
            if next_timeout:
                next_deadline = int(time.time() * 1000) + round_budget // (2 ** (round_index + 1)) if round_budget else round_state["deadline_ms"]
                boundary("started", round_index + 2, next_deadline)
                round_state["deadline_ms"] = next_deadline
            phase(f"TestExecution completed-round={round_index + 1}/3 exit={status} deadline={next_deadline}")
            if not next_timeout:
                break
    if status:
        raise SystemExit(status)

def snapshot_manifest(lower, overlay, targets):
    inline = inline_records(lower, overlay, {target["Path"] for target in targets if target["Kind"] == "InlineRust"})
    updates = []
    for target in targets:
        path = target["Path"]
        spans = []
        if target["Kind"] == "InlineRust":
            records = [record for record in inline if record["Path"] == path]
            spans = [{"Start": record["Start"], "End": record["End"], "Line": record["Line"], "Postimage": record["Postimage"]} for record in records]
            generated = inline_postimage(target["Current"].encode(), [(span["Start"], span["End"], span["Postimage"].encode()) for span in spans]).decode()
            pending = join(overlay, dir_name(path), "." + base_name(path) + ".pending-snap")
        else:
            pending = join(overlay, path + ".new")
            if exists(pending):
                must(not exists(join(lower, path + ".new")) and kind(pending, REGULAR))
            generated = regular_bytes(pending if exists(pending) else join(overlay, path)).decode()
        if generated == target["Current"]:
            remove(pending)
            continue
        updates.append({"Target": target, "Generated": generated, "GeneratedInlineSpans": spans})
    return canonical({"Updates": updates})

def transport_repository(path, clean_required=None):
    path = os.path.abspath(path)
    must(kind(path, DIRECTORY) and os.path.realpath(path) == path)
    repository = Repo(path)
    repository.policy()
    must(repository.text("rev-parse", "--show-toplevel") == path)
    if clean_required is not None:
        must(repository.clean() == clean_required)
    return repository

def transport_base(repository, tag):
    must(re.fullmatch(r"rust-v[0-9A-Za-z._-]+", tag) and repository.has_ref("refs/tags/" + tag))
    base = f"refs/tags/{tag}^{{}}^"
    must(repository.ok("merge-base", "--is-ancestor", base, "HEAD"))
    return base

def transport_groups(rows):
    groups = []
    for _, members in groupby(rows, itemgetter(2, 3, 4)):
        group = list(members)
        groups.append((group[0][2], group[0][3], group[0][4], group))
    return groups

def transport_complete(group, partial=False):
    kind, members = group[1], group[3]
    if kind == "Transform":
        return len(members) == 1
    patches = [row[5] for row in members]
    minimum = 1 if kind == "FlatPatch" or partial else 2
    return kind in ("FlatPatch", "Unit") and all(row[6] == "GitDiff" and row[8] == "source" for row in members) and len(set(patches)) == len(patches) and len({row[7] for row in members}) == 1 and len(members) >= minimum and (kind == "Unit" or len(members) == 1 and patches[0] == group[2])

def transport_group_index(groups, kind, identity):
    lane, separator, name = identity.partition("/")
    must(kind in ("FlatPatch", "Unit", "Transform") and separator and lane in LANES and name)
    matches = [index for index, group in enumerate(groups) if group[:3] == (lane, kind, name)]
    must(len(matches) == 1)
    return matches[0]

def transport_selector(value):
    kind, separator, identity = value.partition(":")
    lane, slash, name = identity.partition("/")
    must(separator and slash and kind in ("FlatPatch", "Unit", "Transform") and lane in LANES and name)
    return kind, identity

def transport_group_selector(group):
    return f"{group[1]}:{group[0]}/{group[2]}"

def transport_result_identity(kind, lane, group, patch=None):
    result = {"Kind": kind, "Lane": lane, kind: group}
    if kind == "Unit" and patch:
        result["ChildPatch"] = patch
    if kind == "Transform" and patch:
        result["TransformPatch"] = patch
    return result

def transport_rows(repository, tag, complete=False, partial=False):
    base = transport_base(repository, tag)
    rows = history_rows(repository, base)
    if rows:
        validate_history(rows, complete, partial)
    return base, rows

def transport_export_rows(root, predecessor, current_tag=""):
    root = os.path.abspath(root)
    private = transport_repository(dir_name(root))
    must(base_name(root) == predecessor and private.branch() == "main" and not private.sequencer())
    must(re.fullmatch(r"rust-v[0-9A-Za-z._-]+", predecessor) and not predecessor.endswith(("-prev", ".pending")))
    if current_tag:
        must(re.fullmatch(r"rust-v[0-9A-Za-z._-]+", current_tag))
    rows, counts, active, patch_count = [], {}, None, 0
    notes = release_notes(root)
    pattern = r"([0-9]+)-([a-z-]+)/([0-9]{2})-([a-z0-9-]+)(?:/([0-9]{3})-([a-z0-9-]+))?\.diff"
    for relative in root_inventory(root, private.path):
        if relative in notes:
            continue
        match = re.fullmatch(pattern, relative)
        must(match)
        kind = "Unit" if match[5] else "FlatPatch"
        must(match[2] in LANES and int(match[1]) == LANES.index(match[2]) and formal(match[4], match[2], kind))
        lane, structure = match[2], (match[2], int(match[3]), match[4])
        if structure != active:
            counts[lane] = counts.get(lane, 0) + 1
            must(structure[1] == counts[lane])
            active, patch_count = structure, 0
        patch_count += 1
        patch = match[6] or structure[2]
        must(not match[5] or int(match[5]) == patch_count and formal(patch) and patch != structure[2])
        data = regular_bytes(join(root, relative))
        message, separator, _ = data.partition(b"\n\ndiff --git ")
        roll_require(separator, relative, "GitDiff", "canonical commit metadata followed by a patch body", "missing metadata or patch body")
        try:
            metadata = metadata_fields(message, relative)
        except CommitMetadataError as error:
            raise RollInputError(relative, error.field, "canonical exported commit metadata", error.problem) from error
        roll_require(metadata[:4] == (lane, kind, structure[2], patch) and metadata[4] == "GitDiff", relative, "PatchIdentity", (lane, kind, structure[2], patch, "GitDiff"), metadata[:5])
        lines = data.splitlines()
        must(data and not any(line.startswith(b"index ") or invalid_diff_line(line) for line in lines))
        paths = [line.split("\t", 2)[2] for line in private.run("apply", "--numstat", data=data).stdout.decode().splitlines()]
        validate_protocol_segment(paths, lane, metadata[5], relative)
        classification = source_change_class(paths, f"{lane}/{structure[2]}")
        rows.append((join(root, relative), "", *metadata, classification))
    groups = transport_groups(rows)
    must(groups)
    must(all(path == "notes.md" or dir_name(path) in {lane_path(group[0]) for group in groups} for path in notes))
    return validate_history([("", "", *NORMALIZATION_TRANSFORM, "normalization-transform"), *rows])

def transport_source_rows(repository):
    rows, depth = [], 0
    while True:
        commit = "HEAD" if depth == 0 else f"HEAD~{depth}"
        try:
            rows.append((commit, f"{commit}^", *fields(repository, commit), None))
        except CommitMetadataError:
            break
        depth += 1
    must(rows)
    rows.reverse()
    for index, row in enumerate(rows):
        rows[index] = row[:8] + (change_class(repository, row[2], row[6], row[7], row[1], row[0], attributes=row[1]),)
    return validate_history(rows, partial=True)

def transport_write_result(path, value):
    write(path, canonical(value))

def transport_reset(repository, applied=0):
    repository.run("reset", "--quiet", "--hard", "HEAD" if applied == 0 else f"HEAD~{applied}")
    repository.run("clean", "-fd")
    must(repository.clean())

def transport_dirty_snapshot(repository, unit):
    intent_to_add = tuple(
        os.fsdecode(path)
        for path in repository.bytes("diff", "--ita-invisible-in-index", "--name-only", "-z", "--diff-filter=A").split(b"\0")
        if path
    )
    nodes = []
    for name in changed_paths(repository):
        path = join(repository.path, name)
        mode = os.lstat(path).st_mode if os.path.lexists(path) else 0
        file_type = stat.S_IFMT(mode)
        roll_require(file_type in (0, REGULAR, DIRECTORY, SYMLINK), unit, "Path", "file/directory/symlink/absent", name)
        data = regular_bytes(path) if file_type == REGULAR else os.fsencode(os.readlink(path)) if file_type == SYMLINK else b""
        nodes.append((name, file_type, stat.S_IMODE(mode), data))
    roll_require(nodes, unit, "Paths", "nonempty", "empty")
    staged = repository.bytes("diff", "--ita-invisible-in-index", "--cached", "--binary")
    return staged, intent_to_add, tuple(nodes)

def transport_correction(repository, index):
    must(not exists(index) and not exists(index + ".lock"))
    try:
        repository.run("read-tree", "HEAD", index=index)
        repository.run("add", "-A", index=index)
        correction = repository.bytes("diff", *DIFF_ARGUMENTS, "--cached", "HEAD", attributes="HEAD", index=index)
        correction = b"".join(line for line in correction.splitlines(keepends=True) if not line.startswith(b"index "))
        must(correction and not any(invalid_diff_line(line) for line in correction.splitlines()))
        return correction
    finally:
        remove(index)
        remove(index + ".lock")

def transport_restore_dirty(repository, revision, snapshot, unit):
    repository.run("reset", "--quiet", "--hard", revision)
    if snapshot[0]:
        repository.run("apply", "--cached", "--binary", data=snapshot[0])
    for name, file_type, mode, data in snapshot[2]:
        path = join(repository.path, name)
        remove(path)
        if not file_type:
            continue
        os.makedirs(dir_name(path), exist_ok=True)
        if file_type == REGULAR:
            write(path, data, mode)
        elif file_type == DIRECTORY:
            os.mkdir(path, 0o700)
            sync_parent(path)
        else:
            os.symlink(os.fsdecode(data), path)
            sync_parent(path)
    for name, file_type, mode, _ in reversed(snapshot[2]):
        if file_type == DIRECTORY:
            os.chmod(join(repository.path, name), mode)
    if snapshot[1]:
        repository.run("add", "-N", "--", *snapshot[1])
    roll_require(transport_dirty_snapshot(repository, unit) == snapshot, unit, "Rollback", "exact preimage", "different")

def transport_interrupt(*_):
    signal.signal(signal.SIGTERM, signal.SIG_IGN)
    os.kill(0, signal.SIGTERM)
    raise InterruptedError

def transport_orig_head(repository):
    path = repository.text("rev-parse", "--path-format=absolute", "--git-path", "ORIG_HEAD")
    return path, (stat.S_IMODE(os.lstat(path).st_mode), regular_bytes(path)) if exists(path) else None

def transport_restore_orig_head(path, original):
    pending = path + ".codex-build-pending"
    must(not exists(pending))
    if original:
        write(pending, original[1], original[0])
        try:
            os.replace(pending, path)
        except BaseException:
            remove(pending)
            raise
    else:
        remove(path)

def transport_chrome(target, asset_root, check=False):
    asset, working, writable = TRANSFORM_ASSETS[NORMALIZATION_TRANSFORM]
    script = join(asset_root, asset)
    must(kind(script, REGULAR) and os.path.realpath(asset_root) == asset_root)
    process = subprocess.run(["/usr/bin/python3", "-s", script, *(["--check"] if check else [])], cwd=join(target.path, working), env=ENV, stdout=subprocess.PIPE, stderr=subprocess.PIPE, check=False)
    return (process.stderr.decode(errors="replace").strip() or "terminal-chrome transform failed")[:4096] if process.returncode else ""

def transport_format_generation(target, formatter, asset_root, iteration):
    endpoint, working, scratch, runtime, tag, deadline = formatter
    attempt = f"chrome-{base_name(asset_root)}-{iteration}"
    command = [endpoint, "repository-format", "--skill-root", dir_name(endpoint), "--project-working-root", working, "--project-scratch-root", scratch, "--runtime-tmp-root", runtime, "--source", target.path, "--release", tag, "--attempt-id", attempt, "--deadline", deadline]
    blocked = signal.pthread_sigmask(signal.SIG_BLOCK, {signal.SIGTERM})
    try:
        process = subprocess.run(command, env=ENV, capture_output=True)
    finally:
        signal.pthread_sigmask(signal.SIG_SETMASK, blocked)
    try:
        result = json.loads(process.stdout, object_pairs_hook=unique_json_object)
        must(isinstance(result, dict) and result.get("RecordType") == "RepositoryFormatResult" and result.get("Source") == target.path and result.get("Release") == tag and result.get("AttemptID") == attempt)
    except (ValueError, RuntimeError):
        return "RepositoryFormatHelper: invalid formatter terminal; " + process.stderr.decode(errors="replace")[:4096]
    if process.returncode or result.get("Terminal") != "Pass":
        domain = "Product" if result.get("FailureDomain") == "Product" else "Helper"
        diagnostic = "\n".join((result.get("Stderr", ""), result.get("Stdout", ""), process.stderr.decode(errors="replace"))).strip()
        return f"RepositoryFormat{domain}: {diagnostic[:4096] or result.get('Terminal')}"
    must(result.get("FailureDomain") == "None")
    return ""

def transport_transform(target, metadata, asset_root, formatter):
    writable = TRANSFORM_ASSETS[metadata][2]
    parent = target.text("rev-parse", "HEAD")
    previous_handler = signal.signal(signal.SIGTERM, transport_interrupt)
    first = None
    try:
        for iteration in range(2):
            failure = transport_chrome(target, asset_root)
            if not failure:
                failure = transport_format_generation(target, formatter, asset_root, iteration)
            if not failure:
                failure = transport_chrome(target, asset_root, True)
            if failure:
                transport_reset(target)
                return failure
            paths = changed_paths(target)
            must(paths and all(any(path == root or path.startswith(root + "/") for root in writable) for path in paths))
            must(target.text("rev-parse", "HEAD") == parent)
            target.run("add", "-A")
            change_class(target, metadata[0], metadata[4], metadata[5])
            current = {path: regular_bytes(join(target.path, path)) if exists(join(target.path, path)) else None for path in paths}
            if iteration == 0:
                first = current
                transport_reset(target)
            elif current != first:
                transport_reset(target)
                return "terminal-chrome formatted postimages differ between identical generations"
        metadata_commit(target, *metadata)
        return ""
    except BaseException:
        target.run("reset", "--quiet", "--hard", parent)
        transport_reset(target)
        raise
    finally:
        signal.signal(signal.SIGTERM, previous_handler)

def transport_schema_bundle(bundle):
    value = json.loads(regular_bytes(bundle), object_pairs_hook=unique_json_object)
    must(isinstance(value, dict) and set(value) == {"Files"} and isinstance(value["Files"], list))
    files = []
    for record in value["Files"]:
        must(isinstance(record, dict) and set(record) == {"ContentBase64", "Path"})
        path, content = record["Path"], record["ContentBase64"]
        must(isinstance(path, str) and isinstance(content, str) and git_path(path) == path and generated_schema_path(f"{SCHEMA_ROOT}/{path}"))
        files.append((path, base64.b64decode(content, validate=True)))
    must({path.split("/", 1)[0] for path, _ in files} == set(SCHEMA_OUTPUTS))
    return files

def transport_schema_postimages(target, metadata, files, source_commit):
    actual = target.text("rev-parse", "HEAD")
    roll_require(target.clean() and actual == source_commit, target.path, "SchemaSourceCommit", source_commit, actual)
    try:
        for name in SCHEMA_OUTPUTS:
            remove(join(target.path, SCHEMA_ROOT, name))
        for name, content in files:
            path = join(target.path, SCHEMA_ROOT, name)
            os.makedirs(dir_name(path), exist_ok=True)
            write(path, content, 0o644)
        paths = changed_paths(target)
        must(all(generated_schema_path(path) for path in paths))
        if paths:
            target.run("add", "-A")
            change_class(target, metadata[0], metadata[4], metadata[5])
        metadata_commit(target, *metadata, allow_empty=True)
    except BaseException:
        transport_reset(target)
        raise

def transport_snapshot_bundle(target, bundle, commit="HEAD"):
    must(isinstance(bundle, dict) and set(bundle) == {"SourceCommit", "Updates"} and bundle["SourceCommit"] == target.text("rev-parse", commit) and isinstance(bundle["Updates"], list))
    updates, previous = [], ""
    for update in bundle["Updates"]:
        must(isinstance(update, dict) and set(update) == {"Generated", "GeneratedInlineSpans", "Target"})
        descriptor = update["Target"]
        must(isinstance(descriptor, dict) and set(descriptor) == {"Current", "ExistingInlineSpans", "Kind", "Path"})
        relative, target_kind = descriptor["Path"], descriptor["Kind"]
        must(isinstance(relative, str) and previous < relative and isinstance(descriptor["Current"], str) and isinstance(update["Generated"], str) and isinstance(descriptor["ExistingInlineSpans"], list) and isinstance(update["GeneratedInlineSpans"], list))
        _, inline = snapshot_source_path(relative)
        absolute = join(target.path, relative)
        if commit == "HEAD":
            current = regular_bytes(absolute).decode() if exists(absolute) else ""
        else:
            current = target.bytes("show", commit + ":" + relative).decode() if target.ok("cat-file", "-e", commit + ":" + relative) else ""
        must(current == descriptor["Current"] and update["Generated"] != current and ((target_kind == "InlineRust") == inline))
        if inline:
            spans = {(span["Start"], span["End"], span["Line"]): span for span in descriptor["ExistingInlineSpans"] if isinstance(span, dict) and set(span) == {"Start", "End", "Line", "Preimage"}}
            must(len(spans) == len(descriptor["ExistingInlineSpans"]) and spans and update["GeneratedInlineSpans"])
            rebuilt, last, replacements = current.encode(), 0, []
            for span in update["GeneratedInlineSpans"]:
                must(isinstance(span, dict) and set(span) == {"Start", "End", "Line", "Postimage"})
                key = (span["Start"], span["End"], span["Line"])
                original = spans.get(key)
                must(original is not None and isinstance(span["Postimage"], str) and isinstance(original["Preimage"], str) and rebuilt[key[0]:key[1]].decode() == original["Preimage"] and key[0] >= last)
                replacements.append((key[0], key[1], span["Postimage"].encode()))
                last = key[1]
            for start, end, postimage in sorted(replacements, reverse=True):
                rebuilt = rebuilt[:start] + postimage + rebuilt[end:]
            must(rebuilt.decode() == update["Generated"])
        else:
            must(target_kind == "ExternalSnapshot" and not descriptor["ExistingInlineSpans"] and not update["GeneratedInlineSpans"] and update["Generated"])
        updates.append((relative, update["Generated"].encode(), exists(absolute)))
        previous = relative
    return updates

def transport_snapshot_postimages(target, metadata, updates):
    head, created, directories = target.text("rev-parse", "HEAD"), [], []
    try:
        for relative, generated, present in updates:
            path = join(target.path, relative)
            if present:
                replace_regular(path, generated, stat.S_IMODE(os.lstat(path).st_mode))
            else:
                parent, missing = dir_name(path), []
                while parent != target.path and not exists(parent):
                    missing.append(parent)
                    parent = dir_name(parent)
                os.makedirs(dir_name(path), mode=0o755, exist_ok=True)
                directories.extend(reversed(missing))
                write(path, generated, 0o644)
                created.append(path)
        target.run("add", "-A")
        must(set(changed_paths(target)) == {relative for relative, _, _ in updates})
        change_class(target, metadata[0], metadata[4], metadata[5])
        metadata_commit(target, *metadata, allow_empty=True)
    except BaseException:
        target.run("reset", "--quiet", "--hard", head)
        for path in created:
            remove(path)
        for path in reversed(directories):
            os.rmdir(path)
        raise

def source_prefix_preimage(root):
    return tuple((path, stat.S_IMODE(os.lstat(join(root, path)).st_mode), regular_bytes(join(root, path))) for path in root_inventory(root, dir_name(root)))

def transport_apply(arguments):
    must(len(arguments) == 16)
    source_path, target_path, tag, start_selector, end_value, rustfmt, asset_root, predecessor, child_patch_sequence, source_custody, endpoint, working, scratch, runtime, deadline, output = arguments
    target = transport_repository(target_path, True)
    source = None if predecessor else transport_repository(source_path, True)
    must((source is None or source.path != target.path) and kind(rustfmt, REGULAR) and source_custody == ("exported" if predecessor else "retained"))
    must(kind(endpoint, REGULAR) and os.path.realpath(endpoint) == endpoint and all(os.path.isabs(path) for path in (working, scratch, runtime)))
    formatter = (endpoint, working, scratch, runtime, tag, deadline)
    source_before = source_prefix_preimage(source_path) if predecessor else None
    source_rows = transport_export_rows(source_path, predecessor, tag) if predecessor else transport_source_rows(source)
    source_bytes = {join(source_path, path): data for path, mode, data in source_before} if predecessor else None
    if predecessor:
        must(source_prefix_preimage(source_path) == source_before and not end_value)
    _, target_rows = transport_rows(target, tag, partial=True)
    groups = transport_groups(source_rows)
    start_kind, start_identity = transport_selector(start_selector)
    start = transport_group_index(groups, start_kind, start_identity)
    end_kind, end_identity = transport_selector(end_value) if end_value else (start_kind, start_identity)
    end = transport_group_index(groups, end_kind, end_identity)
    must(end >= start)
    selected = [group for group in groups[start:end + 1] if tuple(group[3][0][2:8]) not in (*SCHEMA_TRANSFORMS.values(), *SNAPSHOT_TRANSFORMS.values())]
    must(all(transport_complete(group) for group in selected))
    prefix = transport_groups(target_rows)
    declared = child_patch_sequence.split(", ") if child_patch_sequence else []
    must((start_kind == "Unit" and len(declared) >= 2 or start_kind != "Unit" and not declared) and len(declared) == len(set(declared)) and all(formal(patch) for patch in declared))
    def reject_sequence(expected):
        transport_write_result(output, {**transport_result_identity("Unit", selected[0][0], selected[0][2]), "Result": "Input", "Output": str(RollInputError(start_selector, "ChildPatchSequence", expected, declared))})
    if not selected:
        must(start_kind == end_kind == "Transform")
        transport_write_result(output, {"AppliedRows": [], "FirstLaneChild": start_selector, "LastLaneChild": end_value or start_selector, "Result": "Applied"})
        return
    if prefix and prefix[-1][:3] == selected[0][:3]:
        must(start_kind == "Unit")
        current_patches = [row[5] for row in prefix[-1][3]]
        source_members = selected[0][3]
        source_patches = [row[5] for row in source_members]
        suffixes = [index for index in range(1, len(source_members)) if current_patches + source_patches[index:] == declared]
        if len(suffixes) != 1:
            return reject_sequence({"CurrentPrefix": current_patches, "SourceUnit": source_patches})
        selected[0] = (*selected[0][:3], source_members[suffixes[0]:])
        prefix = prefix[:-1]
    elif start_kind == "Unit":
        source_patches = [row[5] for row in selected[0][3]]
        if source_patches != declared:
            return reject_sequence(source_patches)
    must(not {group[:3] for group in prefix} & {group[:3] for group in selected})
    prospective = target_rows + [row for _, _, _, members in selected for row in members]
    validate_history(prospective, partial=True)
    applied = []
    for lane, form, group, members in selected:
        if predecessor:
            must(source_prefix_preimage(source_path) == source_before)
        original_tip = target.text("rev-parse", "HEAD")
        failure = transport_apply_unit(source, target, members, rustfmt, asset_root, formatter, source_bytes)
        if predecessor:
            try:
                must(source_prefix_preimage(source_path) == source_before)
            except BaseException:
                target.run("reset", "--quiet", "--hard", original_tip)
                raise
        if failure:
            transport_write_result(output, {"AppliedRows": applied, **transport_result_identity(form, lane, group), "Output": failure, "Result": "Conflict"})
            return
        record = transport_patch_record((lane, form, group, members))
        if form == "Unit" and (lane, form, group) == selected[0][:3]:
            record["ChildPatches"] = declared
        applied.append(record)
        transport_rows(target, tag)
    transport_write_result(output, {"AppliedRows": applied, "FirstLaneChild": start_selector, "LastLaneChild": end_value or start_selector, "Result": "Applied"})

def transport_prepare(arguments):
    must(len(arguments) == 4)
    target = transport_repository(arguments[0], True)
    tag, expected, output = arguments[1:]
    _, rows = transport_rows(target, tag, partial=True)
    if expected:
        groups = transport_groups(rows)
        ordinary = next(group for group in reversed(groups) if group[1] != "Transform")
        actual = transport_group_selector(groups[-1])
        base = transport_group_selector(ordinary)
        must(expected == actual or actual == base and expected.startswith(f"Transform:{ordinary[0]}/"))
    head = target.text("rev-parse", "HEAD")
    source_rows = [row for row in rows if row[6] == "GitDiff" or row[2:8] == NORMALIZATION_TRANSFORM]
    effect = "Current"
    if len(source_rows) != len(rows):
        parent = target.text("rev-parse", source_rows[-1][0])
        ignored = [os.fsdecode(path) for path in target.bytes("ls-files", "--others", "--ignored", "--exclude-standard", "-z").split(b"\0") if path]
        paths = changed(target, parent, head)
        must(not any(path == retained or path.startswith(retained + "/") or retained.startswith(path + "/") for path in paths for retained in ignored))
        blocked = signal.pthread_sigmask(signal.SIG_BLOCK, {signal.SIGTERM})
        try:
            must(target.clean() and target.text("rev-parse", "HEAD") == head)
            target.run("reset", "--quiet", "--hard", parent)
            must(target.clean() and target.text("rev-parse", "HEAD") == parent)
            effect = "Removed"
        finally:
            signal.pthread_sigmask(signal.SIG_SETMASK, blocked)
    transport_write_result(output, {"Rows": transport_patch_records(transport_rows(target, tag, partial=True)[1]), "Output": effect, "Result": "Prepared"})

def transport_finalize(arguments):
    must(len(arguments) == 8)
    target = transport_repository(arguments[0], True)
    tag, lane, close_snapshots, expected, bundle, source_commit, output = arguments[1:]
    close_snapshots = close_snapshots == "true"
    must(lane in LANES)
    _, rows = transport_rows(target, tag, partial=True)
    groups = transport_groups(rows)
    must(transport_complete(groups[-1]))
    ordinary = [row for row in rows if row[6] == "GitDiff"]
    must(ordinary and ordinary[-1][2] == lane)
    if close_snapshots:
        base = next(group for group in reversed(groups) if group[1] != "Transform")
        must(expected in (transport_group_selector(base), transport_group_selector(groups[-1])))
    schema = SCHEMA_TRANSFORMS.get(lane)
    required = schema_required(rows)
    must(close_snapshots or required)
    schema_present = schema is not None and any(row[2:8] == schema for row in rows)
    snapshot = SNAPSHOT_TRANSFORMS[lane]
    snapshot_present = any(row[2:8] == snapshot for row in rows)
    effect = "Current"
    if required and not schema_present:
        if not bundle:
            must(not source_commit)
            transport_write_result(output, {"Rows": transport_patch_records(rows), **transport_result_identity("Transform", lane, schema[2], schema[3]), "Result": "ProjectionRequired"})
            return
        files = transport_schema_bundle(bundle)
        transport_schema_postimages(target, schema, files, source_commit)
        effect = "Committed"
        bundle = source_commit = ""
    if close_snapshots and not snapshot_present:
        if not bundle:
            rows = transport_rows(target, tag, partial=True)[1]
            transport_write_result(output, {"Rows": transport_patch_records(rows), "Output": effect, **transport_result_identity("Transform", lane, snapshot[2], snapshot[3]), "Result": "SnapshotProjectionRequired"})
            return
        transport_snapshot_postimages(target, snapshot, transport_snapshot_bundle(target, json.loads(regular_bytes(bundle), object_pairs_hook=unique_json_object)))
        effect = "Committed"
    else:
        must(not bundle and not source_commit and (close_snapshots or not snapshot_present))
    rows = transport_rows(target, tag, partial=True)[1]
    metadata = snapshot if close_snapshots else schema
    must(metadata is not None)
    transport_write_result(output, {"Rows": transport_patch_records(rows), "Output": effect, **transport_result_identity("Transform", lane, metadata[2], metadata[3]), "Result": "Finalized"})

def transport_config_schema(arguments):
    must(len(arguments) == 6)
    target_path, tag, selector, child_patch_sequence, bundle_path, output = arguments
    target = transport_repository(target_path, True)
    head = target.text("rev-parse", "HEAD")
    branch = target.text("symbolic-ref", "HEAD")
    _, rows = transport_rows(target, tag, partial=True)
    groups = transport_groups(rows)
    patch_kind, identity = transport_selector(selector)
    must(groups and transport_group_selector(groups[-1]) == selector and patch_kind in ("FlatPatch", "Unit"))
    members = groups[-1][3]
    declared = child_patch_sequence.split(", ") if child_patch_sequence else []
    must(all(row[6] == "GitDiff" for row in members) and (patch_kind == "FlatPatch" and not declared or patch_kind == "Unit" and len(declared) >= 2 and len(members) <= len(declared) and [row[5] for row in members] == declared[:len(members)]))
    relative = "codex-rs/core/config.schema.json"
    fixture = join(target.path, relative)
    must(os.path.realpath(fixture) == fixture)
    before = regular_bytes(fixture)
    if not bundle_path:
        lane, _, name = identity.partition("/")
        transport_write_result(output, {"Result": "ConfigSchemaProjectionRequired", **transport_result_identity(patch_kind, lane, name), "Output": head})
        return
    bundle = json.loads(regular_bytes(bundle_path), object_pairs_hook=unique_json_object)
    must(isinstance(bundle, dict) and set(bundle) == {"SourceHead", "BeforeBase64", "GeneratedBase64"} and all(isinstance(value, str) for value in bundle.values()))
    must(bundle["SourceHead"] == head and base64.b64decode(bundle["BeforeBase64"], validate=True) == before)
    generated = base64.b64decode(bundle["GeneratedBase64"], validate=True)
    must(isinstance(json.loads(generated), dict))
    lane, _, name = identity.partition("/")
    result = {"Result": "ConfigSchemaGenerated", **transport_result_identity(patch_kind, lane, name), "Output": "Current", "ArtifactPath": relative}
    if generated == before:
        must(target.clean() and target.text("rev-parse", "HEAD") == head and target.text("symbolic-ref", "HEAD") == branch and regular_bytes(fixture) == before)
        transport_write_result(output, result)
        return
    pending = fixture + ".release-roll-pending"
    must(not exists(pending))
    mode = stat.S_IMODE(os.lstat(fixture).st_mode)
    previous_mask = signal.pthread_sigmask(signal.SIG_BLOCK, {signal.SIGTERM})
    try:
        must(target.clean() and target.text("rev-parse", "HEAD") == head and target.text("symbolic-ref", "HEAD") == branch)
        release_replace(fixture, before, generated)
        must(target.text("rev-parse", "HEAD") == head and target.text("symbolic-ref", "HEAD") == branch and target.ok("diff", "--cached", "--quiet") and changed_paths(target) == (relative,) and regular_bytes(fixture) == generated)
        result["Output"] = "Changed"
        transport_write_result(output, result)
    except BaseException:
        if exists(pending) and regular_bytes(pending) == generated:
            os.unlink(pending)
            sync_parent(pending)
        if regular_bytes(fixture) == generated:
            replace_regular(fixture, before, mode)
        raise
    finally:
        signal.pthread_sigmask(signal.SIG_SETMASK, previous_mask)

def transport_list(arguments):
    must(len(arguments) == 4)
    source_path, tag, predecessor, output = arguments
    rows = transport_export_rows(source_path, predecessor, tag) if predecessor else transport_rows(transport_repository(source_path, True), tag, partial=True)[1]
    transport_write_result(output, {"Rows": transport_patch_records(rows), "Result": "Listed"})

def transport_patch_record(group):
    lane, kind, name, members = group
    record = {"Form": members[0][6], "Kind": kind, "Lane": lane, "Segment": members[0][7]}
    if kind == "FlatPatch":
        record["FlatPatch"] = name
    elif kind == "Unit":
        record.update(Unit=name, ChildPatches=[row[5] for row in members])
    else:
        record.update(Transform=name, TransformPatch=members[0][5])
    return record

def transport_patch_records(rows):
    return [transport_patch_record(group) for group in transport_groups(rows)]

def transport_completed_frontier(rows, terminal):
    must(isinstance(terminal, str) and terminal and rows)
    kind, identity = transport_selector(terminal)
    lane, _, name = identity.partition("/")
    must(rows[-1]["Kind"] == kind and rows[-1][kind] == name and rows[-1]["Lane"] == lane)
    return rows

def transport_fork(arguments):
    must(len(arguments) == 10)
    scratch, tag, parent_ordinal, open_selector, terminal_selector, retained_parents, main_path, parent_path, target_root, output = arguments
    main = transport_repository(main_path)
    main_branch = tag + "-main-worktree"
    prefix = main_branch + "-forked-"
    must(main.branch() == main_branch and os.path.commonpath((scratch, main.path)) == scratch)
    must(target_root == join(scratch, "forked-worktrees"))
    branches = main.text("for-each-ref", "--format=%(refname:short)", f"refs/heads/{prefix}*").splitlines()
    ordinals = [int(branch.removeprefix(prefix)) for branch in branches if re.fullmatch(re.escape(prefix) + r"[0-9]{3}", branch)]
    parent = prefix + parent_ordinal if parent_ordinal else main_branch
    retained = retained_parents.split(", ") if retained_parents else []
    must((not parent_ordinal or retained) and len(retained) == len(set(retained)) and all(os.path.isabs(path) and path != parent_path and kind(path, DIRECTORY) for path in retained))
    parent_repository = transport_repository(parent_path, True)
    common = main.text("rev-parse", "--path-format=absolute", "--git-common-dir")
    must((not parent_ordinal or parent in branches) and parent_repository.branch() == parent and common == parent_repository.text("rev-parse", "--path-format=absolute", "--git-common-dir"))
    groups = transport_groups(transport_rows(parent_repository, tag, partial=True)[1])
    transport_selector(open_selector)
    terminal_kind, terminal_identity = transport_selector(terminal_selector)
    must(transport_group_index(groups, terminal_kind, terminal_identity) == len(groups) - 1)
    name = next((f"{ordinal:03d}" for ordinal in range(1, 1000) if ordinal not in ordinals), "")
    must(re.fullmatch(r"(?!000)[0-9]{3}", name))
    branch, target = prefix + name, join(target_root, name)
    os.makedirs(target_root, mode=0o700, exist_ok=True)
    must(os.path.realpath(target_root) == target_root and not exists(target))
    main.run("worktree", "add", "--quiet", "-b", branch, target, parent)
    fork = transport_repository(target, True)
    must(fork.branch() == branch)
    agents(fork)
    transport_write_result(output, {"Output": target, "Result": "Forked"})

def transport_apply_unit(source, target, members, rustfmt, asset_root, formatter, source_bytes):
    metadata = tuple(members[0][2:8])
    if metadata[4] == TRANSFORM:
        must(len(members) == 1 and metadata == NORMALIZATION_TRANSFORM)
        return transport_transform(target, metadata, asset_root, formatter)
    applied = 0
    try:
        for row in members:
            data = source_bytes[row[0]] if source is None else patch_bytes(source, row[1], row[0], row[1])
            applied_patch = target.run("apply", data=data, check=False) if data else None
            if applied_patch is None or applied_patch.returncode:
                reason = applied_patch.stderr.decode(errors="replace").strip()[:4096] if applied_patch else "empty GitDiff"
                transport_reset(target, applied)
                return f"Patch GitDiff {row[5]}: {reason or f'git apply exit {applied_patch.returncode}'}"
            if not roll_format_patch(target, rustfmt):
                transport_reset(target, applied)
                return "Patch Rust formatting failed"
            target.run("add", "-A")
            change_class(target, row[2], row[6], row[7])
            metadata_commit(target, *row[2:8])
            applied += 1
        failure = transport_chrome(target, asset_root, True)
        if failure:
            transport_reset(target, applied)
        return failure
    except BaseException:
        transport_reset(target, applied)
        raise

def transport_replay_materialized(target, members, patches, artifacts, rustfmt):
    must(len(patches) == len(members) == len(artifacts))
    for row, data, artifact in zip(members, patches, artifacts):
        must(row[6] == "GitDiff")
        if not data or target.run("apply", data=data, check=False).returncode:
            return False, None
        if not roll_format_patch(target, rustfmt):
            return False, None
        target.run("add", "-A")
        change_class(target, row[2], row[6], row[7])
        finding = git_diff_finding(row, artifact, staged_patch_bytes(target, "HEAD"))
        if finding:
            return False, finding
        metadata_commit(target, *row[2:8])
    return True, None

def transport_revise(arguments):
    must(len(arguments) == 13)
    target_path, tag, lane, patch_kind, group, patch, form, segment, rustfmt, asset_root, child_patch_sequence, terminal_selector, output = arguments
    target = transport_repository(target_path, False)
    metadata = (lane, patch_kind, group, patch, form, segment)
    identity = f"{lane}/{group}"
    git_diff = form == "GitDiff" and patch_kind in ("FlatPatch", "Unit") and lane in LANES and formal(group, lane, patch_kind) and formal(patch) and (patch_kind == "Unit") == bool(child_patch_sequence) and segment in lane_segments(lane)
    must(kind(rustfmt, REGULAR) and git_diff)
    _, rows = transport_rows(target, tag, partial=True)
    groups = transport_groups(rows)
    group_index = transport_group_index(groups, patch_kind, identity)
    roll_require(group_index == len(groups) - 1 and terminal_selector == f"{patch_kind}:{identity}", identity, "PatchBoundary", "terminal Flat Patch or Unit", transport_group_selector(groups[-1]))
    reviewed = groups[group_index]
    selected = [position for position, row in enumerate(reviewed[3]) if row[2:8] == metadata]
    must(transport_complete(reviewed, partial=patch_kind == "Unit") and len(selected) == 1)
    patch_index = selected[0]
    must(source_change_class(changed_paths(target), identity) == "source")
    declared = child_patch_sequence.split(", ") if child_patch_sequence else []
    committed = [row[5] for row in reviewed[3]]
    must((patch_kind == "Unit" and len(declared) >= 2 and committed == declared[:len(committed)]) or patch_kind != "Unit" and not declared)
    suffix = reviewed[3][patch_index + 1:]
    patches = [patch_bytes(target, row[1], row[0], row[1]) for row in suffix]
    artifacts = {row[0]: artifact for row, artifact in projected_patch_rows([row for row in rows if row[6] == "GitDiff"])}
    suffix_artifacts = [artifacts[row[0]] for row in suffix]
    later_records = transport_patch_records(suffix)
    correction = transport_correction(target, join(asset_root, "revise.index"))
    snapshot = transport_dirty_snapshot(target, identity)
    selected_tip = reviewed[3][patch_index][0]
    phase = ["original"]
    orig_path, orig = transport_orig_head(target)

    def restore_original():
        must(phase[0] == "rewriting")
        signal.signal(signal.SIGTERM, signal.SIG_IGN)
        phase[0] = "restoring"
        target.run("reset", "--quiet", "--hard", "ORIG_HEAD")
        transport_restore_dirty(target, "HEAD", snapshot, identity)
        transport_restore_orig_head(orig_path, orig)
        phase[0] = "original"
        signal.signal(signal.SIGTERM, transport_interrupt)

    signal.signal(signal.SIGTERM, transport_interrupt)
    try:
        blocked = signal.pthread_sigmask(signal.SIG_BLOCK, {signal.SIGTERM})
        try:
            target.run("reset", "--quiet", "--hard", selected_tip)
            phase[0] = "rewriting"
        finally:
            signal.pthread_sigmask(signal.SIG_SETMASK, blocked)
        if target.run("apply", data=correction, check=False).returncode or not roll_format_patch(target, rustfmt):
            restore_original()
            transport_write_result(output, {**transport_result_identity(patch_kind, lane, group, patch), "Result": "Conflict"})
            return
        target.run("add", "-A")
        change_class(target, lane, form, segment)
        finding = git_diff_finding(reviewed[3][patch_index], artifacts[selected_tip], staged_patch_bytes(target, "HEAD^"))
        if finding:
            restore_original()
            transport_write_result(output, finding)
            return
        metadata_commit(target, *metadata, amend=True)
        replayed, finding = transport_replay_materialized(target, suffix, patches, suffix_artifacts, rustfmt)
        if not replayed:
            restore_original()
            transport_write_result(output, finding or {**transport_result_identity(patch_kind, lane, group, patch), "Result": "Conflict"})
            return
        _, revised_rows = transport_rows(target, tag, partial=True)
        revised_groups = transport_groups(revised_rows)
        must(patch_kind != "Unit" or [row[5] for row in revised_groups[group_index][3]] == committed)
        revised_later = revised_groups[group_index][3][patch_index + 1:]
        must(transport_patch_records(revised_later) == later_records)
        must(transport_group_selector(revised_groups[-1]) == terminal_selector)
        failure = transport_chrome(target, asset_root, True)
        if failure:
            restore_original()
            transport_write_result(output, {**transport_result_identity(patch_kind, lane, group, patch), "Output": failure, "Result": "Conflict"})
            return
        blocked = signal.pthread_sigmask(signal.SIG_BLOCK, {signal.SIGTERM})
        try:
            try:
                transport_write_result(output, {**transport_result_identity(patch_kind, lane, group, patch), "Result": "Revised"})
                transport_restore_orig_head(orig_path, orig)
            except BaseException:
                remove(output)
                restore_original()
                raise
            phase[0] = "completed"
        finally:
            signal.pthread_sigmask(signal.SIG_SETMASK, blocked)
    except BaseException:
        if not exists(output) and phase[0] == "rewriting":
            restore_original()
        raise

def transport_commit(arguments):
    must(len(arguments) == 12)
    target = transport_repository(arguments[0])
    tag, lane, patch_kind, group, patch, form, segment, rustfmt, child_patch_sequence, skip_line_limit, output = arguments[1:]
    must(kind(rustfmt, REGULAR) and form == "GitDiff" and skip_line_limit in ("true", "false"))
    metadata = (lane, patch_kind, group, patch, form, segment)
    identity = f"{lane}/{group}"
    result = {**transport_result_identity(patch_kind, lane, group, patch), "Result": "Committed"}
    def reject(field, expected, actual):
        transport_write_result(output, {**result, "Result": "Input", "Output": str(RollInputError(identity, field, expected, actual))})
    if lane == "mixed" and patch_kind != "Unit":
        return reject("PatchKind", "Unit", patch_kind)
    if lane == "draft" and patch_kind == "Unit" and not formal(group, lane, patch_kind):
        return reject("UnitName", "[a-z][a-z0-9]*", group)
    must(patch_kind in ("FlatPatch", "Unit") and lane in LANES and formal(group, lane, patch_kind) and formal(patch) and (patch_kind == "FlatPatch") == (group == patch) and segment in lane_segments(lane))
    declared = child_patch_sequence.split(", ") if child_patch_sequence else []
    must(patch_kind == "FlatPatch" and not declared or patch_kind == "Unit" and len(declared) >= 2 and len(declared) == len(set(declared)) and all(formal(child) for child in declared))
    _, rows = transport_rows(target, tag, partial=True)
    same_group = rows and rows[-1][6] == "GitDiff" and rows[-1][2:5] == metadata[:3]
    clean = target.clean()
    group_rows = [row for row in rows if row[2:5] == metadata[:3]] if same_group else []
    if group_rows and group_rows[0][7] != segment:
        return reject("Segment", group_rows[0][7], segment)
    current = [row[5] for row in group_rows]
    must(patch_kind == "FlatPatch" or current == declared[:len(current)])
    if clean and rows and rows[-1][2:8] == metadata:
        transport_write_result(output, result)
        return
    must(not clean)
    paths = changed_paths(target)
    if segment != "schema" and any(path == PROTOCOL_ROOT or path.startswith(PROTOCOL_ROOT + "/") for path in paths):
        return reject("Segment", "schema for protocol source", segment)
    if same_group:
        must(patch_kind == "Unit" and patch not in current and current + [patch] == declared[:len(current) + 1])
    else:
        must(metadata[:3] not in [row[2:5] for row in rows])
        must(patch_kind == "FlatPatch" or patch == declared[0])
    classification = source_change_class(paths, identity)
    must(classification == "source")
    prospective = rows + [("new", "HEAD", *metadata, classification)]
    validate_history(prospective, partial=True)
    snapshot = transport_dirty_snapshot(target, identity)

    def restore_input():
        revision = "HEAD^" if target.clean() and fields(target, "HEAD") == metadata else "HEAD"
        transport_restore_dirty(target, revision, snapshot, identity)

    signal.signal(signal.SIGTERM, transport_interrupt)
    try:
        must(roll_format_patch(target, rustfmt))
        failure = transport_chrome(target, dir_name(output), True)
        if failure:
            restore_input()
            transport_write_result(output, {**transport_result_identity(patch_kind, lane, group, patch), "Output": failure, "Result": "Conflict"})
            return
        target.run("add", "-A")
        change_class(target, lane, form, segment)
        row, artifact = projected_patch_rows([row for row in prospective if row[6] == "GitDiff"])[-1]
        finding = git_diff_finding(row, artifact, staged_patch_bytes(target, "HEAD"), skip_line_limit == "false")
        if finding:
            restore_input()
            transport_write_result(output, finding)
            return
        metadata_commit(target, *metadata)
        transport_rows(target, tag, partial=True)
        transport_write_result(output, result)
    except BaseException:
        if not exists(output):
            restore_input()
        raise

def export_preimage(repository, tag):
    must(repository.branch() == "main" and not repository.sequencer() and repository.ok("diff", "--cached", "--quiet"))
    must(kind(join(repository.path, ".git"), DIRECTORY))
    dirty = changed_paths(repository)
    must(all("/" in path and re.fullmatch(r"rust-v[0-9A-Za-z._-]+", path.split("/", 1)[0]) and not path.split("/", 1)[0].endswith(("-prev", ".pending")) for path in dirty))
    must(not repository.bytes("ls-files", "--", tag + "-prev"))
    names = sorted(name for name in os.listdir(repository.path) if re.fullmatch(r"rust-v[0-9A-Za-z._-]+", name))
    must(all(not name.endswith(".pending") for name in names))
    directories = []
    for name in names:
        root = join(repository.path, name)
        directories.append((name, source_prefix_preimage(root)))
    for name in (tag, tag + "-prev"):
        if name not in names:
            directories.append((name, None))
    return repository.text("rev-parse", "HEAD"), tuple(sorted(directories)), tuple(dirty), repository.text("for-each-ref", "--format=%(refname) %(objectname)")

def exchange_export_directory(current, staged):
    library = ctypes.CDLL(None, use_errno=True)
    exchange = library.renameat2
    exchange.argtypes = (ctypes.c_int, ctypes.c_char_p, ctypes.c_int, ctypes.c_char_p, ctypes.c_uint)
    exchange.restype = ctypes.c_int
    if exchange(-100, os.fsencode(current), -100, os.fsencode(staged), 2) != 0:
        error = ctypes.get_errno()
        raise OSError(error, os.strerror(error), current)

def transport_export(arguments):
    must(len(arguments) == 8)
    source = transport_repository(arguments[0], True)
    working, scratch, private_root, tag, open_patch, skip_line_limit, output = arguments[1:]
    must(skip_line_limit in ("true", "false"))
    working, private_root = os.path.abspath(working), os.path.abspath(private_root)
    must(os.path.isabs(scratch) and kind(scratch, DIRECTORY) and os.path.realpath(scratch) == scratch)
    must(private_root == join(working, "build", "patches", "private"))
    head = source.text("rev-parse", "HEAD")
    base = transport_base(source, tag)
    rows = validate_history(history_rows(source, base, head))
    records = transport_patch_records(rows)
    must(records)
    if open_patch in map(transport_group_selector, transport_groups(rows)):
        transport_write_result(output, {"Result": "Input", "Output": str(RollInputError(open_patch, "OpenPatch", "TaskWorkspace closure before export", open_patch))})
        return
    projected, findings = project(source, base, head, skip_line_limit=skip_line_limit == "true")
    if findings:
        transport_write_result(output, findings[0])
        return
    parent, staged_root = dir_name(private_root), private_root + ".pending"
    os.makedirs(parent, exist_ok=True)
    must(kind(parent, DIRECTORY) and os.path.realpath(parent) == parent and not exists(staged_root))
    private = transport_repository(private_root) if exists(private_root) else None
    before = export_preimage(private, tag) if private else None
    lane_directories = {name.split("/", 1)[0] for name, _ in projected}
    lanes = [lane for lane in LANES if lane_path(lane) in lane_directories]
    notes_root = join(scratch, "lane-notes", tag)
    notes = release_notes(notes_root, lanes)
    missing_notes = {"notes.md", *(join(lane_path(lane), "notes.md") for lane in lanes)} - notes.keys()
    if missing_notes:
        raise RuntimeError("missing working notes under " + notes_root + ": " + ", ".join(sorted(missing_notes)))
    projected = tuple(sorted((*projected, *notes.items())))
    clone_seed = ""
    if private is None:
        working_repository = transport_repository(working)
        must(working_repository.branch() == "main")
        clone_seed = working_repository.text("rev-parse", "HEAD^{commit}")
    published, stage_owned = False, False
    result = {"Output": join(private_root, tag), "Rows": records, "Result": "Exported"}
    signal.signal(signal.SIGTERM, transport_interrupt)
    try:
        os.mkdir(staged_root)
        stage_owned = True
        if private:
            shutil.copytree(private_root, staged_root, symlinks=True, dirs_exist_ok=True)
        else:
            git_process(("clone", "--quiet", "--shared", working, staged_root))
        staged = transport_repository(staged_root)
        must(not clone_seed or staged.text("rev-parse", "HEAD") == clone_seed)
        staged_before = export_preimage(staged, tag)
        must(before is None or staged_before == before)
        target, previous = join(staged_root, tag), join(staged_root, tag + "-prev")
        remove(previous)
        if exists(target):
            os.rename(target, previous)
        write_project(target, projected)
        exclude = join(staged_root, ".git", "info", "exclude")
        ignored = regular_bytes(exclude) if exists(exclude) else b""
        if b"/rust-v*-prev/" not in ignored.splitlines():
            remove(exclude)
            write(exclude, ignored + b"\n/rust-v*-prev/\n", 0o644)
        staged.run("add", "-f", "-A", "--", tag)
        staged.run("commit", "--quiet", "--no-verify", "--allow-empty", "-m", "update " + tag)
        must(commit_identity(staged, "HEAD") and staged.ok("diff", "--cached", "--quiet"))
        must(valid_export_commit(staged, tag))
        project_matches_export(source, base, target, head)
        previous_contents = tuple((path, stat.S_IMODE(os.lstat(join(previous, path)).st_mode), regular_bytes(join(previous, path))) for path in root_inventory(previous, staged_root, True))
        must((previous_contents if exists(previous) else None) == dict(staged_before[1])[tag])
        must(source.clean() and source.text("rev-parse", "HEAD") == head)
        must(export_preimage(private, tag) == before if private else not exists(private_root))
        must(release_notes(notes_root, lanes) == notes)
        blocked = signal.pthread_sigmask(signal.SIG_BLOCK, {signal.SIGTERM})
        try:
            if private:
                exchange_export_directory(private_root, staged_root)
            else:
                os.rename(staged_root, private_root)
            published = True
        finally:
            signal.pthread_sigmask(signal.SIG_SETMASK, blocked)
        sync_parent(private_root)
        remove(staged_root)
        transport_write_result(output, result)
    except BaseException as error:
        if not published:
            if stage_owned:
                remove(staged_root)
            raise
        result.update(Result="PublicationFailed", Diagnostic=str(error)[:4096])
        transport_write_result(output, result)

def transport_verify_export(arguments):
    must(len(arguments) == 4)
    source = transport_repository(arguments[0], True)
    private = transport_repository(arguments[1])
    tag, output = arguments[2:]
    must(private.branch() == "main")
    must(valid_export_commit(private, tag))
    base = transport_base(source, tag)
    rows = history_rows(source, base)
    validate_history(rows)
    _, findings = project(source, base)
    must(not findings)
    project_matches_export(source, base, join(private.path, tag))
    transport_write_result(output, {"Rows": transport_patch_records(rows), "Result": "Verified"})

def runner_complete_snapshot_manifest(arguments):
    must(len(arguments) == 3)
    source, overlay, output = arguments
    try:
        data = snapshot_manifest(source, overlay, complete_snapshot_targets(source, overlay))
    except RollInputError as error:
        print(error, file=sys.stderr)
        raise SystemExit(1) from None
    write(output, data)

def regular_bytes(path):
    descriptor = os.open(path, os.O_RDONLY | os.O_CLOEXEC | os.O_NOFOLLOW)
    try:
        before = os.fstat(descriptor)
        must(stat.S_IFMT(before.st_mode) == REGULAR)
        chunks = []
        while data := os.read(descriptor, 1 << 20):
            chunks.append(data)
        after = os.fstat(descriptor)
    finally:
        os.close(descriptor)
    current = os.lstat(path)
    identity = lambda item: (item.st_dev, item.st_ino, item.st_mode, item.st_size, item.st_mtime_ns, item.st_ctime_ns)
    must(identity(before) == identity(after) == identity(current))
    return b"".join(chunks)

def custody_projection(arguments):
    must(len(arguments) == 2)
    source, output = transport_repository(arguments[0]), arguments[1]
    write_custody_projection(source, output)


def custody_maintenance_projection(arguments):
    must(len(arguments) == 2)
    source, output = transport_repository(arguments[0]), arguments[1]
    must(not source.sequencer())
    write_custody_projection(source, output)


def write_custody_projection(source, output):
    clean = str(source.clean()).lower()
    write(output, f"{clean}\n{source.branch()}\n".encode())


def release_read(path):
    data = regular_bytes(path)
    value = json.loads(data, object_pairs_hook=unique_json_object)
    must(data == canonical(value))
    return value, data

def release_same(repository, left, right):
    return repository.ok("merge-base", "--is-ancestor", left, right) and repository.ok("merge-base", "--is-ancestor", right, left)

def agents(*repos):
    for repo in repos:
        path = join(repo.path, "AGENTS.override.md")
        if not exists(path):
            write(path, b"", 0o644)
        must(kind(path, REGULAR) and not regular_bytes(path) and repo.ok("check-ignore", "-q", path))

def release_fixed(repository, main_path, upstream_path, tag):
    main = transport_repository(main_path, True)
    upstream = transport_repository(upstream_path, True)
    common = repository.text("rev-parse", "--path-format=absolute", "--git-common-dir")
    must(main.branch() == tag + "-main-worktree" and upstream.branch() == tag + "-upstream-baseline")
    must(common == main.text("rev-parse", "--path-format=absolute", "--git-common-dir") == upstream.text("rev-parse", "--path-format=absolute", "--git-common-dir"))
    agents(main, upstream)
    release = "refs/tags/" + tag + "^{}"
    marker = "refs/heads/" + tag + "-upstream-baseline"
    must(repository.has_ref("refs/tags/" + tag) and repository.ok("cat-file", "-e", release + "^") and not repository.ok("cat-file", "-e", release + "^2"))
    must(main.ok("merge-base", "--is-ancestor", release + "^", "HEAD"))
    must(not repository.ok("cat-file", "-e", marker + "^2") and release_same(repository, marker + "^", release + "^") and repository.ok("diff", "--quiet", marker + "^", marker) and upstream.text("show", "-s", "--format=%s", "HEAD") == tag + " baseline marker")
    return main, release

def release_task_fields(data):
    fields = {}
    for line in data.decode().splitlines():
        if line.startswith(("+-- ", "`-- ")):
            name, separator, value = line[4:].partition(": ")
            if name in ("CurrentWorktree", "CurrentTerminalUnit", "CurrentTerminalFlatPatch", "CurrentTerminalTransform", "RemainingSuffix", "RetainedParents", "OpenUnit", "OpenFlatPatch", "OpenTransform", "SourcePrefixes"):
                must(separator and value and value == value.strip() and name not in fields)
                fields[name] = value
    return fields

def release_task_export_frontier(data, main):
    fields = release_task_fields(data)
    must(fields.get("CurrentWorktree") == main and fields.get("RemainingSuffix") == "empty")
    open_fields = [name for name in ("OpenUnit", "OpenFlatPatch", "OpenTransform") if name in fields]
    terminal_fields = [(kind, fields[name]) for kind, name in (("Unit", "CurrentTerminalUnit"), ("FlatPatch", "CurrentTerminalFlatPatch"), ("Transform", "CurrentTerminalTransform")) if name in fields]
    must("RetainedParents" not in fields and not open_fields and len(terminal_fields) == 1)
    return f"{terminal_fields[0][0]}:{terminal_fields[0][1]}"

def release_replace(path, before, after):
    must(regular_bytes(path) == before)
    replace_regular(path, after, stat.S_IMODE(os.lstat(path).st_mode))

def release_remove_worktree(repository, path):
    if exists(path):
        must(repository.run("worktree", "remove", path, check=False).returncode == 0 and not exists(path))

def release_inputs(arguments):
    must(len(arguments) == 12)
    working, scratch, records, tag, task_path, predecessor, main_path, upstream_path, task_text, index_text, next_index, output = arguments
    must(all(os.path.isabs(path) and os.path.abspath(path) == path for path in (working, scratch, records, task_path, main_path, upstream_path)))
    must(records == join(scratch, "records") and task_path == join(scratch, "tasks", f"release-roll-{tag}.md"))
    must(main_path == join(scratch, "main-worktree", "source") and upstream_path == join(scratch, "upstream-baseline", "source"))
    repository = transport_repository(working)
    must(repository.branch() == "main" and os.path.realpath(scratch) == scratch)
    agents(repository)
    index_path = join(scratch, "tasks", "index.json")
    index, index_before = release_read(index_path)
    must(index_before == index_text.encode() and set(index) == {"ActiveTask", "OtherTasks", "OverallTask"})
    must(all(isinstance(index[name], list) for name in index) and len(index["ActiveTask"]) <= 1 and not index["OtherTasks"] and len(index["OverallTask"]) <= 1)
    task_before = task_text.encode()
    must(regular_bytes(task_path) == task_before if task_before else not exists(task_path))
    if task_before:
        prefixes = release_task_fields(task_before).get("SourcePrefixes", "").split(", ")
        must(prefixes and prefixes[0] == predecessor and predecessor != tag)
    must(re.fullmatch(r"rust-v[0-9A-Za-z._-]+", tag))
    must(re.fullmatch(r"rust-v[0-9A-Za-z._-]+", predecessor) and predecessor != tag if task_before else not predecessor)
    return repository, working, scratch, records, tag, task_path, predecessor, main_path, upstream_path, task_before, index_path, index, index_before, next_index, output

def release_baseline_start(arguments):
    repository, working, scratch, records, tag, task_path, predecessor, main_path, upstream_path, task_before, index_path, index, index_before, next_index, output = release_inputs(arguments)
    must(index["ActiveTask"] == [os.path.basename(task_path)] and task_before and not next_index)
    private_root = join(working, "build", "patches", "private")
    source_root = join(private_root, predecessor)
    source_before = source_prefix_preimage(source_root) if exists(source_root) else None
    if source_before is not None:
        transport_export_rows(source_root, predecessor, tag)
        must(source_before == source_prefix_preimage(source_root))
    baseline_path = join(scratch, "tasks", "rust-suite-baselines.json")
    baselines, _ = release_read(baseline_path)
    must(isinstance(baselines, dict))
    current = baselines.get(tag)
    inherited = baselines.get(predecessor)
    must(inherited is None or isinstance(inherited, dict) and set(inherited) == {"Established"})
    selection = inherited["Established"]["Selection"] if inherited is not None else None
    if selection is not None:
        must(selection["Tag"] == predecessor)
    bootstrap = selection is None or all(not selection[name] for name in ("CoveredPackages", "CompileOnlyPackages", "HelperBinaries", "ExpectedFailures", "ExpectedTimeouts"))
    if current is not None:
        must(exists(main_path) and exists(upstream_path))
        release_fixed(repository, main_path, upstream_path, tag)
        must(isinstance(current, dict) and set(current) == {"Established"} and current["Established"]["Selection"]["Tag"] == tag)
        must(regular_bytes(task_path) == task_before and regular_bytes(index_path) == index_before)
        transport_write_result(output, {"Bootstrap": False, "PredecessorTag": predecessor, "Ready": True, "Result": "Started"})
        return
    common = repository.text("rev-parse", "--path-format=absolute", "--git-common-dir")
    roles = (
        (main_path, "main-worktree", False),
        (upstream_path, "upstream-baseline", True),
    )
    path_tags = set()
    for path, role, _ in roles:
        if exists(path):
            current = transport_repository(path, True)
            must(current.text("rev-parse", "--path-format=absolute", "--git-common-dir") == common)
            branch = current.branch()
            must(branch in (predecessor + "-" + role, tag + "-" + role))
            path_tags.add(branch.removesuffix("-" + role))
    must(len(path_tags) <= 1)
    predecessor_release = "refs/tags/" + predecessor + "^{}"
    predecessor_base = predecessor_release + "^"
    must(repository.has_ref("refs/tags/" + predecessor) and repository.ok("cat-file", "-e", predecessor_base) and not repository.ok("cat-file", "-e", predecessor_release + "^2"))
    predecessor_refs = {}
    for _, role, marker in roles:
        reference = "refs/heads/" + predecessor + "-" + role
        if not repository.has_ref(reference):
            continue
        revision = repository.text("rev-parse", reference)
        if marker:
            must(repository.text("show", "-s", "--format=%s", reference) == predecessor + " baseline marker" and release_same(repository, reference + "^", predecessor_base) and repository.ok("diff", "--quiet", reference + "^", reference))
        elif source_before is None:
            must(selection is not None and release_same(repository, reference, predecessor_base))
        else:
            project_matches_export(repository, predecessor_base, source_root, reference)
        predecessor_refs[reference] = revision
    must(predecessor not in path_tags or len(predecessor_refs) == 2)
    forks = join(scratch, "forked-worktrees")
    must(not exists(forks) or kind(forks, DIRECTORY) and not os.listdir(forks))
    tag_ref = "refs/tags/" + tag
    signal.signal(signal.SIGTERM, transport_interrupt)
    fetch = repository.run("fetch", "--quiet", "--no-tags", "origin", f"refs/tags/{tag}:refs/tags/{tag}", check=False)
    must(fetch.returncode == 0)
    release = tag_ref + "^{}"
    must(repository.ok("cat-file", "-e", release + "^") and not repository.ok("cat-file", "-e", release + "^2"))
    base = release + "^"
    for path, role, _ in roles:
        if exists(path) and transport_repository(path, True).branch() == predecessor + "-" + role:
            release_remove_worktree(repository, path)
    for reference, revision in predecessor_refs.items():
        repository.run("update-ref", "-d", reference, revision)
    for path, role, marker in roles:
        branch = tag + "-" + role
        reference = "refs/heads/" + branch
        if not exists(path):
            os.makedirs(dir_name(path), exist_ok=True)
            must(os.path.realpath(dir_name(path)) == dir_name(path))
            blocked = signal.pthread_sigmask(signal.SIG_BLOCK, {signal.SIGTERM})
            try:
                if repository.has_ref(reference):
                    repository.run("worktree", "add", "--quiet", path, branch)
                else:
                    repository.run("worktree", "add", "--quiet", "-b", branch, path, base)
            finally:
                signal.pthread_sigmask(signal.SIG_SETMASK, blocked)
        current = transport_repository(path, True)
        must(current.branch() == branch and current.text("rev-parse", "--path-format=absolute", "--git-common-dir") == common)
        if marker:
            if release_same(current, "HEAD", base):
                native_commit(current, "--allow-empty", "-m", tag + " baseline marker")
            must(current.text("show", "-s", "--format=%s", "HEAD") == tag + " baseline marker" and release_same(current, "HEAD^", base) and current.ok("diff", "--quiet", "HEAD^", "HEAD"))
        else:
            must(release_same(current, "HEAD", base))
    release_fixed(repository, main_path, upstream_path, tag)
    cargo_home = join(working, "build", "cache", "cargo-home")
    must(kind(cargo_home, DIRECTORY) and not any(exists(join(cargo_home, name)) for name in ("config", "config.toml", "credentials", "credentials.toml")))
    environment = {**ENV, "CARGO_HOME": cargo_home, "CARGO_TERM_COLOR": "never", "HOME": "/nonexistent"}
    must(subprocess.run((join(working, "build", "toolchains", "native", "toolchain", "rust", "bin", "cargo"), "fetch", "--locked"), cwd=join(upstream_path, "codex-rs"), env=environment).returncode == 0)
    release_fixed(repository, main_path, upstream_path, tag)
    must((source_before is None and not exists(source_root) or source_before is not None and source_prefix_preimage(source_root) == source_before) and regular_bytes(task_path) == task_before and regular_bytes(index_path) == index_before)
    transport_write_result(output, {"Bootstrap": bootstrap, "PredecessorTag": predecessor, "Ready": False, "Result": "Started"})

def release_retire_notes(working, scratch, tag):
    notes_root = join(scratch, "lane-notes", tag)
    if not os.path.lexists(notes_root):
        return
    notes = working_release_notes(notes_root)
    published = join(working, "build", "patches", "private", tag)
    if exists(published):
        private = transport_repository(dir_name(published))
        must(private.branch() == "main" and valid_export_commit(private, tag))
    published_notes = release_notes(published)
    if any(data != published_notes.get(path, b"") for path, data in notes.items()):
        raise RuntimeError("working notes differ from the final export; cleanup retained drafts")
    must(working_release_notes(notes_root) == notes)
    for relative, data in sorted(notes.items()):
        path = join(notes_root, relative)
        must(regular_bytes(path) == data)
        os.unlink(path)
        sync_parent(path)
    if os.path.lexists(notes_root):
        must(working_release_notes(notes_root) == {})
        for directory in sorted(os.listdir(notes_root)):
            os.rmdir(join(notes_root, directory))
        os.rmdir(notes_root)
        sync_parent(notes_root)

def release_baseline_cleanup(arguments):
    repository, working, scratch, records, tag, task_path, predecessor, main_path, upstream_path, task_before, index_path, index, index_before, next_index, output = release_inputs(arguments)
    must(index["ActiveTask"] == [os.path.basename(task_path)] and task_before and not next_index)
    release_fixed(repository, main_path, upstream_path, tag)
    baselines, _ = release_read(join(scratch, "tasks", "rust-suite-baselines.json"))
    value = baselines.get(tag, {})
    must(set(value) == {"Established"} and value["Established"]["Selection"]["Tag"] == tag)
    release_retire_notes(working, scratch, predecessor)
    transport_write_result(output, {"PredecessorTag": predecessor, "Result": "Cleaned"})

def cleanup_temporary_roots(working, scratch, records, runtime):
    roots = (records, join(working, "build", "tmp"), runtime)
    inventory = {}
    for root in roots:
        if not os.path.lexists(root):
            continue
        roll_require(kind(root, DIRECTORY) and os.path.realpath(root) == root, "Cleanup", "Root", "real directory", root)
        for name in sorted(os.listdir(root)):
            path = join(root, name)
            info = os.lstat(path)
            inventory[path] = (info.st_dev, info.st_ino)
    process = join(records, "process")
    if exists(process):
        roll_require(kind(process, DIRECTORY), "Cleanup", "ProcessRoot", "real directory", process)
        for attempt in os.listdir(process):
            directory = join(process, attempt)
            roll_require(kind(directory, DIRECTORY), "Cleanup", "Process", "real directory", directory)
            for name in os.listdir(directory):
                roll_require(name in {base + suffix for base in ("lock.json", "add.json", "terminal.json") for suffix in ("", ".pending", ".stage")}, "Cleanup", "ProcessFile", "current process receipt", name)
                receipt, _ = release_read(join(directory, name))
                roll_require(receipt.get("AttemptID") == attempt, "Cleanup", "AttemptID", attempt, receipt.get("AttemptID"))
                if not name.startswith("add.json"):
                    continue
                group = receipt.get("ProcessGroupID")
                roll_require(type(group) is int and group > 0 and receipt.get("Transition") == "Add", "Cleanup", "ProcessGroup", "positive Add group", group)
                try:
                    os.killpg(group, 0)
                except ProcessLookupError:
                    continue
                raise RollInputError("Cleanup", "ProcessGroup", "reaped producer", group)
    with open("/proc/self/mountinfo", encoding="utf-8") as stream:
        mounts = [re.sub(r"\\([0-7]{3})", lambda match: chr(int(match[1], 8)), line.split()[4]) for line in stream]
    for path in inventory:
        roll_require(not any(mount == path or mount.startswith(path + os.sep) for mount in mounts), "Cleanup", "Mount", "unmounted private data", path)
    def accessible(directory):
        descriptor = os.open(directory, os.O_RDONLY | os.O_DIRECTORY | os.O_NOFOLLOW) if os.access(directory, os.R_OK | os.X_OK) else None
        if descriptor is None:
            os.chmod(directory, 0o700, follow_symlinks=False)
            descriptor = os.open(directory, os.O_RDONLY | os.O_DIRECTORY | os.O_NOFOLLOW)
        try:
            os.fchmod(descriptor, 0o700)
            for name in os.listdir(descriptor):
                if stat.S_ISDIR(os.stat(name, dir_fd=descriptor, follow_symlinks=False).st_mode):
                    accessible(join(directory, name))
        finally:
            os.close(descriptor)
    for path, identity in inventory.items():
        info = os.lstat(path)
        roll_require((info.st_dev, info.st_ino) == identity, "Cleanup", "Preimage", identity, path)
        if stat.S_ISDIR(info.st_mode):
            accessible(path)
            shutil.rmtree(path)
        else:
            os.unlink(path)
    for root in roots:
        if exists(root):
            roll_require(not os.listdir(root), "Cleanup", "Postimage", "empty root", root)

def release_cleanup(arguments):
    if len(arguments) == 4:
        cleanup_temporary_roots(*arguments)
        return
    repository, working, scratch, records, tag, task_path, predecessor, main_path, upstream_path, task_before, index_path, index, index_before, next_text, output = release_inputs(arguments)
    next_index = json.loads(next_text, object_pairs_hook=unique_json_object)
    must(canonical(next_index) == next_text.encode() and set(next_index) == set(index))
    task_name = os.path.basename(task_path)
    must(index["ActiveTask"] in ([task_name], []) and not index["OtherTasks"])
    must(not next_index["ActiveTask"] and not next_index["OtherTasks"] and next_index["OverallTask"] == index["OverallTask"])
    must(task_before if index["ActiveTask"] else True)
    remaining_tasks = {name: regular_bytes(join(scratch, "tasks", name)) for name in index["OverallTask"]}
    main, _ = release_fixed(repository, main_path, upstream_path, tag)
    head = main.text("rev-parse", "HEAD")
    rows = validate_history(history_rows(main, transport_base(main, tag)))
    if task_before:
        transport_completed_frontier(transport_patch_records(rows), release_task_export_frontier(task_before, main_path))
    private = transport_repository(join(working, "build", "patches", "private"))
    must(private.branch() == "main" and valid_export_commit(private, tag))
    project_matches_export(main, transport_base(main, tag), join(private.path, tag))
    must(regular_bytes(index_path) == index_before)
    release_retire_notes(working, scratch, tag)
    if next_text.encode() != index_before:
        release_replace(index_path, index_before, next_text.encode())
    if task_before:
        must(regular_bytes(task_path) == task_before)
        remove(task_path)
        sync_parent(task_path)
    must(regular_bytes(index_path) == next_text.encode() and not exists(task_path) and main.clean() and main.text("rev-parse", "HEAD") == head)
    must(all(regular_bytes(join(scratch, "tasks", name)) == content for name, content in remaining_tasks.items()))
    transport_write_result(output, {"PredecessorTag": predecessor, "Result": "Cleaned"})


COMMANDS = {
    "custody-source-projection": custody_projection,
    "custody-maintenance-source-projection": custody_maintenance_projection,
    "custody-detached-source-projection": custody_projection,
    "patch-replay-apply": transport_apply,
    "patch-replay-commit": transport_commit,
    "patch-replay-config-schema": transport_config_schema,
    "patch-replay-export": transport_export,
    "patch-replay-finalize": transport_finalize,
    "patch-replay-fork": transport_fork,
    "patch-replay-list": transport_list,
    "patch-replay-prepare": transport_prepare,
    "patch-replay-revise": transport_revise,
    "patch-replay-verify-export": transport_verify_export,
    "runner-complete-snapshot-manifest": runner_complete_snapshot_manifest,
    "runner-test-selection": runner_test_selection,
    "rust-suite-packages": rust_suite_packages,
    "release-baseline-cleanup": release_baseline_cleanup,
    "release-cleanup": release_cleanup,
    "release-baseline-start": release_baseline_start,
    "rust-suite-execute": rust_suite_execute,
}


must(len(sys.argv) > 1 and sys.argv[1] in COMMANDS)
try:
    COMMANDS[sys.argv[1]](sys.argv[2:])
except LaneStructureError as error:
    if not sys.argv[1].startswith("patch-replay-"):
        raise
    transport_write_result(sys.argv[-1], {"Output": str(error), "Result": "Input"})
