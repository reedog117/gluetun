#!/usr/bin/env sh
set -eu

ROOT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
ENV_FILE="${ROOT_DIR}/.env.purevpn"
IMAGE_TAG="${IMAGE_TAG:-gluetun:purevpn-local}"
SERVERS_JSON="${ROOT_DIR}/internal/storage/servers.json"

# Per test case max wait for connection.
CONNECT_DEADLINE_SECONDS="${CONNECT_DEADLINE_SECONDS:-180}"
# Reduce failed OpenVPN handshakes from 60s to 15s for this test harness only.
FAILED_HANDSHAKE_TIMEOUT_SECONDS="${FAILED_HANDSHAKE_TIMEOUT_SECONDS:-15}"
# Set to 1 to force PUREVPN_SERVER_TYPE filtering in Gluetun.
USE_SERVER_TYPE_FILTER="${USE_SERVER_TYPE_FILTER:-0}"
TEST_TYPES="${TEST_TYPES:-regular,portforwarding,quantumresistant,obfuscation}"
TEST_PROTOCOLS="${TEST_PROTOCOLS:-udp,tcp}"
SUCCESS_CACHE="${SUCCESS_CACHE:-/tmp/purevpn_success_hosts.tsv}"
PREFERRED_HOST_PREFIXES="${PREFERRED_HOST_PREFIXES:-us,ca,uk,nl,de,fr,se,ch}"

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

pick_random_hostname() {
  case "$1" in
    regular) echo '.port_forward!=true and .quantum_resistant!=true and .obfuscated!=true' ;;
    portforwarding) echo '.port_forward==true' ;;
    quantumresistant) echo '.quantum_resistant==true' ;;
    obfuscation) echo '.obfuscated==true' ;;
    *) return 1 ;;
  esac
}

choose_hostname() {
  server_type="$1"
  protocol="$2"
  protocol_field="$protocol"

  trait_filter="$(pick_random_hostname "${server_type}")" || return 1
  candidates="$(jq -r ".purevpn.servers[] | select(.${protocol_field}==true) | select(${trait_filter}) | .hostname" "${SERVERS_JSON}")"
  if [ -z "${candidates}" ]; then
    return 1
  fi

  if [ -f "${SUCCESS_CACHE}" ]; then
    cached="$(awk -v t="${server_type}" -v p="${protocol}" '$1==t && $2==p {print $3; exit}' "${SUCCESS_CACHE}")"
    if [ -n "${cached}" ] && printf '%s\n' "${candidates}" | rg -qx "${cached}"; then
      echo "${cached}"
      return 0
    fi
  fi

  auto_candidates="$(printf '%s\n' "${candidates}" | rg -- '-auto-' || true)"
  preferred_regex="^($(echo "${PREFERRED_HOST_PREFIXES}" | tr ',' '|'))"
  preferred_candidates="$(printf '%s\n' "${candidates}" | rg -i "${preferred_regex}" || true)"
  preferred_auto_candidates="$(printf '%s\n' "${auto_candidates}" | rg -i "${preferred_regex}" || true)"

  if [ -n "${preferred_auto_candidates}" ]; then
    printf '%s\n' "${preferred_auto_candidates}" | awk 'BEGIN{srand()} {a[NR]=$0} END{if (NR>0) print a[int(rand()*NR)+1]}'
    return 0
  fi

  if [ -n "${preferred_candidates}" ]; then
    printf '%s\n' "${preferred_candidates}" | awk 'BEGIN{srand()} {a[NR]=$0} END{if (NR>0) print a[int(rand()*NR)+1]}'
    return 0
  fi

  if [ -n "${auto_candidates}" ]; then
    printf '%s\n' "${auto_candidates}" | awk 'BEGIN{srand()} {a[NR]=$0} END{if (NR>0) print a[int(rand()*NR)+1]}'
    return 0
  fi

  printf '%s\n' "${candidates}" | awk 'BEGIN{srand()} {a[NR]=$0} END{if (NR>0) print a[int(rand()*NR)+1]}'
}

store_success_host() {
  server_type="$1"
  protocol="$2"
  hostname="$3"
  tmp="$(mktemp)"
  {
    if [ -f "${SUCCESS_CACHE}" ]; then
      awk -v t="${server_type}" -v p="${protocol}" '!($1==t && $2==p)' "${SUCCESS_CACHE}"
    fi
    echo "${server_type} ${protocol} ${hostname}"
  } > "${tmp}"
  mv "${tmp}" "${SUCCESS_CACHE}"
}

