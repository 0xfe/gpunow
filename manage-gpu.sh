#!/usr/bin/env bash

set -euo pipefail

# Creates a single L4 GPU VM that auto-deletes at termination (spot + max run duration).
# Note: this command will create a resource named "gpu0" in the specified project/zone.
#
# Monthly cost for this config is $342.37

INSTANCE_NAME="gpu0"
DISK_NAME="$INSTANCE_NAME"
PORTS_DEFAULT="22,80,443,8000"
PORTS_TAG="${INSTANCE_NAME}-ports"
CLOUD_INIT_TEMPLATE="gpu-cloud-init.yaml"
SETUP_SCRIPT="setup.sh"
CLOUD_INIT_RENDERED=""

# Project/region/machine settings
PROJECT="symbolic-axe-717"
ZONE="us-east1-d"
MACHINE_TYPE="g2-standard-16"

# Network interface settings
NETWORK_NAME="default"
NETWORK_IFACE="network=${NETWORK_NAME},network-tier=PREMIUM,stack-type=IPV4_ONLY"

# Scheduling/spot settings
PROVISIONING_MODEL="SPOT"
MAX_RUN_HOURS_DEFAULT="12"
INSTANCE_TERMINATION_ACTION="DELETE"
# Note: stopping an instance does NOT delete disks. Disks are deleted only when
# the instance is deleted and the disk's auto-delete flag is enabled.

# Service account + scopes
SERVICE_ACCOUNT="846951638556-compute@developer.gserviceaccount.com"
SCOPES="https://www.googleapis.com/auth/devstorage.read_only,https://www.googleapis.com/auth/logging.write,https://www.googleapis.com/auth/monitoring.write,https://www.googleapis.com/auth/service.management.readonly,https://www.googleapis.com/auth/servicecontrol,https://www.googleapis.com/auth/trace.append"

# GPU accelerator
ACCELERATOR="count=1,type=nvidia-l4"

# Network tags / firewall
TAGS_BASE="http-server,https-server,lb-health-check"
TAGS="${TAGS_BASE},${PORTS_TAG}"

# Boot disk and image
DISK_SPEC="auto-delete=yes,boot=yes,device-name=${INSTANCE_NAME},image=projects/ubuntu-os-accelerator-images/global/images/ubuntu-accelerator-2404-amd64-with-nvidia-580-v20260118,mode=rw,size=200,type=pd-standard"

# Shielded VM options
SHIELDED_SECURE_BOOT="no"
SHIELDED_VTPM="yes"
SHIELDED_INTEGRITY_MONITORING="yes"

# Labels and reservation
LABELS="gpu=,dev=,goog-ec-src=vm_add-gcloud"
RESERVATION_AFFINITY="none"

# Key revocation behavior
KEY_REVOCATION_ACTION="stop"

usage() {
  cat <<EOF
Usage: $(basename "$0") <start|stop|show|update> [options]

Commands:
  start  Create or start the GPU instance if needed
         --ports 22,80,443,8000 (comma-separated; default: $PORTS_DEFAULT)
         --max-hours N (override max run duration in hours)
  stop   Stop the GPU instance (default) or delete it
         --delete      Delete the instance instead of stopping
         --keep-disks  Preserve disks when deleting
  show   Show instance details
  update Update instance scheduling settings (instance must be stopped)
         --max-hours N (set max run duration in hours)
EOF
}

get_status() {
  gcloud compute instances describe "$INSTANCE_NAME" \
    --project="$PROJECT" \
    --zone="$ZONE" \
    --format="value(status)" 2>/dev/null || true
}

disk_exists() {
  gcloud compute disks describe "$DISK_NAME" \
    --project="$PROJECT" \
    --zone="$ZONE" >/dev/null 2>&1
}

ports_to_allow() {
  local ports_csv="$1"
  local allow=""
  local port
  IFS=',' read -r -a ports <<< "$ports_csv"
  for port in "${ports[@]}"; do
    port="${port// /}"
    if [[ -z "$port" ]]; then
      continue
    fi
    if [[ -n "$allow" ]]; then
      allow+=","
    fi
    allow+="tcp:${port}"
  done
  printf "%s" "$allow"
}

ensure_firewall_rule() {
  local ports_csv="$1"
  local allow
  local rule_name="${INSTANCE_NAME}-ports"
  allow="$(ports_to_allow "$ports_csv")"
  if [[ -z "$allow" ]]; then
    echo "No ports specified."
    exit 1
  fi

  if gcloud compute firewall-rules describe "$rule_name" \
    --project="$PROJECT" >/dev/null 2>&1; then
    run_firewall_cmd update "$rule_name" \
      --project="$PROJECT" \
      --allow="$allow" \
      --target-tags="$PORTS_TAG"
  else
    run_firewall_cmd create "$rule_name" \
      --project="$PROJECT" \
      --network="$NETWORK_NAME" \
      --allow="$allow" \
      --target-tags="$PORTS_TAG" \
      --direction=INGRESS
  fi
}

run_firewall_cmd() {
  local action="$1"
  shift
  local output
  if ! output="$(gcloud compute firewall-rules "$action" "$@" 2>&1)"; then
    echo "$output" >&2
    return 1
  fi
  echo "$output" | sed -E 's#https?://[^ ]*/projects/#projects/#g'
}

