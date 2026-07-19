# 2026-06-11 Secret History Remediation

## Current Evidence

Non-destructive history audit found local-only files in Git history:

- `.DS_Store`
- `auth.json`
- `config.toml`
- `test.txt`
- `downloaded.txt`
- `baidupan_test.py`
- `baidu_pcs.go.tmpl`
- `config/stroing_llm.json` contained a high-confidence OpenAI-style key pattern in history.

Current-tree tracking has already been removed and guarded by:

- `.gitignore`
- `scripts/secret_scan.sh`
- `scripts/history_secret_audit.sh`

## Required Manual Remediation Before Public Push

History rewrite is intentionally not executed automatically because it is destructive for collaborators and remotes. Before publishing this repository, do the following:

1. Create a backup clone or archive of the current repository.
2. Coordinate with anyone who has a clone of this repo.
3. Rotate every token or credential that ever appeared in `auth.json`, `config.toml`, `test.txt`, Baidu token files, or LLM config JSON files. History cleanup does not revoke leaked credentials.
4. Rewrite history to remove local-only files.
5. Force-push only after confirming all collaborators know they must re-clone or hard-reset.

## Recommended Rewrite Command

Use `git filter-repo` if available:

```bash
git filter-repo \
  --path .DS_Store \
  --path auth.json \
  --path config.toml \
  --path test.txt \
  --path downloaded.txt \
  --path baidupan_test.py \
  --path baidu_pcs.go.tmpl \
  --path baiduyun_token.json \
  --path config/weak_llm.json \
  --path config/strong_llm.json \
  --path config/stroing_llm.json \
  --path config/semantic_scholar.json \
  --invert-paths
```

Then verify:

```bash
bash scripts/secret_scan.sh
bash scripts/history_secret_audit.sh
```

If `git filter-repo` is unavailable, install it via your preferred package manager or use BFG Repo-Cleaner with an equivalent path removal policy.

## Current Status

- Current working tree secret tracking: remediated.
- Current tracked secret-pattern scan: passing.
- Git history secret audit: currently expected to fail until history is rewritten.
- Credential rotation: must be performed outside the repository.
