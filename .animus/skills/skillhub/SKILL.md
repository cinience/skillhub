---
name: skillhub
description: Search, install, and publish skills on a SkillHub registry using curl
allowed-tools: Bash
user-invocable: true
argument-hint: "<search query or slug>"
arguments: [query]
keywords: [skillhub, skill, install, publish, registry]
when_to_use: When the user wants to search, install, publish, or manage skills on a SkillHub registry
---

# SkillHub Operations

Interact with a SkillHub registry to search, install, and publish skills using curl.

## Environment Variables

- `SKILLHUB_REGISTRY` — Registry URL (default: `http://localhost:10070`)
- `SKILLHUB_TOKEN` — API token for authenticated operations
- `SKILLHUB_AGENT_SECRET` — Shared secret for agent auto-provisioning

Set the base URL:
```bash
REGISTRY="${SKILLHUB_REGISTRY:-http://localhost:10070}"
```

## 1. Auto-Provisioning (Get Token)

For internal agents, automatically create a user and get an API token using a shared secret:

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
```

## 2. Search Skills

Search by keyword (use `$ARGUMENTS` as the query if provided):

```bash
curl -s "$REGISTRY/api/v1/search?q=$ARGUMENTS" | jq '.hits[] | {slug, displayName, summary}'
```

## 3. List Skills

```bash
curl -s "$REGISTRY/api/v1/skills?sort=downloads&limit=20" | jq '.data[] | {slug, displayName, downloads}'
```

Sort options: `downloads`, `created`, `updated`, `stars`.

## 4. Inspect a Skill

```bash
curl -s "$REGISTRY/api/v1/skills/SLUG" | jq .
```

Get versions:
```bash
curl -s "$REGISTRY/api/v1/skills/SLUG/versions" | jq '.data[] | {version, createdAt}'
```

Read SKILL.md content:
```bash
curl -s "$REGISTRY/api/v1/skills/SLUG/file?path=SKILL.md&version=latest"
```

## 5. Install a Skill

Download and extract to the local skills directory:

```bash
SLUG="my-skill"
VERSION="latest"
SKILLS_DIR="${HOME}/.skillhub/skills"

mkdir -p "$SKILLS_DIR/$SLUG"
curl -s -o /tmp/skill.zip "$REGISTRY/api/v1/download?slug=$SLUG&version=$VERSION"
unzip -o /tmp/skill.zip -d "$SKILLS_DIR/$SLUG"
rm /tmp/skill.zip
```

## 6. Publish a Skill

Publish a skill directory using multipart form upload. Requires authentication.

```bash
TOKEN="${SKILLHUB_TOKEN}"

# Publish from a directory — add each file as a form field
curl -s -X POST "$REGISTRY/api/v1/skills" \
  -H "Authorization: Bearer $TOKEN" \
  -F "slug=my-skill" \
  -F "version=1.0.0" \
  -F "summary=A brief description" \
  -F "tags=cli,automation" \
  -F "files=@./SKILL.md" \
  -F "files=@./script.sh"
```

The slug can also be inferred from the `name` field in SKILL.md frontmatter if `--slug` is omitted.

## 7. Other Operations

**Delete a skill** (owner only):
```bash
curl -s -X DELETE "$REGISTRY/api/v1/skills/SLUG" -H "Authorization: Bearer $TOKEN"
```

**Request public visibility**:
```bash
curl -s -X POST "$REGISTRY/api/v1/skills/SLUG/request-public" -H "Authorization: Bearer $TOKEN"
```

**Check identity**:
```bash
curl -s "$REGISTRY/api/v1/whoami" -H "Authorization: Bearer $TOKEN" | jq .
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
