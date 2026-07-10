# ExAPI

ExAPI is a private AI API gateway for routing local tools, coding agents, and applications through managed upstream AI accounts.

This fork is derived from `Wei-Shaw/sub2api`. Some internal identifiers still use `sub2api` for compatibility, while the visible product direction is ExAPI: private control plane, quota visibility, account-pool resilience, and local integration.

## Highlights

- OpenAI-compatible `/v1` gateway.
- Claude/Codex/Gemini/Antigravity-compatible routes inherited from upstream.
- Localhost/WireGuard-only private admin control plane for single-user deployments.
- Account pools, scheduled account tests, quota windows, and usage logs.
- API key management for IDEs, agents, and local automation.

## Deployment modes

### Private single-user mode

Recommended for personal infrastructure:

- expose only AI gateway routes publicly;
- keep `/admin`, `/login`, and `/api/v1/*` control APIs private;
- prioritize Accounts, API Keys, Usage, Proxies, Ops, and Settings in the UI.

### Standard multi-user mode

The inherited user/subscription/payment features remain available for controlled team deployments, but they are not the default focus of this fork.

## Compatibility

The Go module path, Docker/systemd names, database defaults, cache keys, and some documentation still intentionally reference `sub2api`. See [`docs/UPSTREAM_COMPATIBILITY.md`](docs/UPSTREAM_COMPATIBILITY.md) before attempting a full runtime rename.

## Safety notice

Using upstream consumer AI accounts through a gateway may violate provider terms of service. Review provider terms and local law before use. You are responsible for deployment, account, and data risks.
