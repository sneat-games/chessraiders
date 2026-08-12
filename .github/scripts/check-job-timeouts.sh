#!/usr/bin/env bash
# Fails if any job in any .github/workflows/*.yml has no timeout-minutes.
#
# Why this exists: the private sneat-co/chessraiders repo lost 2h17m of CI
# fleet-wide the day before this script was written (2026-08-10) to a job
# with no timeout-minutes that hung, combined with `cancel-in-progress:
# false` on that workflow — every later run queued behind the hung one with
# zero jobs started. spec/plans/publish-the-standard-bot.md Task 10 asks
# this repo to carry that lesson forward rather than repeat it.
#
# This walks the JOB STRUCTURE of each workflow file (every key indented
# exactly two spaces directly under `jobs:`), not a hardcoded job-name list,
# so a job added later is checked automatically without anyone remembering
# to update this script. It deliberately does not parse arbitrary third-
# party YAML — only the two workflow files this repository authors itself,
# in the plain, unnested job-list style GitHub Actions workflows use.
set -euo pipefail

cd "$(dirname "$0")/../.."

missing=0

for workflow in .github/workflows/*.yml; do
  while IFS= read -r line; do
    echo "MISSING ${workflow}: job \"${line}\" has no timeout-minutes"
    missing=1
  done < <(
    awk '
      # Enter the jobs: block.
      /^jobs:[[:space:]]*$/ { in_jobs = 1; next }
      # A non-indented line ends the jobs: block (next top-level key, or EOF via END).
      in_jobs && /^[^[:space:]]/ { in_jobs = 0 }

      # A job key: exactly two spaces of indent, then a bare `name:` key.
      in_jobs && /^  [A-Za-z_][A-Za-z0-9_-]*:[[:space:]]*$/ {
        if (job != "" && !has_timeout) print job
        job = $0
        sub(/^  /, "", job)
        sub(/:.*/, "", job)
        has_timeout = 0
        next
      }

      # A timeout-minutes key belonging to the current job (four-space indent).
      in_jobs && /^    timeout-minutes:/ { has_timeout = 1 }

      END { if (job != "" && !has_timeout) print job }
    ' "$workflow"
  )
done

if [ "$missing" -ne 0 ]; then
  echo
  echo "Every job in every workflow must declare timeout-minutes. A job with" \
    "no timeout can hang forever; combined with a workflow whose" \
    "concurrency group has cancel-in-progress: false, one hung job queues" \
    "every later run behind it — the private repo's 2h17m outage on" \
    "2026-08-10. Add timeout-minutes to the job(s) named above."
  exit 1
fi

echo "workflow timeout policy: every job declares timeout-minutes"
