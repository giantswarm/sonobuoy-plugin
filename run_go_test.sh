#!/usr/bin/env sh

results_dir="${RESULTS_DIR:-/tmp/results}"

# saveResults prepares the results for handoff to the Sonobuoy worker.
# See: https://github.com/vmware-tanzu/sonobuoy/blob/master/docs/plugins.md
saveResults() {
	# Signal to the worker that we are done and where to find the results.
	printf ${results_dir}/out > "${results_dir}/done"
}

# Ensure that we tell the Sonobuoy worker we are done regardless of results.
trap saveResults EXIT

mkdir "${results_dir}" || true
go test -v -timeout 99999s ./... 2>&1 | tee -a "${results_dir}/out"
