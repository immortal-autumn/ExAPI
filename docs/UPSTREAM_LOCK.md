# ExAPI upstream lock

ExAPI is based on Sub2API `v0.1.171` and carries only the eleven audited
hotfixes listed in [`upstream.lock.json`](../upstream.lock.json). The lock is
part of the release contract: a release must contain every listed commit as an
ancestor, and future upstream updates require an explicit audit and lock-file
change.

The current ExAPI release built on this locked baseline is recorded in
[`PROJECT_STATUS.md`](PROJECT_STATUS.md). Do not infer the deployed ExAPI
version from the upstream tag.

Run `python3 tools/check_upstream_lock.py` from the repository root before
publishing a release. The lock deliberately excludes payment, subscription,
model-plaza, CAPTCHA, sponsor, customer-OAuth, and version-sync changes.
