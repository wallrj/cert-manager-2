#!/usr/bin/env bash

# Copyright 2022 The cert-manager Authors.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

set -o nounset
set -o errexit
set -o pipefail

: ${DATA_FILE:=/dev/stdout}

TOKEN=$(kubectl create token -n cert-manager cert-manager)

function metadata_only() {
    curl $* \
         --insecure \
         --silent \
         -H 'Accept: application/json;as=PartialObjectMetadataList;g=meta.k8s.io;v=v1' \
         -H "Authorization: Bearer ${TOKEN}"
}

jq '{"timestamp": now, $certificates_count, $certificaterequests_count} + $secrets + $resources' \
   --null-input \
   --argfile certificates_count <(metadata_only https://127.0.0.1:34173/apis/cert-manager.io/v1/certificates?limit=1 | jq '.metadata.remainingItemCount + 1') \
   --argfile certificaterequests_count <(metadata_only https://127.0.0.1:34173/apis/cert-manager.io/v1/certificaterequests?limit=1 | jq '.metadata.remainingItemCount + 1') \
   --argfile secrets <(kubectl get secret --all-namespaces -o json | jq 'include "module-cm"; summarizeSecretList') \
   --argfile resources <(kubectl get --raw /apis/metrics.k8s.io/v1beta1/namespaces/cert-manager/pods | jq 'include "module-cm"; summarizePodMetrics') >> ${DATA_FILE}
