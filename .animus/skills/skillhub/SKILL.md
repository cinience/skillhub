---
name: skillhub
description: Search, install, publish, and manage skills on a SkillHub registry using curl
allowed-tools: Bash
user-invocable: true
argument-hint: "<search query or slug>"
arguments: [query]
keywords: [skillhub, skill, install, publish, registry, star, rating, token, namespace, webhook]
when_to_use: When the user wants to search, install, publish, rate, star, or otherwise manage skills on a SkillHub registry
---

# SkillHub Operations

Interact with a SkillHub registry to search, install, publish, and manage skills using curl.

## Environment Variables

- `SKILLHUB_REGISTRY` — Registry URL (default: `http://localhost:10070`)
- `SKILLHUB_TOKEN` — API token for authenticated operations (Bearer token)
- `SKILLHUB_AGENT_SECRET` — Shared secret for agent auto-provisioning

Set the base URL:
```bash
REGISTRY="${SKILLHUB_REGISTRY:-http://localhost:10070}"
```

Authenticated requests use `Authorization: Bearer $SKILLHUB_TOKEN`.

## 1. Auto-Provisioning (Get Token)

Internal agents can auto-register and obtain an API token using a shared secret:

```bash
curl -s -X POST "$REGISTRY/api/v1/agent/provision" \
  -H 'Content-Type: application/json' \
  -d "{\"handle\":\"$(hostname)-agent\",\"secret\":\"$SKILLHUB_AGENT_SECRET\"}" | jq .
```

Response: `{"token":"clh_...","handle":"my-agent","userId":"uuid"}`

Save the token for subsequent requests:
```bash
TOKEN=$(curl -s -X POST "$REGISTRY/api/v1/agent/provision" \
  -H 'Content-Type: application/json' \
  -d "{\"handle\":\"$(hostname)-agent\",\"secret\":\"$SKILLHUB_AGENT_SECRET\"}" | jq -r .token)
export SKILLHUB_TOKEN="$TOKEN"
```

The endpoint returns 404 if `SKILLHUB_AGENT_SECRET` is not set server-side.

## 2. Search Skills

Search by keyword (public, approved, non-deleted only):

```bash
curl -s "$REGISTRY/api/v1/search?q=$ARGUMENTS" | jq '.hits[] | {slug, displayName, summary}'
```

Query params: `q` (required), `limit` (≤100, default 20), `offset`, `sort`.
Response: `{hits: [...], estimatedTotalHits, ...}`.

## 3. List Skills

```bash
curl -s "$REGISTRY/api/v1/skills?sort=downloads&limit=20" \
  | jq '.data[] | {slug, displayName, category, downloads}'
```

Query params: `sort` (`created`|`updated`|`downloads`|`stars`, default `created`), `limit` (≤100), `cursor`, `category`.

Filter by category:
```bash
curl -s "$REGISTRY/api/v1/skills?category=devops&sort=downloads" | jq '.data[] | {slug, displayName}'
```

Paginate with cursor:
```bash
NEXT=$(curl -s "$REGISTRY/api/v1/skills?limit=20" | jq -r .nextCursor)
curl -s "$REGISTRY/api/v1/skills?limit=20&cursor=$NEXT" | jq .
```

Categories: `devops`, `security`, `data`, `frontend`, `backend`, `infra`, `testing`, `ai`, `general`.

## 4. Inspect a Skill

```bash
curl -s "$REGISTRY/api/v1/skills/SLUG" | jq .
```

List versions:
```bash
curl -s "$REGISTRY/api/v1/skills/SLUG/versions" | jq '.versions[] | {version, createdAt}'
```

Get a specific version:
```bash
curl -s "$REGISTRY/api/v1/skills/SLUG/versions/1.0.0" | jq .
```

Read a file from a version (defaults: `version=latest`, `path=SKILL.md`):
```bash
curl -s "$REGISTRY/api/v1/skills/SLUG/file?path=SKILL.md&version=latest"
```

## 5. Install a Skill

Download as ZIP and extract to a local skills directory:

```bash
SLUG="my-skill"
VERSION="latest"
SKILLS_DIR="${HOME}/.skillhub/skills"

mkdir -p "$SKILLS_DIR/$SLUG"
curl -sfL -o /tmp/skill.zip "$REGISTRY/api/v1/download?slug=$SLUG&version=$VERSION"
unzip -o /tmp/skill.zip -d "$SKILLS_DIR/$SLUG"
rm /tmp/skill.zip
```

Resolve a fingerprint (content hash) to a version:
```bash
curl -s "$REGISTRY/api/v1/resolve?fingerprint=HEX" | jq .
```

