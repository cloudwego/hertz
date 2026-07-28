#!/bin/bash
# Copyright 2025 CloudWeGo Authors
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#   http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

#
# Hertz Hotfix Release Script
#
# This script creates hotfix releases for specific minor versions.
# It performs the following steps:
#   1. Prompts for the minor version to create a hotfix for (e.g., v0.14.x)
#   2. Finds the latest patch version for that minor version
#   3. Creates or uses existing hotfix branch from the latest patch
#   4. Shows changes in the hotfix branch
#   5. Creates and pushes a new patch version tag
#
# Usage:
#   ./release-hotfix.sh
#
# Prerequisites:
#   - Git repository must be fetched with latest changes
#   - Hotfix branch must exist (script can create it if needed)
#   - Hotfix commits must be on origin
#
# The script will interactively prompt for:
#   - Minor version to hotfix (e.g., v0.14.x)
#   - Confirmation to create hotfix branch (if it doesn't exist)
#   - Confirmation before creating the hotfix tag
#
# Example workflow:
#   1. Run script and specify v0.14.x
#   2. Script creates v0.14.x-hotfix branch from latest v0.14.* tag
#   3. Create PR with hotfix changes to the hotfix branch
#   4. Run script again to create the hotfix release
#

set -e

# Parse arguments
DRY_RUN=false
while [[ $# -gt 0 ]]; do
    case $1 in
        --dry-run)
            DRY_RUN=true
            shift
            ;;
        -h|--help)
            echo "Usage: $0 [--dry-run]"
            echo "  --dry-run    Show what would be done without making changes"
            exit 0
            ;;
        *)
            echo "Unknown option: $1"
            echo "Use --help for usage information"
            exit 1
            ;;
    esac
done

SCRIPTS_ROOT=$(git rev-parse --show-toplevel)/scripts/
CHECK_VERSION_CMD=$SCRIPTS_ROOT/.utils/check_version.sh
CHECK_GO_MOD_CMD=$SCRIPTS_ROOT/.utils/check_go_mod.sh

# Source shared utility functions
source "$SCRIPTS_ROOT/.utils/funcs.sh"

echo "🔧 Hertz Hotfix Release Script"
echo "==============================="
if [ "$DRY_RUN" = true ]; then
    echo "🧪 DRY RUN MODE - No changes will be made"
    echo "========================================="
fi

# Fetch latest changes from origin
echo "📥 Fetching latest changes from origin..."
git fetch -p --force --tags origin


# 1. Ask which minor version to fix
echo
read -p "⌨️  Enter the minor version to create hotfix for (e.g., v0.14.x): " minor_version
if [ -z "$minor_version" ]; then
    echo "❌ Error: Minor version is required"
    exit 1
fi

# Normalize minor version format
if [[ ! "$minor_version" =~ ^v[0-9]+\.[0-9]+\.x$ ]]; then
    # Try to fix common formats
    if [[ "$minor_version" =~ ^v?([0-9]+)\.([0-9]+)$ ]]; then
        minor_version="v${BASH_REMATCH[1]}.${BASH_REMATCH[2]}.x"
    elif [[ "$minor_version" =~ ^v?([0-9]+)\.([0-9]+)\.x$ ]]; then
        minor_version="v${BASH_REMATCH[1]}.${BASH_REMATCH[2]}.x"
    else
        echo "❌ Error: Invalid minor version format. Please use format like 'v0.14.x'"
        exit 1
    fi
fi

echo "🎯 Target minor version: $minor_version"

# 2. Get the latest version of the given minor version
latest_patch=$(get_latest_patch_version "$minor_version")
if [ $? -ne 0 ]; then
    echo "❌ Error: $latest_patch"
    exit 1
fi

echo "📌 Latest patch version: $latest_patch"

