from pathlib import Path
import re
import subprocess
import sys
import unicodedata

FONT_FAMILY = "Cascadia Mono PL"
FONT_STYLE = "Regular"
TUI_SOURCE = Path("codex-rs/tui/src")
TUI_SUITE_SNAPSHOTS = Path("codex-rs/tui/tests/suite/snapshots")
MERMAID_SOURCE = Path("codex-rs/mermaid/src")
SOURCE_ROOTS = (TUI_SOURCE, TUI_SUITE_SNAPSHOTS, MERMAID_SOURCE)

CHECK_MARK = "+"
CLOSE_MARK = "x"
WARNING_MARK = "!"
INFORMATION_MARK = "i"
WAVE_MARK = "~"
SQUARE = "▪"
SPACED_SQUARE = SQUARE + " "
BLACK_SQUARE = "■"
WHITE_SQUARE = "□"
WHITE_SMALL_SQUARE = "▫"
DOTTED_SQUARE = "⬚"
CORNERS = "┌┐└┘"
ELLIPSIS = "…"
RIGHT_MARK = "›"
UP_MARK = "↑"
DOWN_MARK = "↓"
SHIFT_MARK = "⇡"
SKIPPED_MATH_SOURCE_PREFIX = "tui/src/markdown_render/math"
SKIPPED_MATH_SOURCES = {
    "tui/src/streaming/inline_math_tests.rs",
    "tui/src/streaming/math_tests.rs",
}


def characters(codepoints: str) -> str:
    return "".join(chr(int(codepoint, 16)) for codepoint in codepoints.split())


def replacements(codepoints: str, replacement: str) -> dict:
    return dict.fromkeys(characters(codepoints), replacement)


VISIBLE_REPLACEMENTS = {
    **replacements("25E6", WHITE_SMALL_SQUARE),
    **replacements("2713 2714 2611", CHECK_MARK),
    **replacements("2717 2718 00D7", CLOSE_MARK),
    **replacements("26A0", WARNING_MARK),
    **replacements("24D8", INFORMATION_MARK),
    **replacements("25CC", DOTTED_SQUARE),
    **replacements("25CB 2727 2610", WHITE_SQUARE),
    **replacements("25CF", SQUARE),
    **replacements("2726", BLACK_SQUARE),
    **replacements("2728 1F50C 1F512", SPACED_SQUARE),
    **replacements("22EE", ELLIPSIS),
    **replacements("21B3 21B5 21E5", RIGHT_MARK),
    **replacements("2303", UP_MARK),
    **replacements("2304", DOWN_MARK),
    **replacements("21E7", SHIFT_MARK),
    **replacements("2301 2325", WAVE_MARK),
}
ROUNDED_BORDERS = characters("256D 256E 2570 256F")
ROUNDED_BORDER_TRANSLATION = str.maketrans(dict(zip(ROUNDED_BORDERS, CORNERS)))

UNICODE_SEMANTICS_FILES = {
    "codex-rs/tui/src/bottom_pane/chat_composer.rs",
    "codex-rs/tui/src/bottom_pane/textarea.rs",
    "codex-rs/tui/src/live_wrap.rs",
    "codex-rs/tui/src/mention_codec.rs",
    "codex-rs/tui/src/streaming/controller.rs",
    "codex-rs/tui/src/text_formatting.rs",
    "codex-rs/tui/src/wrapping.rs",
}
UNICODE_PAYLOAD_SYMBOLS = {
    "mermaid/src/families_tests.rs": characters("1F680"),
    "tui/src/app/agent_center_tests.rs": characters("1F469 1F4BB"),
    "tui/src/transcript_view/prompt_style_tests.rs": characters("1F469 1F4BB"),
    "tui/src/bottom_pane/selection_popup_common.rs": characters("1F44D 1F3FB 1F468 1F469 1F467 1F466"),
    "tui/src/bottom_pane/textarea.rs": characters("274C"),
    "tui/src/bottom_pane/textarea/hyperlinks_tests.rs": characters("1F469 1F4BB"),
    "tui/src/diff_render.rs": characters("1F680 2728"),
    "tui/src/history_cell/search_tests.rs": characters("1F980"),
    "tui/src/streaming/render_tests.rs": characters("2728"),
}
RUST_REGIONS = re.compile(
    r'(?P<raw>(?:br|cr|r)(?P<hashes>#{0,255})".*?"(?P=hashes))'
    r'|(?P<string>(?:b|c)?"(?:\\.|[^"\\])*")'
    r"|(?P<character>b?'(?:\\u\{[0-9a-fA-F_]+\}|\\x[0-9a-fA-F]{2}|\\.|[^'\\\n])')"
    r'|(?P<line>//[^\n]*)|(?P<block>/\*)',
    re.DOTALL,
)

def artifact_source(path: str, text: str) -> tuple:
    if not path.endswith(".snap"):
        return path.removeprefix("codex-rs/"), 0
    lines = text.split("\n")
    if not lines or lines[0].rstrip() != "---":
        raise ValueError(f"snapshot metadata is absent: {path}")
    end = next(index for index in range(1, len(lines)) if lines[index].rstrip() == "---")
    sources = [line.removeprefix("source: ") for line in lines[1:end] if line.startswith("source: ")]
    if len(sources) != 1:
        raise ValueError(f"snapshot source metadata differs: {path}")
    return sources[0], end + 1

def unicode_payload(source: str) -> set:
    payload = set(UNICODE_PAYLOAD_SYMBOLS.get(source, ""))
    if "/empty_state_animation" in source or source == "tui/src/onboarding/welcome.rs":
        payload.update(map(chr, range(0x2800, 0x2900)))
    return payload

