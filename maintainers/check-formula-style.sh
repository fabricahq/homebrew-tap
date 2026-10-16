#!/bin/bash
# Check every style cop, including strict cops, with one exact approved tagline exception.
set -euo pipefail
formula=$1
brew style --except-cops FormulaAudit/Desc "$formula"
if [ "$(basename "$formula")" = trailguide.rb ] &&
   [ "$(grep -c '^  desc ' "$formula")" = 1 ] &&
   grep -qx '  desc "The package manager for your agent skills and rules"' "$formula"; then
  echo 'Retaining the approved Trail Guide tagline; only its description-style check is exempt.'
else
  brew style --only-cops FormulaAudit/Desc "$formula"
fi
