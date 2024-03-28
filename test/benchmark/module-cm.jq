# siParse converts a string quantity like "1Ki" to a number 1024
def siParse:
  {n: pow(10;-9), m: pow(10;-3), Ki: pow(2;10), Mi: pow(2;20), Gi: pow(2;30)} as $multipliers
  | capture("(?<value>\\d+)(?<si>n|m|Ki|Mi|Gi)")
  | (.value | tonumber) * $multipliers[.si]
;

# summarizeSecretList parses the output of `kubectl get secrets -o json`, counts
# the Secrets and calculates the total size of data in all the Secrets.
def summarizeSecretList:
  .items
  | {
    "secrets_size": map(.data // {} | map(@base64d | length) | add) | add,
    "secrets_count": length
  }
;

# summarizePodMetrics parses the output of Metrics Server API.
# `kubectl get --raw /apis/metrics.k8s.io/v1beta1/namespaces/cert-manager/pods`
def summarizePodMetrics:
  .items
  | map(
        .metadata.labels["app.kubernetes.io/component"] as $component
        |
        {
          "\($component)_memory": .containers[0].usage.memory | siParse,
          "\($component)_cpu": .containers[0].usage.cpu | siParse,
        }
      )
  | add
;

# recordsToCSV converts a list of objects to CSV
# https://stackoverflow.com/questions/32960857/how-to-convert-arbitrary-simple-json-to-csv-using-jq
def recordsToCSV:
    (map(keys) | add | unique) as $cols | map(. as $row | $cols | map($row[.])) as $rows | $cols, $rows[] | @csv
;