ensure_instance_tags() {
  gcloud compute instances add-tags "$INSTANCE_NAME" \
    --project="$PROJECT" \
    --zone="$ZONE" \
    --tags="$TAGS"
}

set_max_run_duration() {
  local max_hours="$1"
  local duration="${max_hours}h"
  gcloud compute instances set-scheduling "$INSTANCE_NAME" \
    --project="$PROJECT" \
    --zone="$ZONE" \
    --max-run-duration="$duration" \
    --instance-termination-action="$INSTANCE_TERMINATION_ACTION"
}

render_cloud_init() {
  if [[ ! -f "$CLOUD_INIT_TEMPLATE" ]]; then
    echo "Missing cloud-init template: $CLOUD_INIT_TEMPLATE"
    exit 1
  fi
  if [[ ! -f "$SETUP_SCRIPT" ]]; then
    echo "Missing setup script: $SETUP_SCRIPT"
    exit 1
  fi

  local tmp_file
  tmp_file="$(mktemp -t gpu-cloud-init.XXXXXX.yaml)"
  awk -v setup_file="$SETUP_SCRIPT" -v indent="      " '
    BEGIN {
      while ((getline line < setup_file) > 0) {
        setup[++n] = line
      }
      close(setup_file)
    }
    /{{SETUP_SH}}/ {
      for (i = 1; i <= n; i++) {
        print indent setup[i]
      }
      next
    }
    { print }
  ' "$CLOUD_INIT_TEMPLATE" > "$tmp_file"
  CLOUD_INIT_RENDERED="$tmp_file"
  trap 'rm -f "$CLOUD_INIT_RENDERED"' EXIT
}

start_instance() {
  local ports_csv="$1"
  local max_hours="$2"
  local max_hours_set="$3"
  local status
  status="$(get_status)"

  ensure_firewall_rule "$ports_csv"
  if [[ -n "$status" ]]; then
    ensure_instance_tags
  fi

  if [[ "$status" == "RUNNING" ]]; then
    echo "$INSTANCE_NAME is already RUNNING."
    if [[ "$max_hours_set" == "true" ]]; then
      echo "Max run duration can only be updated while stopped. Use: ./manage-gpu.sh update --max-hours $max_hours"
    fi
    return 0
  fi

  if [[ -n "$status" ]]; then
    echo "Starting existing $INSTANCE_NAME (current status: $status)."
    if [[ "$max_hours_set" == "true" && "$status" == "TERMINATED" ]]; then
      set_max_run_duration "$max_hours"
    elif [[ "$max_hours_set" == "true" ]]; then
      echo "Max run duration can only be updated while stopped (TERMINATED)."
    fi
    gcloud compute instances start "$INSTANCE_NAME" \
      --project="$PROJECT" \
      --zone="$ZONE"
    return 0
  fi

  render_cloud_init

  local max_run_duration
  max_run_duration="${max_hours}h"

  if disk_exists; then
    local existing_disk_spec
    existing_disk_spec="auto-delete=yes,boot=yes,device-name=${INSTANCE_NAME},mode=rw,name=${DISK_NAME}"
    gcloud beta compute instances create "$INSTANCE_NAME" \
      --project="$PROJECT" \
      --zone="$ZONE" \
      --machine-type="$MACHINE_TYPE" \
      --network-interface="$NETWORK_IFACE" \
      --no-restart-on-failure \
      --maintenance-policy=TERMINATE \
      --provisioning-model="$PROVISIONING_MODEL" \
      --instance-termination-action="$INSTANCE_TERMINATION_ACTION" \
      --max-run-duration="$max_run_duration" \
      --service-account="$SERVICE_ACCOUNT" \
      --scopes="$SCOPES" \
      --accelerator="$ACCELERATOR" \
      --tags="$TAGS" \
      --metadata-from-file="user-data=${CLOUD_INIT_RENDERED}" \
      --disk="$existing_disk_spec" \
      --no-shielded-secure-boot \
      --shielded-vtpm \
      --shielded-integrity-monitoring \
      --labels="$LABELS" \
      --reservation-affinity="$RESERVATION_AFFINITY" \
      --key-revocation-action-type="$KEY_REVOCATION_ACTION"
  else
    gcloud beta compute instances create "$INSTANCE_NAME" \
      --project="$PROJECT" \
      --zone="$ZONE" \
      --machine-type="$MACHINE_TYPE" \
      --network-interface="$NETWORK_IFACE" \
      --no-restart-on-failure \
      --maintenance-policy=TERMINATE \
      --provisioning-model="$PROVISIONING_MODEL" \
      --instance-termination-action="$INSTANCE_TERMINATION_ACTION" \
      --max-run-duration="$max_run_duration" \
      --service-account="$SERVICE_ACCOUNT" \
      --scopes="$SCOPES" \
      --accelerator="$ACCELERATOR" \
      --tags="$TAGS" \
      --metadata-from-file="user-data=${CLOUD_INIT_RENDERED}" \
      --create-disk="$DISK_SPEC" \
      --no-shielded-secure-boot \
      --shielded-vtpm \
      --shielded-integrity-monitoring \
      --labels="$LABELS" \
      --reservation-affinity="$RESERVATION_AFFINITY" \
      --key-revocation-action-type="$KEY_REVOCATION_ACTION"
  fi
}

