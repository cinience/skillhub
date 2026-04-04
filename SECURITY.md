# Security Policy

## Reporting a Vulnerability

If you discover a security vulnerability in SkillHub, please report it responsibly.

**Do NOT open a public GitHub issue for security vulnerabilities.**

Instead, please email: **security@skillhub.dev** (or open a private security advisory on GitHub).

### What to include

- Description of the vulnerability
- Steps to reproduce
- Potential impact
- Suggested fix (if any)

### Response timeline

- **Acknowledgment**: within 48 hours
- **Initial assessment**: within 1 week
- **Fix and disclosure**: coordinated with the reporter

## Supported Versions

| Version | Supported |
|---------|-----------|
| Latest  | Yes       |

## Security Best Practices

When deploying SkillHub in production:

- Change the default admin password immediately
- Use HTTPS (reverse proxy with TLS)
- Set strong `SKILLHUB_ADMIN_PASSWORD` via environment variables
- Restrict network access to the admin API endpoints
- Rotate API tokens periodically
