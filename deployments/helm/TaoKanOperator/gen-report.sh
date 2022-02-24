#! /bin/bash
IFS=$'\n'

LOG_FILE="taokan-report-$(date +'%Y-%m-%d-%H%M').log"

info() {
  echo -e "\033[0;32m$1\033[0m"
}

warn() {
  echo -e "\033[0;93m$1\033[0m"
}

error() {
  echo -e "\033[0;91m$1\033[0m" >&2
}

parse_rsync_summary_log() {
  pod_name=$1
  echo "[ ${pod_name/rsync-worker-/} ]" >> $LOG_FILE
  kubectl logs -n hub $pod_name --tail 40 | grep "Number of files" -A15 >> $LOG_FILE
  total_time=$(kubectl logs -n hub $pod_name --tail 40 | grep "Number of files:" -B2 | head -n1 | awk '{print $5}')
  echo "Total transfer time: ${total_time}" >> $LOG_FILE
  echo "" >> $LOG_FILE
}

handle_running_pod() {
  pod_name=$1
  echo "[ ${pod_name/rsync-worker-/} ]" >> $LOG_FILE
  echo "  Pod $pod_name is still running" >> $LOG_FILE
  echo "" >> $LOG_FILE
}

handle_uknown_status_pod() {
  pod_name=$1
  pod_status=$2
  echo "[ ${pod_name/rsync-worker-/} ]" >> $LOG_FILE
  echo "Pod $pod_name is failed due to $pod_status" >> $LOG_FILE
  kubectl describe pod -n hub $pod_name | grep "State:" -A21 >> $LOG_FILE
  echo "" >> $LOG_FILE
}

main() {
  touch $LOG_FILE
  echo "Generating backup report for $(date +'%Y-%m-%d %H:%M:%S')" > $LOG_FILE
  echo >> $LOG_FILE
  for line in $(kubectl get pod -n hub -l app=rsync-worker | grep -v NAME)
  do
    pod_name="$(echo $line | awk '{print $1}')"
    pod_status="$(echo $line | awk '{print $3}')"

    if [[ "$pod_status" == "Running" ]]; then
      warn "[!] Pod $pod_name is still running"
      handle_running_pod $pod_name
    elif [[ "$pod_status" == "Completed" ]]; then
      info "[O] Pod $pod_name is backup completed"
      parse_rsync_summary_log $pod_name
    else
      error "[X] Pod $pod_name is status $pod_status"
      handle_uknown_status_pod $pod_name $pod_status
      continue
    fi
  done

  echo "Generate report '$LOG_FILE' successfully. Please check it."
}

main "$@"
