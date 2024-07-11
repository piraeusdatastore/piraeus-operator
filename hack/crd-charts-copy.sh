#!/bin/sh -e

echo ' {{ if .Values.installCRDs }}'

# Find and sort the files
files=$(find ./config/crd/bases -type f | sort)

# Check if files are empty
if [ -n "$files" ]; then
    cat $files
fi

echo '{{ end }}'
