#!/usr/bin/env bash

MAKE_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)"

export ARTIFACTS=${ARTIFACTS:-"./_artifacts"}
export KUBE_JUNIT_REPORT_DIR="${ARTIFACTS}"
export KUBE_KEEP_VERBOSE_TEST_OUTPUT=y
export LOG_LEVEL=4

cd $MAKE_ROOT/kubernetes

# Install etcd for tests
./hack/install-etcd.sh

# Install gotestsum which is used to get the junit output
go get gotest.tools/gotestsum

# TODO: i am assuming we should be setting this somewhere else since we are using go elsewhere....
# without this it cant find the gotestsum
export PATH="${GOPATH}/bin:${PATH}"

# There appear to be some occasional flakes
# make test in the upstream repo caches successful results so subsequent runs skip previously
# passing tests
MAX_RETRIES=3
n=0
until [ "$n" -ge $MAX_RETRIES ]
do
   PATH="${MAKE_ROOT}/kubernetes/third_party/etcd:${PATH}" make test KUBE_TIMEOUT=--timeout=600s && break  || $ret
   n=$((n+1)) 
   sleep 15
done

exit $ret
