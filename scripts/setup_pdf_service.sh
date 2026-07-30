#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
service_dir="$repo_root/services/pdf_service"
venv_dir="$service_dir/.venv"
python_bin="${DIVEEND_BOOTSTRAP_PYTHON:-python3}"

"$python_bin" -m venv "$venv_dir"
"$venv_dir/bin/python" -m pip install --upgrade pip
"$venv_dir/bin/python" -m pip install -r "$service_dir/requirements.txt"
"$venv_dir/bin/python" -c 'import fastapi, uvicorn, pymupdf4llm, fitz, pydantic_settings'

printf 'PDF service environment ready: %s\n' "$venv_dir"
