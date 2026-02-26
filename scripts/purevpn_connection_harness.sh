#!/usr/bin/env sh
set -eu

ROOT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
ENV_FILE="${ROOT_DIR}/.env.purevpn"
IMAGE_TAG="gluetun:purevpn-local"

if [ -f "${ENV_FILE}" ]; then
  # shellcheck disable=SC1090
  . "${ENV_FILE}"
fi

# Support both PUREVPN_USER and PUREVPN_USERNAME.
if [ -z "${PUREVPN_USER:-}" ] && [ -n "${PUREVPN_USERNAME:-}" ]; then
  PUREVPN_USER="${PUREVPN_USERNAME}"
fi

: "${PUREVPN_USER:?PUREVPN_USER (or PUREVPN_USERNAME) is required (set in .env.purevpn or environment)}"
: "${PUREVPN_PASSWORD:?PUREVPN_PASSWORD is required (set in .env.purevpn or environment)}"

run_test() {
  protocol="$1"
  name="gluetun-purevpn-${protocol}"

  echo "[harness] Running ${protocol} test..."
  docker rm -f "${name}" >/dev/null 2>&1 || true

  docker run --rm --name "${name}" \
    --cap-add=NET_ADMIN \
    -e VPN_SERVICE_PROVIDER=purevpn \
    -e VPN_TYPE=openvpn \
    -e OPENVPN_PROTOCOL="${protocol}" \
    -e OPENVPN_USER="${PUREVPN_USER}" \
    -e OPENVPN_PASSWORD="${PUREVPN_PASSWORD}" \
    -e DOT=off \
    -e TZ="${TZ:-UTC}" \
    "${IMAGE_TAG}" 2>&1 | tee "/tmp/${name}.log"

  if ! rg -q "connected|Connection established|Initialization Sequence Completed" "/tmp/${name}.log"; then
    echo "[harness] ${protocol} test did not reach a clear connected state"
    return 1
  fi

  echo "[harness] ${protocol} test passed"
}

echo "[harness] Building docker image ${IMAGE_TAG}..."
docker build -t "${IMAGE_TAG}" "${ROOT_DIR}"

run_test tcp
run_test udp

echo "[harness] All tests passed"
