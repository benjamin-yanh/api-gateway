# Production scripts

## Frontend commit and deployment

For frontend-only changes, load the production environment and list every file
that belongs to the intended commit:

```bash
cd /Users/benjamin/Documents/github/api-gateway
source ~/.zshrc
./scripts/commit-deploy-frontend.sh \
  -m "feat: describe the frontend change" -- \
  frontend/path/to/changed-file.tsx \
  frontend/src/i18n/locales/en.json \
  frontend/src/i18n/locales/zh.json
```

The script commits only the listed paths, preserving unrelated staged and
unstaged work. It builds the new commit in an isolated Git worktree, verifies
i18n synchronization, type-checking, lint, formatting, and the production build,
uploads a checksummed archive, activates it with rollback, and validates the HTTP
and HTTPS production endpoints.

`SSH_PASSWORD` must be present in the environment. Use the manual procedure in
`AGENT_HANDOFF.md` for backend or full-stack deployments and for diagnosing a
failed automated deployment.
