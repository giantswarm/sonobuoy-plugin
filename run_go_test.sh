#!/usr/bin/env bash

set -eo pipefail

results_dir="${RESULTS_DIR:-/tmp/results}"
junit_report_file="${results_dir}/combined-report.xml"

# saveResults prepares the results for handoff to the Sonobuoy worker.
# See: https://github.com/vmware-tanzu/sonobuoy/blob/master/site/docs/master/plugins.md
saveResults() {
	# Signal to the worker that we are done and where to find the results.
	printf ${junit_report_file} > "${results_dir}/done"
}

# Ensure that we tell the Sonobuoy worker we are done regardless of results.
trap saveResults EXIT

mkdir "${results_dir}" || true

echo "Report will be saved to ${junit_report_file}"

# Run all tests.
go test -v -timeout 99999s ./tests/... 2>&1 | go-junit-report > "${results_dir}/report.xml"

# Run the deletion test (tiers down the cluster).
go test -v -timeout 99999s ./deletiontests/... 2>&1 | go-junit-report > "${results_dir}/deletiontests.xml"

jrm "${junit_report_file}" "${results_dir}/report.xml" "${results_dir}/deletiontests.xml"
