#!/usr/bin/env bash
set -euo pipefail

cd "$(git rev-parse --show-toplevel)"

blocked_files_regex='^(\.DS_Store|auth\.json|config\.toml|test\.txt|downloaded\.txt|baidupan_test\.py|baidu_pcs\.go\.tmpl|baiduyun_token\.json|config/(weak_llm|strong_llm|stroing_llm|semantic_scholar)\.json)$'

secret_pattern_names=(
  'openai_project_key'
  'openai_legacy_key'
  'anthropic_key'
  'bearer_token'
)
secret_patterns=(
  'sk-proj-[A-Za-z0-9_-]{20,}'
  'sk-[A-Za-z0-9_-]{32,}'
  'sk-ant-[A-Za-z0-9_-]{20,}'
  'Bearer[[:space:]]+[A-Za-z0-9._~+/=-]{32,}'
)

tmp_findings="$(mktemp)"
trap 'rm -f "$tmp_findings"' EXIT

while IFS= read -r rev; do
  while IFS= read -r file; do
    [[ -z "$file" ]] && continue
    if [[ "$file" =~ $blocked_files_regex ]]; then
      printf 'blocked-file %s %s\n' "$rev" "$file" >> "$tmp_findings"
    fi
  done < <(git ls-tree -r --name-only "$rev")

  for i in "${!secret_patterns[@]}"; do
    pattern="${secret_patterns[$i]}"
    name="${secret_pattern_names[$i]}"
    while IFS= read -r match; do
      [[ -z "$match" ]] && continue
      file="${match#*:}"
      printf 'secret-pattern:%s %s %s\n' "$name" "$rev" "$file" >> "$tmp_findings"
    done < <(git grep -I -E -l "$pattern" "$rev" -- . 2>/dev/null || true)
  done
done < <(git rev-list --all)

if [[ -s "$tmp_findings" ]]; then
  echo "Git history still contains local secret/scratch files or high-confidence secret patterns." >&2
  echo "Findings are intentionally limited to commit and file path; secret values are not printed." >&2
  sort -u "$tmp_findings" >&2
  exit 1
fi

echo "Git history secret audit passed."
