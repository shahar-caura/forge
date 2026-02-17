#!/usr/bin/env bash
set -euo pipefail

# Git worktree helper that handles git-crypt decryption.
#
# Usage: ./scripts/git-worktree-add.sh <branch> <path> <base-branch>
#
# Creates a NEW branch from base-branch in a worktree at path.
# Works in any git-crypt repo. Creates the worktree with filters bypassed,
# copies symmetric keys, then detects and re-checkouts encrypted files
# (by their GITCRYPT binary header) to force decryption.

BRANCH="${1:-}"
WORKTREE_PATH="${2:-}"
BASE_BRANCH="${3:-}"

if [[ -z "$BRANCH" || -z "$WORKTREE_PATH" || -z "$BASE_BRANCH" ]]; then
  echo "Usage: $0 <branch> <path> <base-branch>"
  exit 1
fi

# Must run from repo root
if [[ ! -d .git ]]; then
  echo "Error: run this from the repository root (where .git/ is)."
  exit 1
fi

# Worktree path must not already exist
if [[ -e "$WORKTREE_PATH" ]]; then
  echo "Error: '$WORKTREE_PATH' already exists. Remove it or choose a different path."
  exit 1
fi

echo "Creating worktree at '$WORKTREE_PATH' (new branch '$BRANCH' from '$BASE_BRANCH')..."

# 1. Create worktree with new branch, git-crypt filters bypassed
#    Files land as encrypted blobs (smudge=cat means no decryption)
if [[ -d .git/git-crypt/keys ]]; then
  git -c filter.git-crypt.smudge=cat -c filter.git-crypt.clean=cat \
    worktree add -b "$BRANCH" "$WORKTREE_PATH" "$BASE_BRANCH"

  # 2. Copy git-crypt keys into the worktree's GIT_DIR
  WORKTREE_GIT_DIR=".git/worktrees/$(basename "$WORKTREE_PATH")"
  if [[ ! -d "$WORKTREE_GIT_DIR" ]]; then
    echo "Error: worktree git dir not found at '$WORKTREE_GIT_DIR'."
    exit 1
  fi
  cp -r .git/git-crypt "$WORKTREE_GIT_DIR/git-crypt"

  # 3. Find files with the GITCRYPT header, delete them, re-checkout to decrypt
  cd "$WORKTREE_PATH"
  ENCRYPTED=()
  while IFS= read -r f; do
    if [[ -f "$f" ]] && head -c 10 "$f" | grep -q "GITCRYPT"; then
      ENCRYPTED+=("$f")
    fi
  done < <(git ls-files)

  if [[ ${#ENCRYPTED[@]} -gt 0 ]]; then
    rm -f "${ENCRYPTED[@]}"
    git checkout -- "${ENCRYPTED[@]}"
    echo "Decrypted ${#ENCRYPTED[@]} file(s)."
  else
    echo "No encrypted files found."
  fi
else
  # No git-crypt — plain worktree add
  git worktree add -b "$BRANCH" "$WORKTREE_PATH" "$BASE_BRANCH"
fi

echo "Worktree ready at: $WORKTREE_PATH"
