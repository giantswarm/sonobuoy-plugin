#!/usr/bin/env bash

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
go test -v -timeout 6h 2>&1 | tee -a "go_test_output" && go-junit-report <"go_test_output" >"${results_dir}/report.xml"

# Run the deletion test (tear down the cluster).
go test -v -timeout 2h ./deletiontests/... 2>&1 | go-junit-report >"${results_dir}/deletiontests.xml"

jrm "${junit_report_file}" "${results_dir}/report.xml" "${results_dir}/deletiontests.xml"
