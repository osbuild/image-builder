#!/bin/bash
set -euo pipefail

while IFS= read -r -d '' dir; do
    while IFS= read -r -d '' file; do
        echo "-- $file --"

        if [ -f "$dir/_shared.yaml" ] && [[ $(basename "$file") != "_shared.yaml" ]]; then
            cat "$dir/_shared.yaml" "$file" | yamllint -
            cat "$dir/_shared.yaml" "$file" | yq '.' > /dev/null
        else
            cat "$file" | yamllint -
            cat "$file" | yq '.' > /dev/null
        fi
    done < <(find "$dir" -maxdepth 1 -type f '(' -iname '*.yaml' -or -iname '*.yml' ')' -print0 2>/dev/null)
done < <(find . -type d -not -path './.git/*' -print0)