def font_characters() -> set:
    output = subprocess.check_output(
        ["/usr/bin/fc-match", "-f", "%{family}\n%{style}\n%{charset}\n", f"{FONT_FAMILY}:style={FONT_STYLE}"],
        text=True,
    )
    lines = output.splitlines()
    if len(lines) != 3 or lines[:2] != [FONT_FAMILY, FONT_STYLE] or not lines[2]:
        raise ValueError(f"{FONT_FAMILY} {FONT_STYLE} font is unavailable")
    characters = set()
    for interval in lines[2].split():
        start, separator, end = interval.partition("-")
        characters.update(map(chr, range(int(start, 16), int(end if separator else start, 16) + 1)))
    return characters

def unknown_snapshot_symbols(path: str, text: str, font_charset: set) -> dict:
    source, body = artifact_source(path, text)
    payload = unicode_payload(source)
    unknown = {}
    for line_number, line in enumerate(text.split("\n")[body:], body + 1):
        for char in sorted(set(line)):
            symbol = unicodedata.category(char).startswith("S") or unicodedata.category(char) == "Co" or char in VISIBLE_REPLACEMENTS
            if ord(char) > 127 and symbol and char not in font_charset and char not in payload:
                unknown.setdefault(char, (path, line_number))
    return unknown

def rust_comments(text: str, payload: set, expand_escapes=False) -> tuple:
    prefix = "/* codex-chrome-comment-"
    while prefix in text:
        prefix += "_"
    pieces, comments, position = [], {}, 0
    while match := RUST_REGIONS.search(text, position):
        start, end = match.span()
        pieces.append(text[position:start])
        fragment = text[start:end]
        if match.lastgroup == "block":
            depth = 1
            while depth:
                delimiter = re.search(r"/\*|\*/", text[end:])
                if delimiter is None:
                    raise ValueError("unterminated Rust block comment")
                depth += 1 if delimiter.group() == "/*" else -1
                end += delimiter.end()
            fragment = text[start:end]
        if match.lastgroup in {"line", "block"}:
            placeholder = f"{prefix}{len(comments)} */"
            comments[placeholder] = fragment
            fragment = placeholder
        elif expand_escapes and match.lastgroup in {"string", "character"} and not fragment.startswith(("b", "c")):
            def expand(escape):
                slashes, value = escape.groups()
                char = chr(int(value.replace("_", ""), 16))
                owned = char in VISIBLE_REPLACEMENTS or char in ROUNDED_BORDERS
                if len(slashes) % 2 == 0 or not owned or char in payload:
                    return escape.group()
                replacement = char
                if match.lastgroup == "character" and len(replacement) != 1:
                    replacement = SQUARE
                return slashes[:-1] + replacement

            fragment = re.sub(r"(\\+)u\{([0-9a-fA-F_]+)\}", expand, fragment)
        pieces.append(fragment)
        position = end
    pieces.append(text[position:])
    return "".join(pieces), comments

def normalize_visible(text: str, payload: set) -> str:
    return text.translate(
        str.maketrans({source: replacement for source, replacement in VISIBLE_REPLACEMENTS.items() if source not in payload})
    )

def normalize_path(path: str, text: str) -> str:
    source, body = artifact_source(path, text)
    header = "".join(text.splitlines(keepends=True)[:body])
    text = text[len(header):]
    payload = unicode_payload(source)
    comments = {}
    if path.endswith(".rs"):
        text, comments = rust_comments(
            text,
            payload,
            path not in UNICODE_SEMANTICS_FILES,
        )
    normalized = text.translate(ROUNDED_BORDER_TRANSLATION)
    if path == "codex-rs/tui/src/onboarding/auth.rs":
        normalized = normalized.replace(".border_type(BorderType::Rounded)", ".border_type(BorderType::Plain)")
    if path not in UNICODE_SEMANTICS_FILES:
        normalized = normalize_visible(normalized, payload)
    for placeholder, comment in comments.items():
        if normalized.count(placeholder) != 1:
            raise ValueError(f"Rust comment boundary changed: {path}")
        normalized = normalized.replace(placeholder, comment)
    return header + normalized

def main():
    if sys.argv[1:] not in ([], ["--check"]) or not all(root.is_dir() for root in SOURCE_ROOTS):
        raise SystemExit("terminal-chrome invocation or source differs")
    check = bool(sys.argv[1:])
    font_charset = font_characters()
    changes, unknown = {}, {}
    paths = sorted(path for root in SOURCE_ROOTS for path in root.rglob("*"))
    for path in paths:
        if path.suffix not in {".rs", ".snap"}:
            continue
        posix = path.as_posix()
        old = path.read_bytes().decode("utf-8")
        source, _ = artifact_source(posix, old)
        if source.startswith(SKIPPED_MATH_SOURCE_PREFIX) or source in SKIPPED_MATH_SOURCES:
            continue
        new = normalize_path(posix, old)
        if path.suffix == ".snap":
            for char, location in unknown_snapshot_symbols(posix, new, font_charset).items():
                unknown.setdefault(char, location)
        if normalize_path(posix, new) != new:
            raise ValueError(f"terminal-chrome transform is not stable: {posix}")
        if new != old:
            changes[path] = new.encode("utf-8")
    if unknown:
        findings = [
            f"{unknown[char][0]}:{unknown[char][1]} U+{ord(char):04X} {unicodedata.name(char, 'UNNAMED')}"
            for char in sorted(unknown)[:16]
        ]
        raise SystemExit(
            f"UnknownTerminalChromeSymbols count={len(unknown)}: " + "; ".join(findings)
        )
    if check and changes:
        paths = "; ".join(path.as_posix() for path in list(changes)[:16])
        raise SystemExit(f"TerminalUnicodeNormalizationRequired count={len(changes)}: {paths}")
    for path, data in changes.items():
        path.write_bytes(data)

main()
