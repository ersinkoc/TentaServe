# Tentaserve

> **Every protocol. One binary.**

[![Go Version](https://img.shields.io/badge/go-%3E%3D1.22-blue?logo=go)](https://golang.org)
[![License](https://img.shields.io/badge/license-MIT-green)](LICENSE)
[![Zero Dependencies](https://img.shields.io/badge/dependencies-zero-purple)]()

Tentaserve is a self-hosted API gateway that translates between REST and GraphQL — in both directions. It also exposes every endpoint as an [MCP (Model Context Protocol)](https://modelcontextprotocol.io) tool, so AI agents like Claude can use your APIs without any manual configuration.

**Zero dependencies. Single Go binary. Works anywhere.**

---

## Features

- **Bi-directional Translation** — REST APIs become GraphQL. GraphQL APIs become REST. No code changes needed.
- **MCP Server Built-in** — Every endpoint is automatically a tool for AI agents.
- **Zero Dependencies** — Only uses Go standard library. No Docker required.
- **API Gateway** — Rate limiting, caching, circuit breaking, auth forwarding.
- **Single Binary** — One file to deploy. Cross-compiles to any platform.

---

## Quick Start

### Installation

```bash
# Download latest release
curl -L https://github.com/ersinkoc/tentaserve/releases/latest/download/tentaserve-linux-amd64 -o tentaserve
chmod +x tentaserve

# Or build from source
git clone https://github.com/ersinkoc/tentaserve.git
cd tentaserve
make build
```

### Configuration

Create `tentaserve.yaml`:

```yaml
server:
  port: 8080

upstreams:
  - name: "api"
    type: "rest"
    base_url: "https://api.example.com"
    openapi:
      source: "https://api.example.com/openapi.json"

gateway:
  rest_prefix: "/api"
  graphql_path: "/graphql"
  mcp_path: "/mcp"
```

### Run

```bash
./tentaserve serve --config tentaserve.yaml
```

Now you can:
- Query your REST API via GraphQL at `POST /graphql`
- Access your API via MCP at `POST /mcp`

---

## Example: REST → GraphQL

Your REST API has:
```
GET    /users/{id}
GET    /users/{id}/posts
POST   /users/{id}/posts
```

Tentaserve automatically generates:

```graphql
type Query {
  user(id: ID!): User
}

type User {
  id: ID!
  posts: [Post!]!
}

type Mutation {
  createUserPost(userId: ID!, input: PostInput!): Post
}
```

Query it:
```graphql
query {
  user(id: "123") {
    posts {
      title
      createdAt
    }
  }
}
```

---

## Example: GraphQL → REST

Your GraphQL API has:
```graphql
type Query {
  searchProducts(filter: ProductFilter!): [Product!]!
}
```

Tentaserve automatically exposes:
```
POST /rest/searchProducts
Content-Type: application/json

{"filter": {"category": "electronics", "maxPrice": 500}}
```

---

## Example: MCP for AI Agents

AI agents can discover and use your APIs automatically:

```json
// MCP tools/list response
{
  "tools": [
    {
      "name": "get_user_by_id",
      "description": "Get a user by their ID",
      "inputSchema": {
        "type": "object",
        "properties": {
          "id": {"type": "string"}
        },
        "required": ["id"]
      }
    },
    {
      "name": "create_user_post",
      "description": "Create a post for a user",
      "inputSchema": { ... }
    }
  ]
}
```

---

## Documentation

- [Configuration Guide](https://tentaserve.dev/docs/config)
- [REST to GraphQL](https://tentaserve.dev/docs/rest-to-graphql)
- [GraphQL to REST](https://tentaserve.dev/docs/graphql-to-rest)
- [MCP Integration](https://tentaserve.dev/docs/mcp)
- [Deployment](https://tentaserve.dev/docs/deployment)

---

## Building

```bash
# Build for current platform
make build

# Run tests
make test

# Build for all platforms
make build-all

# Build Docker image
make docker
```

---

## Why Tentaserve?

| | Tentaserve | Kong | Apollo Gateway | Hasura |
|---|:---:|:---:|:---:|:---:|
| Bi-directional translation | ✅ | ❌ | ❌ | ❌ |
| MCP support | ✅ | ❌ | ❌ | ❌ |
| Zero dependencies | ✅ | ❌ | ❌ | ❌ |
| Single binary | ✅ | ❌ | ❌ | ❌ |
| Self-hosted | ✅ | ✅ | ✅ | ✅ |

---

## License

MIT License — see [LICENSE](LICENSE) for details.

---

<p align="center">
  <sub>Built with 🐙 by <a href="https://x.com/ersinkoc">Ersin KOÇ</a></sub>
</p>
