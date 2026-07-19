#!/usr/bin/env bash
set -euo pipefail

cd "$(git rev-parse --show-toplevel)"

blocked_files_regex='^(\.DS_Store|auth\.json|config\.toml|test\.txt|downloaded\.txt|baidupan_test\.py|baidu_pcs\.go\.tmpl|baiduyun_token\.json|config/(weak_llm|strong_llm|stroing_llm|semantic_scholar)\.json)$'
blocked_files="$(git ls-files | grep -E "$blocked_files_regex" || true)"
if [[ -n "$blocked_files" ]]; then
  echo "Tracked local secret/scratch files are not allowed:" >&2
  echo "$blocked_files" >&2
  exit 1
fi

tracked_files="$(mktemp)"
trap 'rm -f "$tracked_files"' EXIT
while IFS= read -r file; do
  if [[ -f "$file" ]]; then
    printf '%s\0' "$file" >> "$tracked_files"
  fi
done < <(git ls-files)

secret_pattern_names=(
  'anthropic_key'
  'openai_project_key'
  'openai_legacy_key'
  'bearer_token'
)
secret_patterns=(
  'sk-ant-[A-Za-z0-9_-]{20,}'
  'sk-proj-[A-Za-z0-9_-]{20,}'
  'sk-[A-Za-z0-9_-]{32,}'
  'Bearer[[:space:]]+[A-Za-z0-9._~+/=-]{32,}'
)

for i in "${!secret_patterns[@]}"; do
  pattern="${secret_patterns[$i]}"
  name="${secret_pattern_names[$i]}"
  matches="$(xargs -0 grep -lIE "$pattern" < "$tracked_files" || true)"
  if [[ -n "$matches" ]]; then
    echo "Potential hard-coded secret detected by pattern: $name" >&2
    echo "Matching tracked files are listed without secret values:" >&2
    echo "$matches" >&2
    exit 1
  fi
done

echo "Secret scan passed."
