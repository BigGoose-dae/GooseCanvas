#!/bin/sh
set -eu

if ! command -v gitleaks >/dev/null 2>&1; then
  echo 'Install the scanner: go install github.com/zricethezav/gitleaks/v8@v8.30.1' >&2
  exit 1
fi

scan_tree=$(mktemp -d)
scan_diff=$(mktemp)
trap 'rm -rf "$scan_tree"; rm -f "$scan_diff"' EXIT HUP INT TERM
python3 - "$scan_tree" <<'PYSCAN'
import os, pathlib, shutil, subprocess, sys
paths = subprocess.check_output(['git', 'ls-files', '--cached', '--others', '--exclude-standard', '-z']).decode().split('\0')
blocked = []
for path in sorted(set(filter(None, paths))):
    p = pathlib.PurePosixPath(path)
    if ((p.name == '.env' or p.name.startswith('.env.')) and p.name != '.env.example'
        or p.name == '.settings.key'
        or p.suffix in {'.db', '.sqlite', '.sqlite3', '.pem', '.key'}
        or any(marker in p.name for marker in ('.db-', '.sqlite-', '.sqlite3-'))):
        blocked.append(path)
        continue
    source = pathlib.Path(path)
    target = pathlib.Path(sys.argv[1]) / path
    if source.is_symlink():
        target.parent.mkdir(parents=True, exist_ok=True)
        target.write_text(os.readlink(source))
    elif source.is_file():
        target.parent.mkdir(parents=True, exist_ok=True)
        shutil.copyfile(source, target)
if blocked:
    print('Sensitive files must not be tracked:', *blocked, sep='\n', file=sys.stderr)
    sys.exit(1)
PYSCAN

gitleaks git --redact --no-banner --log-opts=--all .
# Capture diffs first so a failed git command cannot be hidden by a pipeline.
git diff --no-ext-diff --unified=0 > "$scan_diff"
git diff --cached --no-ext-diff --unified=0 >> "$scan_diff"
gitleaks stdin --redact --no-banner < "$scan_diff"

gitleaks dir --redact --no-banner "$scan_tree"
