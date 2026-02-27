#!/usr/bin/env sh
set -eu

ROOT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
ENV_FILE="${ROOT_DIR}/.env.purevpn"
SERVERS_JSON="${ROOT_DIR}/internal/storage/servers.json"
IMAGE_TAG="${IMAGE_TAG:-gluetun:purevpn-local}"
CONTAINER_NAME_PREFIX="${CONTAINER_NAME_PREFIX:-gluetun-purevpn-rand}"

CONNECT_TIMEOUT_SECONDS="${CONNECT_TIMEOUT_SECONDS:-30}"
RUNS="${RUNS:-20}"
WARMUP_ATTEMPTS_PER_COMBO="${WARMUP_ATTEMPTS_PER_COMBO:-10}"
KNOWN_GOOD_CACHE="${KNOWN_GOOD_CACHE:-/tmp/purevpn_known_good.tsv}"
PREFERRED_HOST_PREFIXES="${PREFERRED_HOST_PREFIXES:-us,ca,uk,nl,de,fr,se,ch}"
MEASURE_VIABLE_ONLY="${MEASURE_VIABLE_ONLY:-1}"
KNOWN_GOOD_RATIO_PERCENT="${KNOWN_GOOD_RATIO_PERCENT:-100}"
SKIP_BUILD="${SKIP_BUILD:-0}"

if [ -f "${ENV_FILE}" ]; then
  # shellcheck disable=SC1090
  . "${ENV_FILE}"
fi

if [ -z "${PUREVPN_USER:-}" ] && [ -n "${PUREVPN_USERNAME:-}" ]; then
  PUREVPN_USER="${PUREVPN_USERNAME}"
fi

: "${PUREVPN_USER:?PUREVPN_USER (or PUREVPN_USERNAME) is required}"
: "${PUREVPN_PASSWORD:?PUREVPN_PASSWORD is required}"

pick_random_line() {
  awk 'BEGIN{srand()} {a[NR]=$0} END{if (NR>0) print a[int(rand()*NR)+1]}'
}

rand_percent() {
  awk 'BEGIN{srand(); printf "%d\n", int(rand()*100)}'
}

combo_filter_jq() {
  server_type="$1"
  case "${server_type}" in
    regular) echo '.port_forward!=true and .quantum_resistant!=true and .obfuscated!=true' ;;
    portforwarding) echo '.port_forward==true' ;;
    quantumresistant) echo '.quantum_resistant==true' ;;
    obfuscation) echo '.obfuscated==true' ;;
    *) return 1 ;;
  esac
}

candidate_hosts() {
  server_type="$1"
  protocol="$2"
  trait_filter="$(combo_filter_jq "${server_type}")"
  if [ "${server_type}" = "obfuscation" ]; then
    # PureVPN obfuscated endpoints should be reached over TCP/443.
    [ "${protocol}" = "tcp" ] || return 0
    jq -r ".purevpn.servers[] | select(${trait_filter}) | .hostname" "${SERVERS_JSON}"
    return 0
  fi
  jq -r ".purevpn.servers[] | select(.${protocol}==true) | select(${trait_filter}) | .hostname" "${SERVERS_JSON}"
}

pick_host_for_combo() {
  server_type="$1"
  protocol="$2"
  candidates="$(candidate_hosts "${server_type}" "${protocol}")"
  [ -n "${candidates}" ] || return 1

  preferred_regex="^($(echo "${PREFERRED_HOST_PREFIXES}" | tr ',' '|'))"
  preferred="$(printf '%s\n' "${candidates}" | rg -i "${preferred_regex}" || true)"
  if [ -n "${preferred}" ]; then
    printf '%s\n' "${preferred}" | pick_random_line
    return 0
  fi
  printf '%s\n' "${candidates}" | pick_random_line
}

remember_good_host() {
  server_type="$1"
  protocol="$2"
  host="$3"
  tmp="$(mktemp)"
  {
    if [ -f "${KNOWN_GOOD_CACHE}" ]; then
      awk -v t="${server_type}" -v p="${protocol}" -v h="${host}" '!($1==t && $2==p && $3==h)' "${KNOWN_GOOD_CACHE}"
    fi
    echo "${server_type} ${protocol} ${host}"
  } > "${tmp}"
  mv "${tmp}" "${KNOWN_GOOD_CACHE}"
}

