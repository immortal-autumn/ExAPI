#!/usr/bin/env bash
set -euo pipefail
umask 077

# Install only the workstation notification monitor.  This script does not
# build, restart, or otherwise modify the production ExAPI containers.
repo_root=$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)
source_file="$repo_root/deploy/ops/exapi_readiness_monitor.py"
template_dir="$repo_root/deploy/ops/offhost-monitor"
install_root="${HOME}/.local/lib/exapi-monitor"
unit_root="${XDG_CONFIG_HOME:-${HOME}/.config}/systemd/user"
backup_parent="$repo_root/tmp/offhost-monitor-backup"

usage() {
  cat <<'EOF'
Usage: install-exapi-readiness-monitor.sh [--activate]

Installs the reviewed Python monitor and user units.  --activate performs a
user-systemd daemon-reload and restarts/enables the timer after installation.
Without --activate, files are staged only; this is useful for review.
EOF
}

activate=false
if [[ $# -gt 1 ]]; then usage >&2; exit 2; fi
if [[ ${1:-} == --activate ]]; then activate=true
elif [[ $# -eq 1 ]]; then usage >&2; exit 2
fi

[[ -r "$source_file" ]] || { printf 'missing monitor source: %s\n' "$source_file" >&2; exit 1; }
[[ -r "$template_dir/exapi-readiness-monitor.service" && -r "$template_dir/exapi-readiness-monitor.timer" ]] || {
  printf 'missing systemd templates\n' >&2
  exit 1
}

mkdir -p "$backup_parent" "$install_root" "$unit_root"
backup_root=$(mktemp -d "$backup_parent/$(date -u +%Y%m%dT%H%M%S).XXXXXX")

if [[ "$activate" == true ]]; then
  # Prevent the old shell monitor from writing the shared state while files
  # are being replaced.  A failed install leaves the timer stopped and is
  # therefore obvious to the operator rather than silently running mixed code.
  systemctl --user stop exapi-readiness-monitor.timer 2>/dev/null || true
  systemctl --user stop exapi-readiness-monitor.service 2>/dev/null || true
fi
for path in \
  "$install_root/exapi_readiness_monitor.py" \
  "$install_root/exapi-readiness-monitor.sh" \
  "$unit_root/exapi-readiness-monitor.service" \
  "$unit_root/exapi-readiness-monitor.timer"; do
  if [[ -e "$path" ]]; then
    cp -a -- "$path" "$backup_root/"
  fi
done

install -m 0755 "$source_file" "$install_root/exapi_readiness_monitor.py"
install -m 0644 "$template_dir/exapi-readiness-monitor.service" "$unit_root/exapi-readiness-monitor.service"
install -m 0644 "$template_dir/exapi-readiness-monitor.timer" "$unit_root/exapi-readiness-monitor.timer"

printf 'Installed monitor and units; backup: %s\n' "$backup_root"
if [[ "$activate" == true ]]; then
  systemctl --user daemon-reload
  systemctl --user enable --now exapi-readiness-monitor.timer
  # A timer restart applies the new unit without touching the application.
  systemctl --user restart exapi-readiness-monitor.timer
  printf 'Activated exapi-readiness-monitor.timer\n'
else
  printf 'Review the files, then run this script with --activate.\n'
fi