## 6. Publish a Skill

Upload a skill using multipart form. `slug` and `version` are required (semver); `slug` is **not** auto-inferred from SKILL.md. Requires authentication.

```bash
curl -s -X POST "$REGISTRY/api/v1/skills" \
  -H "Authorization: Bearer $SKILLHUB_TOKEN" \
  -F "slug=my-skill" \
  -F "version=1.0.0" \
  -F "category=devops" \
  -F "displayName=My Skill" \
  -F "summary=A brief description" \
  -F "tags=cli,automation" \
  -F "changelog=Initial release" \
  -F "files=@./SKILL.md" \
  -F "files=@./script.sh"
```

Form fields: `slug` (required), `version` (required, semver), `category`, `displayName`, `summary`, `tags` (comma/space/semicolon separated), `changelog`, `files` (repeatable, ≤50MB total).

Version must be strictly greater than the current latest. New skills default to `visibility=private`, `moderationStatus=approved`.

## 7. Skill Lifecycle

Delete (soft delete, owner/admin only):
```bash
curl -s -X DELETE "$REGISTRY/api/v1/skills/SLUG" -H "Authorization: Bearer $SKILLHUB_TOKEN"
```

Undelete:
```bash
curl -s -X POST "$REGISTRY/api/v1/skills/SLUG/undelete" -H "Authorization: Bearer $SKILLHUB_TOKEN"
```

Request public visibility (triggers moderation review):
```bash
curl -s -X POST "$REGISTRY/api/v1/skills/SLUG/request-public" -H "Authorization: Bearer $SKILLHUB_TOKEN"
```

Check current identity:
```bash
curl -s "$REGISTRY/api/v1/whoami" -H "Authorization: Bearer $SKILLHUB_TOKEN" | jq .
```

## 8. Stars

Star / unstar a skill (authenticated):
```bash
curl -s -X POST   "$REGISTRY/api/v1/stars/SLUG" -H "Authorization: Bearer $SKILLHUB_TOKEN"
curl -s -X DELETE "$REGISTRY/api/v1/stars/SLUG" -H "Authorization: Bearer $SKILLHUB_TOKEN"
```

## 9. Ratings

Rate a skill (score 1-5):
```bash
curl -s -X POST "$REGISTRY/api/v1/skills/SLUG/ratings" \
  -H "Authorization: Bearer $SKILLHUB_TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{"score":5,"comment":"great"}'
```

List ratings (public, cursor paginated):
```bash
curl -s "$REGISTRY/api/v1/skills/SLUG/ratings?limit=20" | jq '.data[] | {score, comment, user}'
```

Delete your own rating:
```bash
curl -s -X DELETE "$REGISTRY/api/v1/skills/SLUG/ratings" -H "Authorization: Bearer $SKILLHUB_TOKEN"
```

## 10. API Tokens

List your tokens:
```bash
curl -s "$REGISTRY/api/v1/tokens" -H "Authorization: Bearer $SKILLHUB_TOKEN" | jq .
```

Create a token (scope: `full`|`read`|`publish`):
```bash
curl -s -X POST "$REGISTRY/api/v1/tokens" \
  -H "Authorization: Bearer $SKILLHUB_TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{"label":"ci-token","scope":"publish","expiresIn":"720h"}'
```

Revoke a token by ID:
```bash
curl -s -X DELETE "$REGISTRY/api/v1/tokens/TOKEN_ID" -H "Authorization: Bearer $SKILLHUB_TOKEN"
```

## 11. Namespaces (Organizations)

Create a namespace:
```bash
curl -s -X POST "$REGISTRY/api/v1/namespaces" \
  -H "Authorization: Bearer $SKILLHUB_TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{"slug":"acme","displayName":"Acme Inc","description":"...","type":"org"}'
```

List your namespaces / get / update / delete:
```bash
curl -s        "$REGISTRY/api/v1/namespaces"      -H "Authorization: Bearer $SKILLHUB_TOKEN"
curl -s        "$REGISTRY/api/v1/namespaces/acme" -H "Authorization: Bearer $SKILLHUB_TOKEN"
curl -s -X PUT "$REGISTRY/api/v1/namespaces/acme" -H "Authorization: Bearer $SKILLHUB_TOKEN" \
  -H 'Content-Type: application/json' -d '{"displayName":"Acme Corp"}'
curl -s -X DELETE "$REGISTRY/api/v1/namespaces/acme" -H "Authorization: Bearer $SKILLHUB_TOKEN"
```

