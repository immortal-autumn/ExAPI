# ExAPI

ExAPI is a private AI API gateway for routing local tools, coding agents, and applications through managed upstream AI accounts.

It is forked from `Wei-Shaw/sub2api` and keeps several internal `sub2api` identifiers for compatibility, but the product direction of this fork is operator-first: public AI gateway endpoints, private control-plane access, quota visibility, and multi-account resilience.

## Highlights

- OpenAI-compatible `/v1` gateway.
- Compatibility routes for Claude/Codex/Gemini/Antigravity-style clients inherited from upstream.
- Private localhost/WireGuard control plane for single-user deployments.
- Account pool management, scheduled account tests, quota/watch windows, and usage logs.
- API key management for local IDEs, agents, and automation.
- Optional multi-user/payment features retained from upstream for deployments that need them.

## Deployment modes

### Private single-user mode

Recommended for personal infrastructure:

- public domain exposes only AI gateway routes;
- admin/control UI is available only through localhost or WireGuard/VPN;
- sidebar and dashboard prioritize accounts, API keys, usage, proxies, ops, and settings;
- quota and local integration details are shown directly on the admin dashboard.

### Standard multi-user mode

ExAPI still retains the upstream SaaS-style features such as users, subscriptions, payments, groups, and redeem codes. These are useful for controlled team deployments but are not the default product emphasis of this fork.

## Architecture

- Backend: Go, Gin, Ent, PostgreSQL, Redis.
- Frontend: Vue 3, TypeScript, Pinia, Vue Router, TailwindCSS, Vite.
- Deployment: Docker/Compose or systemd, typically behind nginx/Caddy/Cloudflare.

The Go module path and many runtime artifact names still reference `sub2api` intentionally. See [`docs/UPSTREAM_COMPATIBILITY.md`](docs/UPSTREAM_COMPATIBILITY.md) before attempting a full module/service/database rename.

## Quick start

For Docker-based deployment, start from the deployment templates under [`deploy/`](deploy/). Existing upstream deployment commands that reference `sub2api` paths or service names are still expected to work unless you perform the optional full artifact migration.

For an ExAPI-branded private deployment, configure:

```env
RUN_MODE=simple
SUB2API_SINGLE_USER_PRIVATE_CONTROL_PLANE=true
SUB2API_PUBLIC_HOST=your-public-ai-gateway.example.com
```

Then expose only the AI gateway paths publicly and keep `/admin`, `/login`, and `/api/v1/*` control APIs private.

## Safety notice

Using upstream consumer AI accounts through a gateway may violate provider terms of service. Review each provider's terms, comply with local law, and operate only accounts and traffic you are authorized to use. This project is provided for technical research and self-hosted infrastructure purposes; you are responsible for deployment, account, and data risks.

## Upstream attribution

ExAPI is derived from the open-source Sub2API project by Wei-Shaw and contributors. This fork removes upstream sponsorship/referral presentation from the default README and focuses on private operator workflows. Historical compatibility notes are preserved separately in [`docs/UPSTREAM_COMPATIBILITY.md`](docs/UPSTREAM_COMPATIBILITY.md).
