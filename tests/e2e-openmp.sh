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
cp "${workspace_dir}/tests/fixtures/e2e/gamemode.php" "${server_dir}/gamemode.php"
cp "${workspace_dir}/tests/fixtures/e2e/composer.json" "${server_dir}/composer.json"
mkdir -p "${server_dir}/packages"
(
    cd "${workspace_dir}"
    go run ./tools/sdkpack -version 0.1.0-beta.1 -out "${server_dir}/packages"
)
composer install --working-dir="${server_dir}" --no-dev --no-interaction --no-progress

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

grep -F "Successfully loaded component ompphp" "${test_dir}/server.log"
grep -F "OMPPHP_E2E_READY" "${test_dir}/server.log"
grep -F "OMPPHP_E2E_SDK" "${test_dir}/server.log"
grep -F "OMPPHP_E2E_TICK" "${test_dir}/server.log"
