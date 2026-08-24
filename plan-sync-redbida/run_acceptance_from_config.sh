#!/bin/sh
set -eu

config_path=${1:-/opt/ksp-cam/config.yaml}
probe_path=${2:-/tmp/redbida_acceptance.py}

read_server_value() {
  field=$1
  awk -v wanted="$field" '
    /^server:/ { in_server = 1; next }
    in_server && /^[^ ]/ { exit }
    in_server && $1 == wanted ":" {
      value = $2
      gsub(/^["'\'' ]+|["'\'' ]+$/, "", value)
      print value
      exit
    }
  ' "$config_path"
}

user=$(read_server_value username)
password=$(read_server_value password)

if [ -z "$user" ] || [ -z "$password" ]; then
  echo "server credentials not found in $config_path" >&2
  exit 1
fi

printf '%s\n' "$password" | "$probe_path" --user "$user" --password-stdin
