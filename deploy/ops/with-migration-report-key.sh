#!/bin/sh
set -eu

fail() {
  printf 'migration report key validation failed: %s\n' "$*" >&2
  exit 1
}

[ "$#" -ge 2 ] || fail 'usage: with-migration-report-key.sh KEY_FILE COMMAND [ARG...]'
key_file=$1
shift

[ ! -L "$key_file" ] || fail 'key file must not be a symlink'
[ -f "$key_file" ] || fail 'key file is not a regular file'
[ -r "$key_file" ] || fail 'key file is not readable'

if mode=$(stat -c '%a' "$key_file" 2>/dev/null); then
  :
elif mode=$(stat -f '%Lp' "$key_file" 2>/dev/null); then
  :
else
  fail 'cannot read key file mode'
fi
[ "$mode" = 600 ] || fail 'key file mode must be 0600'
if link_count=$(stat -c '%h' "$key_file" 2>/dev/null); then
  :
elif link_count=$(stat -f '%l' "$key_file" 2>/dev/null); then
  :
else
  fail 'cannot read key file hardlink count'
fi
[ "$link_count" = 1 ] || fail 'key file must have exactly one hard link'
if owner=$(stat -c '%u' "$key_file" 2>/dev/null); then
  :
elif owner=$(stat -f '%u' "$key_file" 2>/dev/null); then
  :
else
  fail 'cannot read key file owner'
fi
[ "$owner" = "$(id -u)" ] || fail 'key file must be owned by the offline command user'

LC_ALL=C
export LC_ALL
byte_count=$(wc -c <"$key_file" | tr -d '[:space:]')
[ "$byte_count" = 65 ] || fail 'key file must contain exactly 64 hexadecimal characters and one newline'
key=$(cat "$key_file") || fail 'cannot read key file'
[ "${#key}" -eq 64 ] || fail 'decoded key length must be 64 characters'
case "$key" in
  *[!0-9a-f]*) fail 'key must use lowercase hexadecimal characters only' ;;
esac

EXAPI_MIGRATION_REPORT_KEY=$key
export EXAPI_MIGRATION_REPORT_KEY
unset key
exec "$@"
