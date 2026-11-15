#!/usr/bin/env bash
# Build Markdown release notes from Conventional Commits between two refs.
# Usage:
#   bash scripts/release-notes.sh [from-ref] [to-ref]
# Defaults: latest v*.*.* tag → HEAD.
# Env:
#   REPO — github owner/name for compare URL (default: krate-apps/fleet)
#   TAG  — new release tag for the compare URL tip (default: to-ref or HEAD)
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "${ROOT}"

REPO="${REPO:-krate-apps/fleet}"

to_ref="${2:-HEAD}"
if [[ -n "${1:-}" ]]; then
  from_ref="$1"
else
  from_ref="$(git tag -l 'v*.*.*' --sort=-v:refname | head -n1 || true)"
fi

tag_for_url="${TAG:-}"
if [[ -z "${tag_for_url}" ]]; then
  if [[ "${to_ref}" == v* ]]; then
    tag_for_url="${to_ref}"
  else
    tag_for_url="HEAD"
  fi
fi

if [[ -z "${from_ref}" ]]; then
  cat <<EOF
## What's changed

Initial release.

**Commits:** https://github.com/${REPO}/commits/${tag_for_url}

EOF
  exit 0
fi

mapfile -t lines < <(git log --no-merges --pretty=format:'%s' "${from_ref}..${to_ref}")

declare -a feats=() fixes=() docs=() perfs=() refactors=() maint=() others=()
for s in "${lines[@]+"${lines[@]}"}"; do
  [[ -z "${s}" ]] && continue
  if [[ "${s}" =~ ^(feat|fix|docs|perf|refactor|chore|build|ci|test|style|ux|i18n|deps)(\(.+\))?\!?:\ (.+)$ ]]; then
    kind="${BASH_REMATCH[1]}"
    rest="${BASH_REMATCH[3]}"
  else
    kind="other"
    rest="${s}"
  fi
  case "${kind}" in
    feat) feats+=("${rest}") ;;
    fix) fixes+=("${rest}") ;;
    docs) docs+=("${rest}") ;;
    perf) perfs+=("${rest}") ;;
    refactor) refactors+=("${rest}") ;;
    chore|build|ci|test|style|deps) maint+=("${rest}") ;;
    ux|i18n) feats+=("${rest}") ;;
    *) others+=("${rest}") ;;
  esac
done

emit_section() {
  local title="$1"
  shift
  if (($# == 0)); then
    return 0
  fi
  echo "### ${title}"
  echo
  local item
  for item in "$@"; do
    echo "- ${item}"
  done
  echo
}

echo "## What's changed"
echo

if ((${#lines[@]} == 0)); then
  echo "_No commits since \`${from_ref}\`._"
  echo
else
  emit_section "Features" "${feats[@]+"${feats[@]}"}"
  emit_section "Fixes" "${fixes[@]+"${fixes[@]}"}"
  emit_section "Documentation" "${docs[@]+"${docs[@]}"}"
  emit_section "Performance" "${perfs[@]+"${perfs[@]}"}"
  emit_section "Refactoring" "${refactors[@]+"${refactors[@]}"}"
  emit_section "Maintenance" "${maint[@]+"${maint[@]}"}"
  emit_section "Other" "${others[@]+"${others[@]}"}"
fi

echo "**Full changelog:** https://github.com/${REPO}/compare/${from_ref}...${tag_for_url}"
echo
