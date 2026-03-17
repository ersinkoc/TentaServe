# Tentaserve — Brand Guidelines

> Visual identity, voice, positioning, and creative direction for Tentaserve.

**Version:** 1.0
**Author:** Ersin Koç / ECOSTACK TECHNOLOGY OÜ

---

## Table of Contents

1. [Brand Story](#1-brand-story)
2. [Name & Etymology](#2-name--etymology)
3. [Taglines & Positioning](#3-taglines--positioning)
4. [Logo System](#4-logo-system)
5. [Color Palette](#5-color-palette)
6. [Typography](#6-typography)
7. [Mascot — Tenta](#7-mascot--tenta)
8. [Iconography](#8-iconography)
9. [Voice & Tone](#9-voice--tone)
10. [Social Media & Community](#10-social-media--community)
11. [README Badge & Shield](#11-readme-badge--shield)
12. [Presentation Templates](#12-presentation-templates)
13. [Naming Conventions (Technical)](#13-naming-conventions-technical)
14. [Do's and Don'ts](#14-dos-and-donts)

---

## 1. Brand Story

### The Metaphor

An octopus is nature's most versatile connector. Eight arms, each operating independently yet coordinated by a distributed nervous system. It reaches into crevices, adapts its shape to any surface, and communicates through color.

Tentaserve is that octopus for your API infrastructure. Its tentacles reach into REST APIs, GraphQL endpoints, and AI agent protocols — connecting them all through a single, adaptive body. It doesn't force your APIs to change shape. It wraps around them, translating their language, protecting them, and making them accessible to every client that needs them.

### The Origin

Born from the frustration of maintaining three separate tools for API gateway, protocol translation, and AI agent integration. Tentaserve collapses that complexity into a single Go binary — zero dependencies, infinite reach.

### The Promise

**"Your APIs, every protocol, one binary."**

Tentaserve doesn't ask you to rewrite your backend. It sits in front, extends its arms, and makes everything work together — REST clients, GraphQL consumers, and AI agents — without anyone knowing there's an octopus in the middle.

---

## 2. Name & Etymology

### Tentaserve

| Component | Meaning |
|-----------|---------|
| **Tenta-** | From "tentacle" — the reaching, adaptive arms of an octopus |
| **-serve** | To serve traffic, to serve APIs, to be a server |

**Pronunciation:** /ˈtentə.sɜːrv/ — "TEN-tuh-surv"

**Stylization:**
- Full: **Tentaserve** (capital T, rest lowercase — always)
- Never: TentaServe, TENTASERVE, tentaServe, Tenta-serve
- Code/CLI: `tentaserve` (all lowercase)
- Short reference in docs: "Tenta" (informal, in community contexts only)

### Domain Strategy

| Priority | Domain | Purpose |
|----------|--------|---------|
| Primary | tentaserve.dev | Main website, docs |
| Alternate | tentaserve.io | Redirect to .dev |
| GitHub | github.com/ersinkoc/tentaserve | Source code |
| npm (if needed) | @tentaserve/* | Future JS SDK |
| Go module | github.com/ersinkoc/tentaserve | Go import path |

---

## 3. Taglines & Positioning

### Primary Tagline

> **"Every protocol. One binary."**

Short, punchy, captures both the bi-directional translation and the zero-dep single binary story.

### Secondary Taglines (contextual use)

| Context | Tagline |
|---------|---------|
| GitHub description | Bi-directional GraphQL↔REST gateway with built-in MCP server. Zero dependencies. |
| Technical audience | The API gateway that speaks GraphQL, REST, and MCP — in a single Go binary. |
| AI/MCP focus | Give your AI agents access to every API. Automatically. |
| DevOps focus | One binary. No Docker required. No dependencies. Just deploy. |
| Migration pitch | Your REST APIs are now GraphQL. Your GraphQL APIs are now REST. No code changes. |
| Twitter/X one-liner | Tentaserve: an octopus that translates your APIs so your clients don't have to. 🐙 |

### Elevator Pitch (30 seconds)

"Tentaserve is a self-hosted API gateway written in Go that automatically translates between REST and GraphQL in both directions. Point it at your REST API's OpenAPI spec, and your frontend team gets a GraphQL endpoint. Point it at your GraphQL schema, and your mobile team gets REST. It also exposes every endpoint as an MCP tool, so AI agents like Claude can discover and use your APIs without manual configuration. Single binary, zero dependencies, works everywhere."

### Positioning Statement

**For** platform engineers and backend teams **who** manage mixed REST/GraphQL architectures, **Tentaserve** is an API gateway **that** translates between protocols automatically and exposes APIs to AI agents. **Unlike** Kong, Apollo Gateway, or Hasura, **Tentaserve** handles both directions in a single zero-dependency binary with native MCP support.

---

## 4. Logo System

### Concept

The logo combines a minimal octopus silhouette with a connection/flow motif:

#### Primary Mark — "The Octopus Circuit"

```
Description:
- A stylized octopus head (simple dome shape) at center
- Three tentacle lines extending outward:
  - Left tentacle: ends at a "REST" node
  - Right tentacle: ends at a "GQL" node  
  - Bottom tentacle: ends at an "MCP" node
- Tentacles use smooth curves (bezier), not rigid lines
- The head contains two simple dots for eyes
- Overall shape suggests a circuit/network diagram made organic
```

#### Simplified Mark — "The Dome"

```
Description:
- Just the octopus head dome (half-circle with two dot eyes)
- Used at small sizes (favicon, app icon, social avatar)
- Works at 16x16px
```

#### Wordmark

```
tentaserve
└── "tenta" in brand weight (600)
└── "serve" in regular weight (400)
└── Monospace or geometric sans-serif font
```

### Logo Variations

| Variant | Use Case |
|---------|----------|
| Full logo (mark + wordmark) | Website header, README, presentations |
| Mark only | Favicon, social avatar, app icon, badge |
| Wordmark only | CLI output banner, footer, inline references |
| Monochrome (white) | Dark backgrounds, terminal splash |
| Monochrome (dark) | Light backgrounds, printed materials |

### Logo Clear Space

Minimum clear space around the logo = height of the octopus dome on all sides. No other elements should intrude into this space.

### Logo Minimum Size

- Full logo: minimum 120px wide
- Mark only: minimum 24px
- Wordmark: minimum 80px wide

---

## 5. Color Palette

### Primary Colors

| Name | Hex | RGB | Usage |
|------|-----|-----|-------|
| **Tentacle Purple** | `#6C5CE7` | 108, 92, 231 | Primary brand color, logo, CTA buttons, links |
| **Deep Ocean** | `#1B1464` | 27, 20, 100 | Dark backgrounds, headers, code blocks |
| **Ink Black** | `#0D0B21` | 13, 11, 33 | Darkest background, text on light |

### Secondary Colors

| Name | Hex | RGB | Usage |
|------|-----|-----|-------|
| **Coral Reef** | `#FF6B6B` | 255, 107, 107 | REST endpoints, error states, highlights |
| **Seafoam Teal** | `#00D2D3` | 0, 210, 211 | GraphQL endpoints, success states |
| **Amber Signal** | `#FECA57` | 254, 202, 87 | MCP/AI features, warnings, badges |

### Protocol Color Mapping

| Protocol | Color | Meaning |
|----------|-------|---------|
| REST | Coral Reef `#FF6B6B` | Warm, established, the "old reliable" |
| GraphQL | Seafoam Teal `#00D2D3` | Cool, modern, the "new wave" |
| MCP | Amber Signal `#FECA57` | Bright, AI-native, the "future" |
| Tentaserve Core | Tentacle Purple `#6C5CE7` | The connector, the translator |

### Neutral Palette

| Name | Hex | Usage |
|------|-----|-------|
| White | `#FFFFFF` | Backgrounds, text on dark |
| Ghost | `#F8F9FE` | Light backgrounds, cards |
| Mist | `#E8E6F0` | Borders, dividers |
| Slate | `#8B87A3` | Secondary text, captions |
| Charcoal | `#2D2B3A` | Primary text on light backgrounds |

### Gradients

| Name | Definition | Usage |
|------|-----------|-------|
| Ocean Depth | `#6C5CE7` → `#1B1464` | Hero sections, primary backgrounds |
| Tentacle Flow | `#6C5CE7` → `#00D2D3` | Connection lines, flow diagrams |
| Sunset Reef | `#FF6B6B` → `#FECA57` | Accent highlights, call-to-action |

### Dark Mode

In dark mode, swap:
- Background: `#0D0B21` (Ink Black)
- Card surfaces: `#1B1464` (Deep Ocean)
- Text: `#FFFFFF` / `#E8E6F0`
- Primary stays `#6C5CE7`
- Secondary colors lighten 10% for contrast

---

## 6. Typography

### Primary Typeface

**Inter** — for all UI, web, and documentation.

| Weight | Usage |
|--------|-------|
| 400 Regular | Body text, paragraphs |
| 500 Medium | Subheadings, emphasis |
| 600 Semi-Bold | Headings, navigation |
| 700 Bold | Hero text, primary headings |

### Monospace Typeface

**JetBrains Mono** — for all code samples, CLI output, terminal references.

| Weight | Usage |
|--------|-------|
| 400 Regular | Code blocks, inline code |
| 700 Bold | Highlighted code, CLI commands |

### Type Scale

| Element | Size | Weight | Line Height |
|---------|------|--------|-------------|
| Hero headline | 48px / 3rem | 700 | 1.1 |
| Section heading (h1) | 32px / 2rem | 600 | 1.2 |
| Subsection (h2) | 24px / 1.5rem | 600 | 1.3 |
| Card heading (h3) | 20px / 1.25rem | 500 | 1.4 |
| Body text | 16px / 1rem | 400 | 1.6 |
| Small text / captions | 14px / 0.875rem | 400 | 1.5 |
| Code blocks | 14px / 0.875rem | 400 (Mono) | 1.5 |
| Badges / labels | 12px / 0.75rem | 500 | 1 |

---

## 7. Mascot — Tenta

### Character Description

**Tenta** is a friendly, minimal octopus character that represents the Tentaserve brand in informal and community contexts.

**Visual style:**
- Geometric, not realistic — built from simple shapes (dome + circles + curves)
- Two large, expressive dot eyes (no mouth — expression comes from eye positioning)
- Three visible tentacles (not eight — keeps it clean and maps to REST/GraphQL/MCP)
- Tentacle Purple body with slightly darker shading on the dome top
- Compact proportions — fits in a square canvas

### Personality Traits

| Trait | Expression |
|-------|-----------|
| **Helpful** | Always reaching toward something, connecting things |
| **Calm** | No frantic energy — steady, reliable presence |
| **Clever** | Slight head tilt in playful contexts |
| **Adaptable** | Changes color subtly based on context (purple for general, teal for GraphQL, coral for REST) |

### Tenta Poses (for illustrations)

| Pose | Use Case |
|------|----------|
| **Neutral** | Default, used in README, favicon |
| **Reaching** | Tentacles extending to connect two endpoints — used in architecture diagrams |
| **Waving** | Single tentacle raised — used in welcome/onboarding |
| **Holding** | Tentacle wrapped around a package/binary — used for download/install pages |
| **Thinking** | Eyes looking up-left — used for docs/learning content |
| **Celebrating** | Eyes squinted happy — used for release announcements |
| **Sleeping** | Eyes closed — used for maintenance/downtime pages |

### Mascot Usage Rules

- Tenta is never used as the primary logo in professional contexts — it supplements the logo
- Tenta always faces right (default) or toward the content it's referencing
- Tenta never appears stressed, angry, or overwhelmed — the brand is about making things easy
- Tenta can hold or wrap tentacles around icons (REST icon, GraphQL diamond, MCP connector)
- Tenta is always Tentacle Purple as base color, never a different base color

---

## 8. Iconography

### Protocol Icons

Each protocol gets a distinctive icon used consistently across docs, diagrams, and UI:

| Protocol | Icon Concept | Shape |
|----------|-------------|-------|
| REST | Rounded rectangle with "{ }" | Suggests JSON/object — Coral Reef color |
| GraphQL | Diamond/hexagon | Suggests the GraphQL logo shape — Seafoam Teal |
| MCP | Circle with a plug/connector | Suggests Model Context Protocol tool interface — Amber Signal |
| Tentaserve | Octopus dome | The connector in the middle — Tentacle Purple |

### Feature Icons

| Feature | Icon Concept |
|---------|-------------|
| Gateway | Shield outline |
| Cache | Stacked layers |
| Rate Limit | Speedometer/gauge |
| Circuit Breaker | Lightning bolt in circle |
| Auth | Lock/key |
| Health | Heartbeat pulse |
| Metrics | Bar chart |
| Config | Gear/cog |
| Plugin | Puzzle piece |
| Binary | Package/box |

### Icon Style Rules

- Stroke-based, not filled (2px stroke, rounded caps)
- 24x24px base size, scalable
- Single color per icon (from palette)
- No drop shadows, no gradients, no 3D effects
- Consistent corner radius (2px)

---

## 9. Voice & Tone

### Brand Voice

Tentaserve speaks like a **senior engineer who's also a good teacher** — technically precise, but never condescending. Confident without being arrogant. Direct without being cold.

### Voice Attributes

| Attribute | Description | Example |
|-----------|-------------|---------|
| **Clear** | No jargon without explanation, no ambiguity | "Tentaserve converts your OpenAPI spec into a GraphQL schema at startup." — not "Tentaserve leverages schema transformation paradigms." |
| **Confident** | States capabilities directly, doesn't hedge unnecessarily | "Tentaserve handles this." — not "Tentaserve might be able to help with this." |
| **Pragmatic** | Focuses on what users can do, not on internal architecture | "Add your upstream in the config file and restart." — not "The dynamic upstream registration subsystem processes YAML-defined endpoints." |
| **Honest** | Acknowledges limitations directly | "Tentaserve doesn't replace a full API management platform. It's an infrastructure component." |
| **Warm** | Friendly without being unprofessional | "Welcome aboard. Let's connect some APIs." |

### Tone by Context

| Context | Tone |
|---------|------|
| README | Friendly, concise, gets to the point fast. Lead with the one-liner, show install in 3 steps. |
| Documentation | Clear, structured, example-heavy. Every concept gets a code sample. |
| Error messages | Helpful, actionable. Always suggest what to do next. Never just "error occurred." |
| Release notes | Celebratory but factual. Lead with the headline feature, list changes, thank contributors. |
| Twitter/X | Casual, clever, slightly playful. Octopus puns welcome but not forced. |
| GitHub Issues | Patient, thorough. Reproduce the problem, explain the fix, link to relevant docs. |
| Conference talks | Storytelling + demo. Start with the problem, show the before/after, live demo the translation. |

### Writing Rules

1. **Active voice always.** "Tentaserve translates the request" — not "The request is translated by Tentaserve."
2. **Second person for docs.** "You can configure..." — not "Users can configure..." or "One can configure..."
3. **Present tense.** "Tentaserve caches the response" — not "Tentaserve will cache the response."
4. **Short sentences.** Max 25 words per sentence in docs. If you need a semicolon, make it two sentences.
5. **Code over prose.** If a concept can be shown in 5 lines of YAML, show the YAML first, explain after.
6. **No marketing superlatives.** No "revolutionary", "game-changing", "best-in-class". Let the feature speak.

---

## 10. Social Media & Community

### GitHub

- **Repository description:** "Bi-directional GraphQL↔REST API gateway with MCP server. Zero dependencies. Single Go binary."
- **Topics:** `api-gateway`, `graphql`, `rest`, `mcp`, `go`, `proxy`, `protocol-translation`, `zero-dependency`, `ai-agent`
- **Social preview image:** Tenta mascot (reaching pose) + "tentaserve" wordmark + tagline on Deep Ocean background

### X (Twitter)

- **Handle:** @tentaserve (check availability)
- **Bio:** "🐙 Every protocol. One binary. Bi-directional GraphQL↔REST gateway with MCP server. Zero deps. Built in Go. Open source."
- **Pinned tweet format:** Problem → Solution → Demo GIF → Link

### Content Pillars for X

| Pillar | % of content | Example posts |
|--------|-------------|---------------|
| Product updates | 30% | New feature announcements, release notes, benchmark results |
| Technical education | 30% | "How Tentaserve translates REST pagination to Relay cursors", "Inside the DataLoader" |
| Developer pain points | 20% | "Maintaining a separate BFF for each frontend? There's a better way." |
| Community & ecosystem | 10% | User showcases, integration examples, contributor highlights |
| Behind the scenes | 10% | Build decisions, zero-dep challenges, architecture deep dives |

### Hashtag Strategy

Primary: `#tentaserve`
Secondary: `#graphql`, `#restapi`, `#golang`, `#apigateway`, `#mcp`, `#opensource`, `#devtools`

### Community Spaces

| Platform | Purpose |
|----------|---------|
| GitHub Discussions | Feature requests, Q&A, showcases |
| Discord (future) | Real-time community chat |
| Dev.to / Hashnode | Long-form technical articles |

---

## 11. README Badge & Shield

### Shields.io Badges

```markdown
![Go Version](https://img.shields.io/badge/go-%3E%3D1.22-blue?logo=go)
![License](https://img.shields.io/badge/license-MIT-green)
![Zero Dependencies](https://img.shields.io/badge/dependencies-zero-purple)
![Build](https://img.shields.io/github/actions/workflow/status/ersinkoc/tentaserve/ci.yml?branch=main)
![Release](https://img.shields.io/github/v/release/ersinkoc/tentaserve)
```

### Custom Badge

A custom "Powered by Tentaserve" badge for projects using Tentaserve:

```markdown
![Powered by Tentaserve](https://img.shields.io/badge/powered%20by-tentaserve%20🐙-6C5CE7)
```

---

## 12. Presentation Templates

### Slide Deck Color Scheme

| Element | Color |
|---------|-------|
| Background | Deep Ocean `#1B1464` or White `#FFFFFF` |
| Title text | White (on dark) or Charcoal (on light) |
| Accent | Tentacle Purple `#6C5CE7` |
| Code blocks | Ink Black `#0D0B21` with syntax highlighting |
| Diagrams | Protocol colors (Coral/Teal/Amber) on neutral background |

### Slide Templates

1. **Title slide:** Tenta mascot (small, corner) + "Tentaserve" wordmark + tagline + speaker info
2. **Problem slide:** Pain point illustration, red/orange accent
3. **Solution slide:** Architecture diagram with protocol colors
4. **Demo slide:** Terminal screenshot or live demo, minimal chrome
5. **Code slide:** Full-width code block, dark background, large font
6. **Comparison slide:** Feature table, Tentaserve column highlighted in purple
7. **Closing slide:** Tenta (celebrating pose) + GitHub URL + "Star us! ⭐"

---

## 13. Naming Conventions (Technical)

### Config Keys

- snake_case: `rate_limit`, `batch_window`, `max_entries`
- Nested with dots in env vars: `TENTASERVE_GATEWAY_AUTH_STRATEGY`

### CLI Flags

- kebab-case: `--log-level`, `--config`, `--log-format`
- Short flags: single letter, only for most common (`-c` for config, `-p` for port)

### HTTP Headers

- `X-Request-ID` — request tracing
- `X-Tentaserve-Upstream` — which upstream handled the request (debug mode only)
- `X-Tentaserve-Cache` — cache status: `HIT`, `MISS`, `STALE`, `BYPASS`
- `X-Tentaserve-Version` — server version (optional, debug mode)

### Metrics Prefix

All Prometheus metrics: `tentaserve_*`

### MCP Tool Names

- snake_case: `get_user_by_id`, `create_order`, `search_products`
- Prefixed with upstream name only on collision: `users_api_get_user`

### Log Fields

- snake_case: `request_id`, `upstream_name`, `duration_ms`, `status_code`

---

## 14. Do's and Don'ts

### Do

- ✅ Use "Tentaserve" (capital T) in prose, `tentaserve` in code
- ✅ Lead with the problem being solved, not the feature list
- ✅ Show code examples before explaining them
- ✅ Use protocol colors consistently (Coral=REST, Teal=GraphQL, Amber=MCP)
- ✅ Use Tenta mascot in informal/community contexts
- ✅ Acknowledge the octopus theme with subtle humor ("Tentaserve reaches into your APIs")
- ✅ Compare honestly with alternatives, noting where they're better
- ✅ Celebrate the zero-dependency story — it's a key differentiator
- ✅ Use "bi-directional" (with hyphen) consistently
- ✅ Keep the brand feeling technical and trustworthy

### Don't

- ❌ Never write "TentaServe", "TENTASERVE", or "Tenta Serve"
- ❌ Never use the octopus theme in ways that feel creepy or invasive ("Tentaserve wraps around your data" — nope)
- ❌ Never claim Tentaserve replaces Kong/Envoy/Apollo entirely — it solves a specific problem
- ❌ Never use more than 3 colors in a single diagram or illustration
- ❌ Never use Tenta mascot in formal/enterprise contexts (logo only)
- ❌ Never show Tenta stressed, overwhelmed, or negative
- ❌ Never use gradients on the logo mark (solid colors only)
- ❌ Never use marketing buzzwords: "revolutionary", "disruptive", "next-generation", "enterprise-grade"
- ❌ Never abbreviate to "TS" (conflicts with TypeScript)
- ❌ Never use octopus/tentacle imagery that could be interpreted as aggressive or threatening

---

## Appendix: AI Image Generation Prompts

### Logo Generation

```
Minimalist geometric octopus logo, simple dome head with two dot eyes, 
three smooth curved tentacles extending outward, one left one right one down, 
tentacles end at small geometric nodes, flat design, no gradients, 
purple (#6C5CE7) on transparent background, vector style, 
clean lines, modern tech branding, suitable for favicon at 16px
```

### Mascot — Tenta (Neutral Pose)

```
Cute minimal geometric octopus character, simple dome-shaped head, 
two large friendly dot eyes, three visible tentacles in relaxed pose, 
solid purple (#6C5CE7) body with slightly darker dome top, 
compact square proportions, flat illustration style, 
no mouth, expression through eye positioning, 
white/transparent background, tech mascot aesthetic, 
similar to GitHub Octocat simplicity level
```

### Social Preview Card

```
Dark background (#1B1464), centered minimal octopus logo in purple (#6C5CE7), 
below it "tentaserve" in clean sans-serif font white text, 
below that "Every protocol. One binary." in smaller gray text, 
three small icons at bottom: REST (coral), GraphQL (teal), MCP (amber), 
connected by thin dotted lines to the octopus, 
clean modern developer tool aesthetic, 16:9 aspect ratio
```

### Architecture Diagram Style

```
Clean technical diagram on white background, 
boxes with rounded corners connected by smooth curved lines, 
color coded: purple for Tentaserve core, coral (#FF6B6B) for REST, 
teal (#00D2D3) for GraphQL, amber (#FECA57) for MCP, 
minimal text labels in sans-serif font, 
flat design no shadows no gradients, 
similar to Stripe/Vercel documentation diagram style
```

---

*This brand guide ensures Tentaserve presents a consistent, professional, and memorable identity across all touchpoints. The octopus theme is a strength — use it with confidence and subtlety.*
