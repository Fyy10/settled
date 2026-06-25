#!/bin/bash
set -e

# This script helps initialize the ideas directory for the idea-refine skill.
# Run it from the repository root. It intentionally accepts no user-provided
# path arguments and only creates the fixed relative directory below.

IDEAS_DIR="docs/ideas"

if [ ! -d "$IDEAS_DIR" ]; then
  mkdir -p "$IDEAS_DIR"
  echo "Created directory: $IDEAS_DIR" >&2
else
  echo "Directory already exists: $IDEAS_DIR" >&2
fi

echo "{\"status\": \"ready\", \"directory\": \"$IDEAS_DIR\"}"
