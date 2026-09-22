#!/bin/bash
# Check every style cop, including strict cops, with one exact approved tagline exception.
set -euo pipefail
formula=$1
brew style --except-cops FormulaAudit/Desc "$formula"
if [ "$(basename "$formula")" = code-rules.rb ] &&
   [ "$(grep -c '^  desc ' "$formula")" = 1 ] &&
   grep -qx '  desc "The package manager for engineering best practices"' "$formula"; then
  echo 'Retaining the approved Code Rules tagline; only its description-style check is exempt.'
else
  brew style --only-cops FormulaAudit/Desc "$formula"
fi
