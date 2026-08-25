#!/bin/sh
set -eu

repo_dir=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
work_dir=$(mktemp -d "${TMPDIR:-/tmp}/dphcko-xsd.XXXXXX")
trap 'rm -rf "$work_dir"' EXIT HUP INT TERM

decode() {
    source_file=$1
    target_file=$2
    if base64 --decode "$source_file" >"$target_file.gz" 2>/dev/null; then
        :
    else
        base64 -D -i "$source_file" -o "$target_file.gz"
    fi
    gzip -dc "$target_file.gz" >"$target_file"
}

decode "$repo_dir/schemas/dphdp3_epo2.xsd.gz.b64" "$work_dir/dphdp3_epo2.xsd"
decode "$repo_dir/schemas/dphkh1_epo2.xsd.gz.b64" "$work_dir/dphkh1_epo2.xsd"

printf '%s  %s\n' \
    '1373ad0c719123228b8bfe75f019338ba53ce12494c80dda9fc9e3682eb4fde7' "$work_dir/dphdp3_epo2.xsd" \
    'addb0763b4d710120726751ebd2025949414842ac574982925183dfeee96f923' "$work_dir/dphkh1_epo2.xsd" |
    shasum -a 256 -c -

xmllint --noout --schema "$work_dir/dphdp3_epo2.xsd" "$repo_dir/internal/epo/testdata/dphdp3.golden.xml"
xmllint --noout --schema "$work_dir/dphkh1_epo2.xsd" "$repo_dir/internal/epo/testdata/dphkh1.golden.xml"
