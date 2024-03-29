#!/usr/bin/env bash

set -o pipefail

results_dir="${RESULTS_DIR:-/tmp/results}"
junit_report_file="${results_dir}/combined-report.xml"

# saveResults prepares the results for handoff to the Sonobuoy worker.
# See: https://github.com/vmware-tanzu/sonobuoy/blob/master/site/docs/master/plugins.md
saveResults() {
  # Signal to the worker that we are done and where to find the results.
  printf ${junit_report_file} >"${results_dir}/done"
}

# Ensure that we tell the Sonobuoy worker we are done regardless of results.
trap saveResults EXIT

mkdir "${results_dir}" || true

# Run all tests.
go test -run "$E2E_FOCUS" -v -timeout 6h 2>&1 | tee -a "go_test_output" && go-junit-report <"go_test_output" >"${results_dir}/report.xml"

success=$?

# Run the deletion test (tear down the cluster).
go test -run "$E2E_FOCUS" -v -timeout 2h ./deletiontests/... 2>&1 | go-junit-report >"${results_dir}/deletiontests.xml"

deletionsuccess=$?

jrm "${junit_report_file}" "${results_dir}/report.xml" "${results_dir}/deletiontests.xml"

if [ $success -ne 0 ] || [ $deletionsuccess -ne 0 ]
then
  echo "Tests failed"
  exit 1
fi

exit 0
