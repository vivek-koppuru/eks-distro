#!/usr/bin/env bash
# Copyright 2020 Amazon.com Inc. or its affiliates. All Rights Reserved.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#      http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

set -eo pipefail

BASEDIR=$(dirname "$0")
source ${BASEDIR}/set_environment.sh
$PREFLIGHT_CHECK_PASSED || exit 1

# Create the bucket if it doesn't exist
_bucket_name=$(aws s3api list-buckets  --query "Buckets[?Name=='$BUCKET_NAME'].Name | [0]" --out text)
if [ $_bucket_name == "None" ]; then
    echo "Creating kOps state store: $KOPS_STATE_STORE"
    if [ "$AWS_DEFAULT_REGION" == "us-east-1" ]; then
        aws s3api create-bucket --bucket $BUCKET_NAME
    else
        aws s3api create-bucket --bucket $BUCKET_NAME --create-bucket-configuration LocationConstraint=$AWS_DEFAULT_REGION
    fi
else
    echo "Using kOps state store: $KOPS_STATE_STORE"
fi

echo "Creating ./${KOPS_CLUSTER_NAME}/aws-iam-authenticator.yaml"
cat << EOF > ./${KOPS_CLUSTER_NAME}/aws-iam-authenticator.yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: aws-iam-authenticator
  namespace: kube-system
  labels:
    k8s-app: aws-iam-authenticator
data:
  config.yaml: |
    clusterID: $KOPS_CLUSTER_NAME
EOF

echo "Creating ${KOPS_CLUSTER_NAME}.yaml"
${KOPS} toolbox template --template eks-d.tpl --values ./${KOPS_CLUSTER_NAME}/values.yaml > "./${KOPS_CLUSTER_NAME}/${KOPS_CLUSTER_NAME}.yaml"

echo "Creating cluster configuration"
${KOPS} create -f "./${KOPS_CLUSTER_NAME}/${KOPS_CLUSTER_NAME}.yaml"

echo "Creating cluster ssh key"
export SSH_KEY_PATH=${SSH_KEY_PATH:-$HOME/.ssh/id_rsa.pub}
${KOPS} create secret --name $KOPS_CLUSTER_NAME sshpublickey admin -i ${SSH_KEY_PATH}

echo
echo "# Creating ./${KOPS_CLUSTER_NAME}/env.sh"
echo "export AWS_DEFAULT_REGION=$AWS_DEFAULT_REGION" | tee ./${KOPS_CLUSTER_NAME}/env.sh
echo "export AWS_REGION=$AWS_DEFAULT_REGION" | tee -a ./${KOPS_CLUSTER_NAME}/env.sh
echo "export KOPS_CLUSTER_NAME=$KOPS_CLUSTER_NAME" | tee -a ./${KOPS_CLUSTER_NAME}/env.sh
echo "export KOPS_STATE_STORE=$KOPS_STATE_STORE" | tee -a ./${KOPS_CLUSTER_NAME}/env.sh