# 3. Check if hotfix branch exists
hotfix_branch="hotfix/${minor_version}"
if ! git show-ref --verify --quiet "refs/remotes/origin/$hotfix_branch"; then
    echo "❌ Hotfix branch 'origin/$hotfix_branch' does not exist"
    echo
    if [ "$DRY_RUN" = true ]; then
        read -p "🧪 DRY RUN: Would create hotfix branch '$hotfix_branch' from $latest_patch. Continue? (y/N): " create_branch
    else
        read -p "🔧 Create hotfix branch '$hotfix_branch' from $latest_patch? (y/N): " create_branch
    fi
    if [ "$create_branch" = "y" ] || [ "$create_branch" = "Y" ]; then
        if [ "$DRY_RUN" = true ]; then
            echo "🧪 DRY RUN: Would execute:"
            echo "  git push origin \"$latest_patch:refs/heads/$hotfix_branch\""
            echo
            echo "✅ DRY RUN: Hotfix branch '$hotfix_branch' would be created"
            echo "🔗 In real mode, you would create a PR for your hotfix changes to this branch"
            exit 0
        else
            echo "🌿 Creating hotfix branch $hotfix_branch..."
            git push origin "$latest_patch:refs/heads/$hotfix_branch"
            echo
            echo "✅ Hotfix branch '$hotfix_branch' created successfully!"
            echo "🔗 Please create a PR for your hotfix changes to this branch"
            echo "   Then run this script again to create the hotfix release"
            exit 0
        fi
    else
        echo "❌ Cannot proceed without hotfix branch"
        exit 1
    fi
fi

echo "✅ Found hotfix branch: $hotfix_branch"

# 4. Show diff between hotfix branch and latest patch version
if ! show_changes "$latest_patch" "origin/$hotfix_branch" "Changes in hotfix branch since $latest_patch"; then
    exit 0
fi

# Ask user to confirm the changes before proceeding
if ! confirm_changes; then
    exit 1
fi

# 5. Ask user if they want to release the new patch version
new_patch_version=$(increment_patch_version "$latest_patch")

echo
echo "🔖 New patch version will be: $new_patch_version"

# Check version.go file
echo
$CHECK_VERSION_CMD "$new_patch_version"

# Check go.mod
echo
$CHECK_GO_MOD_CMD

# Final confirmation
echo
if [ "$DRY_RUN" = true ]; then
    read -p "DRY RUN: Would create hotfix release tag $new_patch_version from hotfix branch $hotfix_branch. Continue? (y/N): " confirm
else
    read -p "Create hotfix release tag $new_patch_version from hotfix branch $hotfix_branch? (y/N): " confirm
fi
if [ "$confirm" != "y" ] && [ "$confirm" != "Y" ]; then
    echo "❌ Release cancelled"
    exit 1
fi

# Get the latest commit from hotfix branch
hotfix_commit=$(git rev-parse "origin/$hotfix_branch")
check_commit_exists "$hotfix_commit" "$hotfix_branch"

# Create and push tag
if [ "$DRY_RUN" = true ]; then
    echo
    echo "🧪 DRY RUN: Would execute:"
    echo "  git tag -a \"$new_patch_version\" \"$hotfix_commit\" -m \"Hotfix release $new_patch_version\""
    echo "  git push origin \"$new_patch_version\""
else
    echo "🏷️  Creating tag $new_patch_version..."
    git tag -a "$new_patch_version" "$hotfix_commit" -m "Hotfix release $new_patch_version"

    echo "📤 Pushing tag to origin..."
    git push origin "$new_patch_version"
fi

echo
if [ "$DRY_RUN" = true ]; then
    echo "🧪 DRY RUN COMPLETE - No changes were made"
    echo "Tag: $new_patch_version"
    echo "Commit: $hotfix_commit"
    echo "Based on hotfix branch: $hotfix_branch"
else
    echo "🎉 Hotfix release $new_patch_version created successfully!"
    echo "Tag: $new_patch_version"
    echo "Commit: $hotfix_commit"
    echo "Based on hotfix branch: $hotfix_branch"
fi