test_case() {
  server_type="$1"
  protocol="$2"
  started_at="$(date +%s)"
  openvpn_flags="--connect-timeout ${FAILED_HANDSHAKE_TIMEOUT_SECONDS} --server-poll-timeout ${FAILED_HANDSHAKE_TIMEOUT_SECONDS} --hand-window ${FAILED_HANDSHAKE_TIMEOUT_SECONDS} --connect-retry-max 1"
  attempt=0
  first_attempt_success=0

  while :; do
    elapsed="$(( $(date +%s) - started_at ))"
    if [ "${elapsed}" -ge "${CONNECT_DEADLINE_SECONDS}" ]; then
      echo "[harness] ${server_type}/${protocol}: timeout after ${CONNECT_DEADLINE_SECONDS}s"
      echo "0"
      return 1
    fi

    hostname="$(choose_hostname "${server_type}" "${protocol}" || true)"
    if [ -z "${hostname}" ]; then
      echo "[harness] no hostname found for ${server_type}/${protocol} in bundled servers"
      echo "skip"
      return 0
    fi
    attempt=$((attempt + 1))

    name="gluetun-purevpn-${server_type}-${protocol}"
    log_file="/tmp/${name}.log"
    rm -f "${log_file}"
    docker rm -f "${name}" >/dev/null 2>&1 || true

    echo "[harness] ${server_type}/${protocol}: attempt ${attempt}, hostname ${hostname}"

    extra_type_env=""
    if [ "${USE_SERVER_TYPE_FILTER}" = "1" ]; then
      extra_type_env="-e PUREVPN_SERVER_TYPE=${server_type}"
    fi
    # shellcheck disable=SC2086
    docker run -d --name "${name}" \
      --cap-add=NET_ADMIN \
      -e VPN_SERVICE_PROVIDER=purevpn \
      -e VPN_TYPE=openvpn \
      -e OPENVPN_PROTOCOL="${protocol}" \
      -e OPENVPN_USER="${PUREVPN_USER}" \
      -e OPENVPN_PASSWORD="${PUREVPN_PASSWORD}" \
      -e SERVER_HOSTNAMES="${hostname}" \
      -e OPENVPN_FLAGS="${openvpn_flags}" \
      -e DOT=off \
      -e TZ="${TZ:-UTC}" \
      ${extra_type_env} \
      "${IMAGE_TAG}" >/dev/null

    attempt_started_at="$(date +%s)"
    attempt_failed=0
    while :; do
      if ! docker ps -a --format '{{.Names}}' | rg -qx "${name}"; then
        echo "[harness] ${server_type}/${protocol}: attempt ${attempt} container exited early"
        attempt_failed=1
        break
      fi
      docker logs "${name}" > "${log_file}" 2>&1 || true

      if rg -qi "connected|Connection established|Initialization Sequence Completed" "${log_file}"; then
        elapsed="$(( $(date +%s) - started_at ))"
        echo "[harness] ${server_type}/${protocol}: connected in ${elapsed}s (attempt ${attempt})"
        if [ "${attempt}" -eq 1 ]; then
          first_attempt_success=1
        fi
        store_success_host "${server_type}" "${protocol}" "${hostname}"
        docker rm -f "${name}" >/dev/null 2>&1 || true
        echo "${first_attempt_success}"
        return 0
      fi

      if rg -qi "TLS key negotiation failed|AUTH_FAILED|cannot load certificate|Exiting due to fatal error|no server found|connect-retry-max \\(1\\) times unsuccessful" "${log_file}"; then
        elapsed_attempt="$(( $(date +%s) - attempt_started_at ))"
        echo "[harness] ${server_type}/${protocol}: attempt ${attempt} failed in ${elapsed_attempt}s"
        tail -n 12 "${log_file}" || true
        docker rm -f "${name}" >/dev/null 2>&1 || true
        attempt_failed=1
        break
      fi

      elapsed_attempt="$(( $(date +%s) - attempt_started_at ))"
      if [ "${elapsed_attempt}" -ge "${FAILED_HANDSHAKE_TIMEOUT_SECONDS}" ]; then
        echo "[harness] ${server_type}/${protocol}: attempt ${attempt} exceeded ${FAILED_HANDSHAKE_TIMEOUT_SECONDS}s without success"
        tail -n 12 "${log_file}" || true
        docker rm -f "${name}" >/dev/null 2>&1 || true
        attempt_failed=1
        break
      fi

      sleep 2
    done

    if [ "${attempt_failed}" -eq 1 ]; then
      continue
    fi
  done
}

echo "[harness] Building docker image ${IMAGE_TAG}..."
docker build -t "${IMAGE_TAG}" "${ROOT_DIR}" >/dev/null

failed=0

first_attempt_successes=0
tested_cases=0
for server_type in $(echo "${TEST_TYPES}" | tr ',' ' '); do
  for protocol in $(echo "${TEST_PROTOCOLS}" | tr ',' ' '); do
    result_file="$(mktemp)"
    test_case "${server_type}" "${protocol}" 2>&1 | tee "${result_file}"
    result="$(tail -n 1 "${result_file}")"
    rm -f "${result_file}"
    if [ "${result}" = "skip" ]; then
      continue
    fi
    tested_cases=$((tested_cases + 1))
    if [ "${result}" = "1" ]; then
      first_attempt_successes=$((first_attempt_successes + 1))
    fi
    if [ "${result}" != "1" ] && [ "${result}" != "0" ]; then
      failed=1
    fi
  done
done

if [ "${tested_cases}" -eq 0 ]; then
  echo "[harness] no testable cases found"
  exit 1
fi

score="$(awk -v a="${first_attempt_successes}" -v b="${tested_cases}" 'BEGIN { printf "%.2f", (100*a)/b }')"
echo "[harness] first-attempt success: ${first_attempt_successes}/${tested_cases} (${score}%)"

if [ "${failed}" -ne 0 ]; then
  exit 1
fi
