#!/usr/bin/env python3
"""High-signal repository secret scanner used by `make secret-scan`.

The scanner intentionally favors precision over generic entropy guesses. It
covers credential formats that should never be committed, ignores build output
and binary files, and redacts every finding. Test-only PEM fixtures are accepted
only when their decoded-looking body is clearly a short placeholder.
"""

from __future__ import annotations

import argparse
from dataclasses import dataclass
from pathlib import Path
import os
import re
import sys
from typing import Iterable

EXCLUDED_DIRS = {
    ".git",
    ".idea",
    ".vscode",
    ".pnpm-store",
    "node_modules",
    "dist",
    "build",
    "coverage",
    "tmp",
    ".tmp",
    ".cache",
    ".codex-go-cache",
    ".codex-go-mod-cache",
    ".mimocode",
    "data",
    "postgres_data",
    "redis_data",
    "__pycache__",
}
BINARY_SUFFIXES = {
    ".png", ".jpg", ".jpeg", ".gif", ".webp", ".ico", ".woff", ".woff2",
    ".ttf", ".otf", ".zip", ".gz", ".tar", ".7z", ".pdf", ".exe",
    ".dll", ".dylib", ".so", ".a", ".o", ".wasm", ".pyc",
}
MAX_FILE_BYTES = 10 * 1024 * 1024


@dataclass(frozen=True)
class Pattern:
    name: str
    regex: re.Pattern[str]


PATTERNS = (
    Pattern("github-fine-grained-token", re.compile(r"github_pat_[A-Za-z0-9_]{40,}")),
    Pattern("github-token", re.compile(r"gh[pousr]_[A-Za-z0-9]{36,}")),
    Pattern("openai-style-api-key", re.compile(r"\bsk-[A-Za-z0-9_-]{32,}\b")),
    Pattern("aws-access-key", re.compile(r"\b(?:AKIA|ASIA)[0-9A-Z]{16}\b")),
    Pattern("google-api-key", re.compile(r"\bAIza[0-9A-Za-z_-]{35}\b")),
    Pattern("slack-token", re.compile(r"\bxox[baprs]-[0-9A-Za-z-]{24,}\b")),
    Pattern("stripe-live-key", re.compile(r"\b(?:sk|rk)_live_[0-9A-Za-z]{20,}\b")),
)
PEM_RE = re.compile(
    r"-----BEGIN (?P<kind>(?:RSA |EC |OPENSSH )?PRIVATE KEY)-----"
    r"(?P<body>[\s\S]*?)"
    r"-----END (?P=kind)-----"
)


def iter_files(root: Path) -> Iterable[Path]:
    for current, dirs, files in os.walk(root):
        dirs[:] = sorted(d for d in dirs if d not in EXCLUDED_DIRS)
        base = Path(current)
        for name in sorted(files):
            path = base / name
            if path.is_symlink() or path.suffix.lower() in BINARY_SUFFIXES:
                continue
            try:
                if path.stat().st_size > MAX_FILE_BYTES:
                    continue
            except OSError:
                continue
            yield path


def is_text(data: bytes) -> bool:
    return b"\x00" not in data[:8192]


def line_number(text: str, offset: int) -> int:
    return text.count("\n", 0, offset) + 1


def redacted(value: str) -> str:
    compact = "".join(value.split())
    if len(compact) <= 12:
        return "<redacted>"
    return f"{compact[:6]}…{compact[-4:]}"


def placeholder_pem(body: str) -> bool:
    compact = re.sub(r"\s+", "", body)
    lowered = compact.lower()
    if not compact:
        return True
    if any(marker in lowered for marker in ("...", "placeholder", "example", "dummy", "test", "abc", "data")):
        return True
    # Real PEM private keys contain hundreds or thousands of base64 characters.
    return len(compact) < 256


def scan_file(path: Path, root: Path) -> list[str]:
    try:
        data = path.read_bytes()
    except OSError as exc:
        return [f"{path.relative_to(root)}: unable to read: {exc}"]
    if not is_text(data):
        return []
    text = data.decode("utf-8", errors="ignore")
    rel = path.relative_to(root).as_posix()
    findings: list[str] = []

    for pattern in PATTERNS:
        for match in pattern.regex.finditer(text):
            findings.append(
                f"{rel}:{line_number(text, match.start())}: {pattern.name}: {redacted(match.group(0))}"
            )

    for match in PEM_RE.finditer(text):
        if placeholder_pem(match.group("body")):
            continue
        findings.append(
            f"{rel}:{line_number(text, match.start())}: private-key: <redacted PEM>"
        )
    return findings


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--root", default=".")
    args = parser.parse_args()

    root = Path(args.root).resolve()
    findings: list[str] = []
    scanned = 0
    for path in iter_files(root):
        scanned += 1
        findings.extend(scan_file(path, root))

    if findings:
        print("Potential committed secrets detected:", file=sys.stderr)
        for finding in findings:
            print(f"  {finding}", file=sys.stderr)
        return 1

    print(f"secret scan passed; scanned {scanned} files")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
