#!/usr/bin/env bash
# GitHub Actions Ubuntu only: Bash, Go, Git, gh, jq, sha256sum, cmp, mktemp.
# Called only after every verification/package job succeeds. Never publishes.
set -euo pipefail

: "${RELEASE_TAG:?}" "${RELEASE_COMMIT:?}" "${GH_REPO:?}" "${GH_TOKEN:?}"
go run ./scripts/release source "$RELEASE_TAG" "$RELEASE_COMMIT"

# Recheck the remote tag immediately before staging, including annotated tags.
object=$(gh api "repos/$GH_REPO/git/ref/tags/$RELEASE_TAG")
kind=$(jq -er '.object.type' <<< "$object")
sha=$(jq -er '.object.sha' <<< "$object")
while [[ "$kind" == tag ]]; do
  object=$(gh api "repos/$GH_REPO/git/tags/$sha")
  kind=$(jq -er '.object.type' <<< "$object")
  sha=$(jq -er '.object.sha' <<< "$object")
done
[[ "$kind" == commit && "$sha" == "$RELEASE_COMMIT" ]]

# A successful paginated API read is required; auth/network failures cannot
# masquerade as absence. Refuse existing drafts AND published releases.
existing=$(gh api --paginate "repos/$GH_REPO/releases?per_page=100" \
  --jq ".[] | select(.tag_name == \"$RELEASE_TAG\") | .id")
if [[ -n "$existing" ]]; then
  echo "Release already exists for $RELEASE_TAG; manual review required." >&2
  exit 1
fi

scratch=$(mktemp -d)
trap 'rm -rf -- "$scratch"' EXIT
# Recompute the expected manifest from a copy without the existing manifest.
# This also rejects extra/missing files immediately before the API write.
mkdir "$scratch/assets"
cp dist/* "$scratch/assets/"
rm "$scratch/assets/SHA256SUMS.txt"
go run ./scripts/release checksums "$RELEASE_TAG" "$scratch/assets"
cmp dist/SHA256SUMS.txt "$scratch/assets/SHA256SUMS.txt"
(cd dist && sha256sum --check SHA256SUMS.txt)

prerelease=false
if [[ "${RELEASE_TAG%%+*}" == *-* ]]; then prerelease=true; fi
{
  printf 'WhichWhy %s\n\nSource commit: `%s`\n\n' "$RELEASE_TAG" "$RELEASE_COMMIT"
  cat docs/release-notes-v1.md
} > "$scratch/notes.md"

# --verify-tag prevents implicit tag creation. No edit/publish operation exists.
# A failed upload may leave an incomplete DRAFT; humans must not publish it.
gh release create "$RELEASE_TAG" dist/* --verify-tag --target "$RELEASE_COMMIT" \
  --draft --prerelease="$prerelease" --title "WhichWhy $RELEASE_TAG" \
  --notes-file "$scratch/notes.md"

release=$(gh release view "$RELEASE_TAG" --json isDraft,isPrerelease,tagName,assets)
jq -e --arg tag "$RELEASE_TAG" --argjson pre "$prerelease" \
  '.isDraft == true and .isPrerelease == $pre and .tagName == $tag and (.assets | length) == 5' \
  <<< "$release"
gh release download "$RELEASE_TAG" --dir "$scratch/download"
cmp dist/SHA256SUMS.txt "$scratch/download/SHA256SUMS.txt"
# Reject unexpected names, then verify the actual downloaded bytes.
diff <(find dist -maxdepth 1 -type f -printf '%f\n' | sort) \
  <(find "$scratch/download" -maxdepth 1 -type f -printf '%f\n' | sort)
(cd "$scratch/download" && sha256sum --check SHA256SUMS.txt)
echo "Draft verified. Publication requires Mission Control approval."