stop_instance() {
  local delete_instance="$1"
  local keep_disks="$2"
  local status
  status="$(get_status)"

  if [[ -z "$status" ]]; then
    echo "$INSTANCE_NAME not found."
    return 0
  fi

  if [[ "$delete_instance" == "true" ]]; then
    # Delete removes the VM; disks are deleted only if auto-delete is enabled,
    # unless --keep-disks is used.
    if [[ "$keep_disks" == "true" ]]; then
      gcloud compute instances delete "$INSTANCE_NAME" \
        --project="$PROJECT" \
        --zone="$ZONE" \
        --keep-disks=all
    else
      gcloud compute instances delete "$INSTANCE_NAME" \
        --project="$PROJECT" \
        --zone="$ZONE" \
        --delete-disks=all
    fi
    return 0
  fi

  # Stop keeps the VM's disks intact; it does not delete storage.
  if [[ "$status" == "TERMINATED" ]]; then
    echo "$INSTANCE_NAME is already TERMINATED."
    return 0
  fi

  gcloud compute instances stop "$INSTANCE_NAME" \
    --project="$PROJECT" \
    --zone="$ZONE"
}

update_instance() {
  local max_hours="$1"
  local status
  status="$(get_status)"

  if [[ -z "$status" ]]; then
    echo "$INSTANCE_NAME not found."
    return 0
  fi

  if [[ "$status" != "TERMINATED" ]]; then
    echo "Instance must be stopped (TERMINATED) before updating max run duration."
    return 1
  fi

  set_max_run_duration "$max_hours"
}

show_instance() {
  local status
  status="$(get_status)"

  if [[ -z "$status" ]]; then
    echo "$INSTANCE_NAME not found."
    return 0
  fi

  gcloud compute instances describe "$INSTANCE_NAME" \
    --project="$PROJECT" \
    --zone="$ZONE" \
    --format="table(name,zone.basename(),status,machineType.basename(),networkInterfaces[0].accessConfigs[0].natIP,disks[0].source.basename(),disks[0].diskSizeGb)"
  echo
  echo "Full details:"
  echo "gcloud compute instances describe $INSTANCE_NAME --project=$PROJECT --zone=$ZONE"
}

COMMAND="${1-}"
if [[ -z "$COMMAND" ]]; then
  usage
  exit 1
fi
shift || true

PORTS="$PORTS_DEFAULT"
DELETE_INSTANCE="false"
KEEP_DISKS="false"
MAX_RUN_HOURS="$MAX_RUN_HOURS_DEFAULT"
MAX_RUN_HOURS_SET="false"

while [[ $# -gt 0 ]]; do
  case "$1" in
    --ports)
      if [[ -z "${2-}" ]]; then
        echo "Missing value for --ports."
        exit 1
      fi
      PORTS="$2"
      shift 2
      ;;
    --delete)
      DELETE_INSTANCE="true"
      shift
      ;;
    --keep-disks)
      KEEP_DISKS="true"
      shift
      ;;
    --max-hours)
      if [[ -z "${2-}" ]]; then
        echo "Missing value for --max-hours."
        exit 1
      fi
      if [[ ! "$2" =~ ^[0-9]+$ ]] || [[ "$2" -le 0 ]]; then
        echo "--max-hours must be a positive integer."
        exit 1
      fi
      MAX_RUN_HOURS="$2"
      MAX_RUN_HOURS_SET="true"
      shift 2
      ;;
    --help|-h)
      usage
      exit 0
      ;;
    *)
      echo "Unknown option: $1"
      usage
      exit 1
      ;;
  esac
done

if [[ "$COMMAND" != "start" && "$PORTS" != "$PORTS_DEFAULT" ]]; then
  echo "--ports is only valid with the start command."
  exit 1
fi

if [[ "$COMMAND" != "stop" && ( "$DELETE_INSTANCE" == "true" || "$KEEP_DISKS" == "true" ) ]]; then
  echo "--delete/--keep-disks are only valid with the stop command."
  exit 1
fi

if [[ "$COMMAND" != "start" && "$COMMAND" != "update" && "$MAX_RUN_HOURS_SET" == "true" ]]; then
  echo "--max-hours is only valid with the start or update command."
  exit 1
fi

if [[ "$COMMAND" == "update" && "$MAX_RUN_HOURS_SET" != "true" ]]; then
  echo "--max-hours is required with the update command."
  exit 1
fi

case "$COMMAND" in
  start)
    start_instance "$PORTS" "$MAX_RUN_HOURS" "$MAX_RUN_HOURS_SET"
    ;;
  stop)
    stop_instance "$DELETE_INSTANCE" "$KEEP_DISKS"
    ;;
  show)
    show_instance
    ;;
  update)
    update_instance "$MAX_RUN_HOURS"
    ;;
  *)
    usage
    exit 1
    ;;
esac