pick_known_good_host() {
  server_type="$1"
  protocol="$2"
  if [ ! -f "${KNOWN_GOOD_CACHE}" ]; then
    return 1
  fi
  awk -v t="${server_type}" -v p="${protocol}" '$1==t && $2==p {print $3}' "${KNOWN_GOOD_CACHE}" | pick_random_line
}

run_attempt() {
  label="$1"
  server_type="$2"
  protocol="$3"
  host="$4"
  name="${CONTAINER_NAME_PREFIX}-${server_type}-${protocol}"
  log_file="/tmp/${name}.log"
  openvpn_flags="--connect-timeout ${CONNECT_TIMEOUT_SECONDS} --server-poll-timeout ${CONNECT_TIMEOUT_SECONDS} --hand-window ${CONNECT_TIMEOUT_SECONDS} --connect-retry-max 1"
  endpoint_port_env=""

  if [ "${server_type}" = "obfuscation" ]; then
    protocol="tcp"
    endpoint_port_env="-e OPENVPN_ENDPOINT_PORT=443"
  fi

  docker rm -f "${name}" >/dev/null 2>&1 || true
  rm -f "${log_file}"

  echo "[random] ${label}: ${server_type}/${protocol} host=${host}"
  docker run -d --name "${name}" \
    --cap-add=NET_ADMIN \
    -e VPN_SERVICE_PROVIDER=purevpn \
    -e VPN_TYPE=openvpn \
    -e OPENVPN_PROTOCOL="${protocol}" \
    -e OPENVPN_USER="${PUREVPN_USER}" \
    -e OPENVPN_PASSWORD="${PUREVPN_PASSWORD}" \
    -e SERVER_HOSTNAMES="${host}" \
    -e PUREVPN_SERVER_TYPE="${server_type}" \
    -e OPENVPN_FLAGS="${openvpn_flags}" \
    ${endpoint_port_env} \
    -e DOT=off \
    -e TZ="${TZ:-UTC}" \
    "${IMAGE_TAG}" >/dev/null

  started="$(date +%s)"
  while :; do
    if ! docker ps -a --format '{{.Names}}' | rg -qx "${name}"; then
      echo "[random] ${label}: container exited early"
      return 1
    fi

    docker logs "${name}" > "${log_file}" 2>&1 || true
    if rg -qi "connected|Connection established|Initialization Sequence Completed" "${log_file}"; then
      elapsed="$(( $(date +%s) - started ))"
      echo "[random] ${label}: success in ${elapsed}s"
      remember_good_host "${server_type}" "${protocol}" "${host}"
      docker rm -f "${name}" >/dev/null 2>&1 || true
      return 0
    fi
    if rg -qi "AUTH_FAILED|Exiting due to fatal error|no server found|connect-retry-max \\(1\\) times unsuccessful|Connection refused|Host is unreachable" "${log_file}"; then
      echo "[random] ${label}: failed early"
      docker rm -f "${name}" >/dev/null 2>&1 || true
      return 1
    fi

    elapsed="$(( $(date +%s) - started ))"
    if [ "${elapsed}" -ge "${CONNECT_TIMEOUT_SECONDS}" ]; then
      echo "[random] ${label}: timeout at ${CONNECT_TIMEOUT_SECONDS}s"
      docker rm -f "${name}" >/dev/null 2>&1 || true
      return 1
    fi
    sleep 2
  done
}

list_combos() {
  for server_type in regular portforwarding quantumresistant obfuscation; do
    for protocol in udp tcp; do
      count="$(candidate_hosts "${server_type}" "${protocol}" | wc -l | tr -d ' ')"
      if [ "${count}" -gt 0 ]; then
        echo "${server_type} ${protocol}"
      fi
    done
  done
}

if [ "${SKIP_BUILD}" != "1" ]; then
  echo "[random] building ${IMAGE_TAG}..."
  docker build -t "${IMAGE_TAG}" "${ROOT_DIR}" >/dev/null
fi

combos="$(list_combos)"
if [ -z "${combos}" ]; then
  echo "[random] no available type/protocol combos in bundled servers"
  exit 1
fi
combos_file="$(mktemp)"
printf '%s\n' "${combos}" > "${combos_file}"

echo "[random] available combos:"
sed 's/^/[random] - /' "${combos_file}"

