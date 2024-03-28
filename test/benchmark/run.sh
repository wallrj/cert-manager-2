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

label="cert-manager-metrics-experiment"

: ${TARGET_CERTIFICATES_COUNT:=2000}

export EXPERIMENT_ID="${RANDOM}"
export DATA_FILE="experiment.data.${EXPERIMENT_ID}.json"

logger -s \
       experiment_id="${EXPERIMENT_ID}" \
       data_file="${DATA_FILE}" \
       target_certificates_count="${TARGET_CERTIFICATES_COUNT}"

./measure.sh

trap "systemctl  --user stop  '*${EXPERIMENT_ID}*'" EXIT

systemd-run --user \
            --unit=${EXPERIMENT_ID}-measure \
            --collect \
            --same-dir \
            --setenv "PATH=${PATH}" \
            --setenv "EXPERIMENT_ID=${EXPERIMENT_ID}" \
            --setenv "DATA_FILE=${DATA_FILE}" \
            --on-calendar '*:*:0/10' \
            ./measure.sh

systemd-run --user \
            --unit=${EXPERIMENT_ID}-load \
            --collect \
            --same-dir \
            --setenv "PATH=${PATH}" \
            --setenv "EXPERIMENT_ID=${EXPERIMENT_ID}" \
            --property=StartLimitBurst=1000 \
            --timer-property=AccuracySec=1s \
            --on-calendar '*:*:0/1' \
            ./load.sh

logger -s PHASE: RAMP UP
while true; do
    actual_certificates_count=$(jq --slurp '.[-1].certificates_count // -1' "${DATA_FILE}")
    actual_certificaterequests_count=$(jq --slurp '.[-1].certificaterequests_count // -1' "${DATA_FILE}")
    actual_secrets_count=$(jq --slurp '.[-1].secrets_count // -1' "${DATA_FILE}")
    actual_secrets_size=$(jq --slurp '.[-1].secrets_size // -1' "${DATA_FILE}")
    logger -s \
           actual_certificates_count=${actual_certificates_count} \
           actual_certificaterequests_count=${actual_certificaterequests_count} \
           actual_secrets_count=${actual_secrets_count} \
           actual_secrets_size=${actual_secrets_size}

    if [[ "${actual_certificates_count}" -ge "${TARGET_CERTIFICATES_COUNT}" ]]; then
        logger -s PHASE: RAMP UP: TARGET REACHED
        systemctl  --user stop "*${EXPERIMENT_ID}-load*"
        break
    fi
    sleep 30
done

logger -s PHASE: CATCH UP
while true; do
    actual_certificates_count=$(jq --slurp '.[-1].certificates_count // -1' "${DATA_FILE}")
    actual_certificaterequests_count=$(jq --slurp '.[-1].certificaterequests_count // -1' "${DATA_FILE}")
    actual_secrets_count=$(jq --slurp '.[-1].secrets_count // -1' "${DATA_FILE}")
    actual_secrets_size=$(jq --slurp '.[-1].secrets_size // -1' "${DATA_FILE}")
    logger -s \
           actual_certificates_count=${actual_certificates_count} \
           actual_certificaterequests_count=${actual_certificaterequests_count} \
           actual_secrets_count=${actual_secrets_count} \
           actual_secrets_size=${actual_secrets_size}

    if [[ "${actual_certificaterequests_count}" -ge "${TARGET_CERTIFICATES_COUNT}" ]]; then
        logger -s PHASE: CATCH UP: TARGET REACHED
        logger -s All Certificates reconciled
        break
    fi
    sleep 30
done

logger -s PHASE: STEADY STATE
sleep 300

logger -s PHASE: RAMP DOWN
kubectl delete ns --selector=cert-manager-metrics-experiment
sleep 60
