#!/usr/bin/env bash
set -euo pipefail

# First-time publish script for waxon.
# Claims package names on npm and PyPI so trusted publishing can take over.
# Run this ONCE from the host (not the jail), then configure trusted publishers.

cd "$(dirname "$0")/.."

echo "=== Preparing npm package ==="
cp README.md LICENSE dist/npm/
cd dist/npm
jq '.version = "0.0.1"' package.json > tmp.json && mv tmp.json package.json
mkdir -p bin

echo ""
echo "Publishing waxon@0.0.1 to npm (placeholder)..."
echo "You may need to run 'npm login' first."
echo ""
# No --provenance flag locally — provenance only works in GitHub Actions
npm publish --access public --no-provenance

echo ""
echo "=== npm done ==="
echo ""
echo "Now configure trusted publishing:"
echo "  1. Go to https://www.npmjs.com/package/@mschulkind/waxon/access"
echo "  2. Add trusted publisher:"
echo "     - Repository: mschulkind-oss/waxon"
echo "     - Workflow:   release.yml"
echo ""

cd ../..

echo "=== Preparing PyPI package ==="
cp README.md LICENSE dist/pypi/
cd dist/pypi

echo ""
echo "Publishing waxon to PyPI (placeholder)..."
echo ""

# Set a fixed version for the placeholder (bypass setuptools_scm)
SETUPTOOLS_SCM_PRETEND_VERSION=0.0.1 uv build
uv publish dist/*

echo ""
echo "=== PyPI done ==="
echo ""
echo "Now configure trusted publishing:"
echo "  1. Go to https://pypi.org/manage/project/waxon/settings/publishing/"
echo "  2. Add trusted publisher:"
echo "     - Owner:       mschulkind-oss"
echo "     - Repository:  waxon"
echo "     - Workflow:    release.yml"
echo "     - Environment: pypi"
echo ""
echo "  3. Create 'pypi' environment in GitHub repo:"
echo "     Settings → Environments → New environment → 'pypi'"
echo ""

cd ../..

echo "=== All done ==="
echo ""
echo "Remaining steps:"
echo "  1. Add HOMEBREW_TAP_TOKEN secret to the GitHub repo"
echo "  2. Push the repo:  git push -u origin main"
echo "  3. Tag and push:   git tag v0.1.0 && git push --tags"
echo ""
echo "The tag push triggers the full release pipeline automatically."