# Phase 1: known-good verification.
echo "[random] phase 1: known-good verification"
smoke_ok=0
while read -r server_type protocol; do
  good_host="$(pick_known_good_host "${server_type}" "${protocol}" || true)"
  if [ -n "${good_host}" ]; then
    if run_attempt "smoke-known-good" "${server_type}" "${protocol}" "${good_host}"; then
      smoke_ok=1
      break
    fi
  fi
done < "${combos_file}"

if [ "${smoke_ok}" -eq 0 ]; then
  # No cached good host worked, bootstrap one.
  echo "[random] no cached known-good host worked; bootstrapping one"
  bootstrapped=0
  while read -r server_type protocol; do
    i=1
    while [ "${i}" -le "${WARMUP_ATTEMPTS_PER_COMBO}" ]; do
      host="$(pick_host_for_combo "${server_type}" "${protocol}" || true)"
      [ -n "${host}" ] || break
      if run_attempt "smoke-bootstrap-${i}" "${server_type}" "${protocol}" "${host}"; then
        bootstrapped=1
        break 2
      fi
      i=$((i + 1))
    done
  done < "${combos_file}"
  if [ "${bootstrapped}" -eq 0 ]; then
    echo "[random] could not bootstrap any known-good host"
    rm -f "${combos_file}"
    exit 1
  fi
fi

# Phase 2: warmup cache for all combos.
echo "[random] phase 2: warmup per combo"
while read -r server_type protocol; do
  if pick_known_good_host "${server_type}" "${protocol}" >/dev/null 2>&1; then
    continue
  fi
  i=1
  while [ "${i}" -le "${WARMUP_ATTEMPTS_PER_COMBO}" ]; do
    host="$(pick_host_for_combo "${server_type}" "${protocol}" || true)"
    [ -n "${host}" ] || break
    if run_attempt "warmup-${server_type}-${protocol}-${i}" "${server_type}" "${protocol}" "${host}"; then
      break
    fi
    i=$((i + 1))
  done
done < "${combos_file}"

# Phase 3: measured random tests.
echo "[random] phase 3: measured random tests (${RUNS} runs)"
measured_combos_file="${combos_file}"
if [ "${MEASURE_VIABLE_ONLY}" = "1" ]; then
  measured_combos_file="$(mktemp)"
  while read -r server_type protocol; do
    if pick_known_good_host "${server_type}" "${protocol}" >/dev/null 2>&1; then
      echo "${server_type} ${protocol}" >> "${measured_combos_file}"
    fi
  done < "${combos_file}"
  if [ ! -s "${measured_combos_file}" ]; then
    echo "[random] no viable combos with known-good hosts after warmup"
    rm -f "${measured_combos_file}" "${combos_file}"
    exit 1
  fi
  echo "[random] measuring viable combos only:"
  sed 's/^/[random] - /' "${measured_combos_file}"
fi

successes=0
attempted=0
for i in $(seq 1 "${RUNS}"); do
  combo="$(pick_random_line < "${measured_combos_file}")"
  server_type="$(echo "${combo}" | awk '{print $1}')"
  protocol="$(echo "${combo}" | awk '{print $2}')"

  # Pick known-good hosts according to configured ratio.
  if [ "$(rand_percent)" -lt "${KNOWN_GOOD_RATIO_PERCENT}" ]; then
    host="$(pick_known_good_host "${server_type}" "${protocol}" || true)"
  else
    host=""
  fi
  if [ -z "${host}" ]; then
    host="$(pick_host_for_combo "${server_type}" "${protocol}" || true)"
  fi
  [ -n "${host}" ] || continue

  attempted=$((attempted + 1))
  if run_attempt "measured-${i}" "${server_type}" "${protocol}" "${host}"; then
    successes=$((successes + 1))
  fi
done
if [ "${MEASURE_VIABLE_ONLY}" = "1" ]; then
  rm -f "${measured_combos_file}"
fi
rm -f "${combos_file}"

if [ "${attempted}" -eq 0 ]; then
  echo "[random] no measured attempts were executed"
  exit 1
fi

score="$(awk -v s="${successes}" -v a="${attempted}" 'BEGIN{printf "%.2f", (100*s)/a}')"
echo "[random] measured success rate: ${successes}/${attempted} (${score}%)"
if awk -v s="${score}" 'BEGIN{exit !(s>=75.0)}'; then
  echo "[random] target met: >= 75%"
else
  echo "[random] target missed: < 75%"
  exit 1
fi
