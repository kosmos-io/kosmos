#!/bin/bash

filename="nodes.txt"
readarray -t globalnodes < "$filename"

function updateState() {
  local nodename="$1"
  local state="$2"
  kubectl patch globalnodes $nodename -p '{"spec": {"state": "'$state'"}}' --type=merge
}

function updateNodeState() {
    local nodename="$1"
    local state="$2"
    kubectl patch node $nodename -p '{"metadata": {"labels": {"kosmos-io/state": "'$state'"}}}'
}

function uncordon() {
   local nodename="$1"
   kubectl uncordon $nodename
   kubectl taint nodes $nodename node.kosmos.io/unschedulable-
}


# Update the state of the global nodes
function  free_globalnodes() {
  local globalnode="$1"
  updateState "$globalnode" "free"
  updateNodeState "$globalnode" "free"
}



# Update the state of the global nodes
function  reserved_globalnodes() {
  local globalnode="$1"
  updateState "$globalnode" "reserved"
  updateNodeState "$globalnode" "reserved"
  uncordon  "$globalnode"
}


# Function to display progress bar
show_progress() {
    local progress=$1
    local total=$2
    local width=$3

    # Calculate percentage
    local percent=$((progress * 100 / total))
    local num_hashes=$((percent * width / 100))

    # Generate progress bar
    local bar="["
    for ((i = 0; i < width; i++)); do
        if ((i < num_hashes)); then
            bar+="#"
        else
            bar+=" "
        fi
    done
    bar+="]"

    # Print progress bar with percentage
    printf "\rProgress: %s %d%%" "$bar" "$percent"
}

# Total steps for the task
total_steps=${#globalnodes[@]}
# Width of the progress bar
bar_width=50

function free() {
    # Simulate a task by looping through steps
    for ((step = 1; step <= total_steps; step++)); do
        # Simulate work with sleep
        index=$((step - 1))
        free_globalnodes ${globalnodes[index]}

        # Update progress bar
        show_progress $step $total_steps $bar_width
    done

    # Print a new line after the progress bar completes
    echo
}

function reserved() {
    # Simulate a task by looping through steps
    for ((step = 1; step <= total_steps; step++)); do
        # Simulate work with sleep
        index=$((step - 1))
        reserved_globalnodes ${globalnodes[index]}

        # Update progress bar
        show_progress $step $total_steps $bar_width
    done

    # Print a new line after the progress bar completes
    echo
}


# See how we were called.
case "$1" in
    free)
    free
    ;;
    reserved)
    reserved
    ;;
    *)
    echo $"usage: $0 free|reserved"
    exit 1
esac