#!/usr/bin/env bash
set -euo pipefail

workflow_run="${OPENMP_WORKFLOW_RUN:-31335425385}"
artifact_sha256="${OPENMP_ARTIFACT_SHA256:-048803a567af43712192c31b640be2700f61776c9b73c610706303c2bd249379}"
artifact_url="https://nightly.link/openmultiplayer/open.mp/actions/runs/${workflow_run}/open.mp-linux-x86_64-.zip"
workspace_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

if [[ "${1:-}" != "--skip-build" ]]; then
    task --dir "${workspace_dir}" component
fi

test_dir="$(mktemp -d)"
cleanup() {
    rm -rf -- "${test_dir}"
}
trap cleanup EXIT

echo "Downloading pinned open.mp Linux x86-64 artifact"
curl --fail --location --show-error --progress-bar \
    --connect-timeout 15 --max-time 180 --retry 3 --retry-all-errors \
    "${artifact_url}" --output "${test_dir}/openmp.zip"
printf '%s  %s\n' "${artifact_sha256}" "${test_dir}/openmp.zip" | sha256sum --check --status
unzip -q "${test_dir}/openmp.zip" -d "${test_dir}"
tar -xJf "${test_dir}/open.mp-linux-x86_64-.tar.xz" -C "${test_dir}"
server_dir="${test_dir}/Server"
cp "${workspace_dir}/build/ompphp.so" "${server_dir}/components/ompphp.so"
g++ -std=c++17 -shared -fPIC \
    -I"${workspace_dir}/third_party/omp-capi/lib/open.mp-sdk/include" \
    -I"${workspace_dir}/third_party/omp-capi/lib/open.mp-sdk/lib/glm" \
    -I"${workspace_dir}/third_party/omp-capi/lib/open.mp-sdk/lib/robin-hood-hashing/src/include" \
    -I"${workspace_dir}/third_party/omp-capi/lib/open.mp-sdk/lib/span-lite/include" \
    -I"${workspace_dir}/third_party/omp-capi/lib/open.mp-sdk/lib/string-view-lite/include" \
    -I"${workspace_dir}/third_party/omp-capi/lib/open.mp-capi/include" \
    -DGLM_FORCE_QUAT_DATA_WXYZ -DGLM_FORCE_SSE2 -Dnssv_CONFIG_SELECT_STRING_VIEW=nssv_STRING_VIEW_NONSTD -Dspan_CONFIG_SELECT_SPAN=span_SPAN_NONSTD \
    "${workspace_dir}/tests/fixtures/callable_component.cpp" -ldl \
    -o "${server_dir}/components/ompphp-callable-fixture.so"

set +e
(
    cd "${server_dir}"
    timeout --signal=INT --kill-after=2s 2s ./omp-server
) >"${test_dir}/stock-capi.log" 2>&1
stock_status=$?
set -e
if [[ ${stock_status} -ne 0 && ${stock_status} -ne 124 ]]; then
    cat "${test_dir}/stock-capi.log"
    exit "${stock_status}"
fi
grep -F "incompatible CAPI component" "${test_dir}/stock-capi.log"
if grep -F "Successfully loaded component ompphp (" "${test_dir}/stock-capi.log"; then
    echo "ompphp unexpectedly loaded with the stock open.mp CAPI" >&2
    exit 1
fi

cp "${workspace_dir}/build/capi/linux/components/\$CAPI.so" "${server_dir}/components/\$CAPI.so"
cp "${workspace_dir}/tests/fixtures/e2e/gamemode.php" "${server_dir}/gamemode.php"
cp "${workspace_dir}/tests/fixtures/e2e/composer.json" "${server_dir}/composer.json"
cp -R "${workspace_dir}/tests/fixtures/e2e/src" "${server_dir}/src"
mkdir -p "${server_dir}/packages"
(
    cd "${workspace_dir}"
    go run ./tools/sdkpack -version 0.1.0-beta.1 -out "${server_dir}/packages"
)
if command -v composer >/dev/null 2>&1; then
    composer install --working-dir="${server_dir}" --no-dev --no-interaction --no-progress
else
    docker run --rm -u "$(id -u):$(id -g)" -v "${server_dir}:/app" -w /app composer:2 \
        install --no-dev --no-interaction --no-progress
fi

set +e
(
    cd "${server_dir}"
    OMPPHP_ENTRY=gamemode.php timeout --signal=INT --kill-after=2s 5s ./omp-server
) >"${test_dir}/server.log" 2>&1
server_status=$?
set -e

if [[ ${server_status} -ne 0 && ${server_status} -ne 124 ]]; then
    cat "${test_dir}/server.log"
    exit "${server_status}"
fi

grep -F "Successfully loaded component ompphp (" "${test_dir}/server.log"
grep -F "OMPPHP_E2E_READY" "${test_dir}/server.log"
grep -F "OMPPHP_E2E_SDK" "${test_dir}/server.log"
grep -F "OMPPHP_E2E_EXTENDED_CAPI" "${test_dir}/server.log"
grep -F "OMPPHP_E2E_CALLABLES" "${test_dir}/server.log" || { cat "${test_dir}/server.log"; exit 1; }
grep -F "PHP handler for Tick failed:" "${test_dir}/server.log"
grep -F "RuntimeException: OMPPHP_E2E_EXPECTED_FAILURE" "${test_dir}/server.log"
grep -F "Stack trace:" "${test_dir}/server.log"
grep -F "OMPPHP_E2E_TICK" "${test_dir}/server.log"
grep -F "OMPPHP_E2E_ASYNC" "${test_dir}/server.log"
grep -F "OMPPHP_E2E_ACTOR" "${test_dir}/server.log"
grep -F "OMPPHP_E2E_TIMER" "${test_dir}/server.log"
