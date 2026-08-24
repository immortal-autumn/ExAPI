#!/bin/sh
set -e

# The production image and Compose service must never start as root. Bind-mount
# ownership is an operator preflight because recursively changing a live data
# tree at every restart is both slow and unsafe.
if [ "$(id -u)" = "0" ]; then
    printf '%s\n' 'refusing to start ExAPI as root; use UID/GID 1000 and prepare /app/data ownership' >&2
    exit 1
fi

runtime_data_dir=${DATA_DIR:-/app/data}
[ -d "$runtime_data_dir" ] || {
    printf '%s\n' "$runtime_data_dir is missing" >&2
    exit 1
}
[ -w "$runtime_data_dir" ] || {
    printf '%s\n' "$runtime_data_dir is not writable by the ExAPI runtime user (UID/GID 1000)" >&2
    exit 1
}

# Compatibility: if the first arg looks like a flag (e.g. --help),
# prepend the default binary so it behaves the same as the old
# ENTRYPOINT ["/app/sub2api"] style.
if [ "${1#-}" != "$1" ]; then
    set -- /app/sub2api "$@"
fi

exec "$@"
