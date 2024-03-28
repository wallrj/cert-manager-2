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

: ${EXPERIMENT_ID?}

label="cert-manager-metrics-experiment"
namespace="application-${RANDOM}"

kubectl create ns "${namespace}"
kubectl label ns "${namespace}" "${label}=${EXPERIMENT_ID}"
kubectl apply --namespace "${namespace}" -f application.yaml
kubectl label --namespace "${namespace}" -f application.yaml "${label}=${EXPERIMENT_ID}"
# kubectl wait --namespace "${namespace}" --timeout=2m --for=condition=ready=true -f application.yaml
