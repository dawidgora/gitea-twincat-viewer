#!/usr/bin/env bash
# Local Gitea bootstrap for gitea-twincat-viewer development.
#
# Creates the admin user, a test repository, and seeds it with the
# example TwinCAT files. This script is idempotent and can be re-run
# safely against an already-initialized data volume.
#
# Notes:
#   - Everything is driven through the Gitea HTTP API so we don't
#     need the gitea CLI inside this container.
#   - We wait for the HTTP endpoint to respond before issuing admin
#     commands, because Gitea blocks /api/v1 until install completes.

set -euo pipefail

GITEA_HOST=${GITEA_HOST:-http://127.0.0.1:3000}
ADMIN_USER=${ADMIN_USER:-gitea_admin}
ADMIN_PASS=${ADMIN_PASS:-adminpass123}
ADMIN_EMAIL=${ADMIN_EMAIL:-[email protected]}
REPO_OWNER=${REPO_OWNER:-${ADMIN_USER}}
REPO_NAME=${REPO_NAME:-twincat-demo}
EXAMPLES_DIR=${EXAMPLES_DIR:-/examples/twincat}

echo "[bootstrap] waiting for Gitea to become reachable at ${GITEA_HOST}"
for i in $(seq 1 120); do
  if curl -fsS -o /dev/null "${GITEA_HOST}/"; then
    echo "[bootstrap] Gitea is up after ${i}s"
    break
  fi
  sleep 1
done

# Wait for the API to be fully wired (install lock created means we
# can use admin endpoints).
for i in $(seq 1 60); do
  if curl -fsS -o /dev/null "${GITEA_HOST}/api/v1/version"; then
    echo "[bootstrap] Gitea API is up after ${i}s"
    break
  fi
  sleep 1
done

# Gitea requires either an interactive session or an API token for
# admin operations. We try a few common credentials and create the
# admin user if necessary, then mint a token via the admin's basic
# auth flow.

create_admin() {
  echo "[bootstrap] creating admin user ${ADMIN_USER}"
  curl -fsS -X POST "${GITEA_HOST}/api/v1/admin/users" \
    -u "${ADMIN_USER}:${ADMIN_PASS}" \
    -H "Content-Type: application/json" \
    -d "{
      \"username\": \"${ADMIN_USER}\",
      \"email\": \"${ADMIN_EMAIL}\",
      \"password\": \"${ADMIN_PASS}\",
      \"must_change_password\": false,
      \"login_name\": \"${ADMIN_USER}\"
    }" >/dev/null || echo "[bootstrap] admin may already exist; continuing"
}

# Try basic auth first (admin user already created on a prior run).
if ! curl -fsS -o /dev/null -u "${ADMIN_USER}:${ADMIN_PASS}" \
     "${GITEA_HOST}/api/v1/user"; then
  echo "[bootstrap] admin user does not exist; attempting self-registration"
  # Self-register via /api/v1/user. When DISABLE_REGISTRATION is off
  # and the install lock is set, the first user becomes an admin.
  if ! curl -fsS -X POST "${GITEA_HOST}/api/v1/user" \
       -H "Content-Type: application/json" \
       -d "{
         \"username\": \"${ADMIN_USER}\",
         \"email\": \"${ADMIN_EMAIL}\",
         \"password\": \"${ADMIN_PASS}\",
         \"must_change_password\": false
       }" >/dev/null 2>&1; then
    echo "[bootstrap] self-registration failed; skipping admin creation"
  fi
fi

# Mint a token using basic auth. Keep it in the persistent data volume so a
# recreated bootstrap container can reuse it. The recovery name is distinct
# from the original "bootstrap" token used by older runs.
TOKEN_DIR=/data/.gitea-bootstrap
TOKEN_FILE="${TOKEN_DIR}/admin-token"
TOKEN_NAME=bootstrap-recovery
mkdir -p "${TOKEN_DIR}"
chmod 700 "${TOKEN_DIR}"
if [[ -s "${TOKEN_FILE}" ]]; then
  ADMIN_TOKEN=$(<"${TOKEN_FILE}")
else
  ADMIN_TOKEN=$(curl -fsS -X POST \
    "${GITEA_HOST}/api/v1/users/${ADMIN_USER}/tokens" \
    -u "${ADMIN_USER}:${ADMIN_PASS}" \
    -H "Content-Type: application/json" \
    -d "{\"name\":\"${TOKEN_NAME}\",\"scopes\":[\"all\"]}" 2>/dev/null | \
    grep -o '"sha1":"[^"]*"' | cut -d'"' -f4) || true
  if [[ -n "${ADMIN_TOKEN}" ]]; then
    (umask 077 && printf '%s\n' "${ADMIN_TOKEN}" > "${TOKEN_FILE}")
    chmod 600 "${TOKEN_FILE}"
  fi
fi

if [[ -z "${ADMIN_TOKEN:-}" ]]; then
  echo "[bootstrap] WARN: could not mint an admin token; repo creation will be skipped"
  exit 0
fi

echo "[bootstrap] ensuring repository ${REPO_OWNER}/${REPO_NAME} exists"
if ! curl -fsS -o /dev/null \
     -H "Authorization: token ${ADMIN_TOKEN}" \
     "${GITEA_HOST}/api/v1/repos/${REPO_OWNER}/${REPO_NAME}"; then
  curl -fsS -X POST "${GITEA_HOST}/api/v1/user/repos" \
    -H "Authorization: token ${ADMIN_TOKEN}" \
    -H "Content-Type: application/json" \
    -d "{\"name\":\"${REPO_NAME}\",\"auto_init\":true,\"private\":false}" >/dev/null
fi

# Gitea's HTTP host as seen from this container is the docker service
# name. Use it explicitly so the URL works regardless of how the
# stack is reachable from outside (localhost:3000 on the host,
# gitea:3000 inside the compose network).
GITEA_HOST_DOCKER="http://gitea:3000"

if [[ -d "${EXAMPLES_DIR}" ]]; then
  echo "[bootstrap] seeding ${EXAMPLES_DIR} into ${REPO_OWNER}/${REPO_NAME}"
  WORK=$(mktemp -d)
  git -C "${WORK}" init -q -b main
  git -C "${WORK}" config user.name "${ADMIN_USER}"
  git -C "${WORK}" config user.email "${ADMIN_EMAIL}"
  git -C "${WORK}" remote add origin \
    "http://${ADMIN_USER}:${ADMIN_PASS}@gitea:3000/${REPO_OWNER}/${REPO_NAME}.git"
  cp "${EXAMPLES_DIR}"/*.TcPOU "${EXAMPLES_DIR}"/*.TcDUT "${EXAMPLES_DIR}"/*.TcGVL "${WORK}/" 2>/dev/null || cp -r "${EXAMPLES_DIR}/." "${WORK}/"
  git -C "${WORK}" add .
  if git -C "${WORK}" diff --cached --quiet; then
    echo "[bootstrap] no changes to commit"
  else
    git -C "${WORK}" commit -q -m "Seed TwinCAT demo files"
    # The repo was created with auto_init=true, so main already has a
    # commit. Use --force-with-lease-equivalent force push to overwrite.
    git -C "${WORK}" push -q -u origin +main 2>&1 || git -C "${WORK}" push -q -u origin main
  fi
  rm -rf "${WORK}"
else
  echo "[bootstrap] WARNING: ${EXAMPLES_DIR} does not exist; skipping seed"
fi

echo "[bootstrap] done"