Members:
```bash
curl -s "$REGISTRY/api/v1/namespaces/acme/members" -H "Authorization: Bearer $SKILLHUB_TOKEN"
curl -s -X POST "$REGISTRY/api/v1/namespaces/acme/members" \
  -H "Authorization: Bearer $SKILLHUB_TOKEN" -H 'Content-Type: application/json' \
  -d '{"handle":"alice","role":"member"}'
curl -s -X DELETE "$REGISTRY/api/v1/namespaces/acme/members/alice" \
  -H "Authorization: Bearer $SKILLHUB_TOKEN"
```

## 12. Notifications

```bash
curl -s "$REGISTRY/api/v1/notifications?limit=20"       -H "Authorization: Bearer $SKILLHUB_TOKEN" | jq .
curl -s "$REGISTRY/api/v1/notifications/unread"         -H "Authorization: Bearer $SKILLHUB_TOKEN" | jq .unread
curl -s -X POST "$REGISTRY/api/v1/notifications/NOTIF_ID/read" -H "Authorization: Bearer $SKILLHUB_TOKEN"
curl -s -X POST "$REGISTRY/api/v1/notifications/read-all"      -H "Authorization: Bearer $SKILLHUB_TOKEN"
```

## 13. Admin Operations

All endpoints under `/api/v1/admin` require `role=admin`.

Users:
```bash
curl -s "$REGISTRY/api/v1/users"                 -H "Authorization: Bearer $SKILLHUB_TOKEN"
curl -s -X POST "$REGISTRY/api/v1/users"         -H "Authorization: Bearer $SKILLHUB_TOKEN" \
  -H 'Content-Type: application/json' -d '{"handle":"bob","role":"user","email":"b@x.com"}'
curl -s -X POST "$REGISTRY/api/v1/users/role"    -H "Authorization: Bearer $SKILLHUB_TOKEN" \
  -H 'Content-Type: application/json' -d '{"userId":"UUID","role":"moderator"}'
curl -s -X POST "$REGISTRY/api/v1/users/ban"     -H "Authorization: Bearer $SKILLHUB_TOKEN" \
  -H 'Content-Type: application/json' -d '{"userId":"UUID","reason":"spam"}'
```

Skill moderation:
```bash
curl -s "$REGISTRY/api/v1/admin/skills?visibility=pending" -H "Authorization: Bearer $SKILLHUB_TOKEN"
curl -s -X POST "$REGISTRY/api/v1/admin/skills/SLUG/review" \
  -H "Authorization: Bearer $SKILLHUB_TOKEN" -H 'Content-Type: application/json' \
  -d '{"action":"approve"}'      # or "reject"
curl -s -X POST "$REGISTRY/api/v1/admin/skills/SLUG/visibility" \
  -H "Authorization: Bearer $SKILLHUB_TOKEN" -H 'Content-Type: application/json' \
  -d '{"visibility":"public"}'   # or "private"
```

Audit logs (filter by `action`, `resource_type`, `actor_id`):
```bash
curl -s "$REGISTRY/api/v1/admin/audit-logs?limit=50&action=ban_user" \
  -H "Authorization: Bearer $SKILLHUB_TOKEN" | jq .
```

Create a token for any user:
```bash
curl -s -X POST "$REGISTRY/api/v1/admin/tokens" \
  -H "Authorization: Bearer $SKILLHUB_TOKEN" -H 'Content-Type: application/json' \
  -d '{"userId":"UUID","label":"service","scope":"full"}'
```

## 14. Webhooks (Git Import)

Incoming webhook endpoints (signature-verified, configured server-side):
```
POST /api/v1/webhooks/github   # X-Hub-Signature-256
POST /api/v1/webhooks/gitlab   # X-Gitlab-Token
POST /api/v1/webhooks/gitea    # X-Gitea-Signature
```

These are called by git providers, not by agents. Enable via server config.

## 15. Health

```bash
curl -s "$REGISTRY/healthz"   # liveness
curl -s "$REGISTRY/readyz"    # readiness (DB check)
```

## SKILL.md Format

When publishing, include a `SKILL.md` file with YAML frontmatter:

```markdown
---
name: my-skill
description: What this skill does
allowed-tools: Bash
keywords: [example, demo]
---

Skill instructions go here...
```

Required fields: `name`, `description`. Optional: `allowed-tools`, `keywords`, `user-invocable`, `argument-hint`, `arguments`, `model`, `when_to_use`.

## Response Conventions

- List endpoints return `{data: [...], nextCursor: "..."}` — paginate by passing `cursor` back.
- Search returns `{hits: [...], estimatedTotalHits: N}`.
- Mutating endpoints return `{message: "..."}` or the created resource.
- Errors return `{error: "..."}` with appropriate HTTP status (400/401/403/404/500).
