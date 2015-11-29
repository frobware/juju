# Print message with function and line number info from perspective of
# the caller and exit with status code 1.
fatal()
{
    local message=$1
    echo "${BASH_SOURCE[1]}: line ${BASH_LINENO[0]}: ${FUNCNAME[1]}: fatal error: ${message:-'died'}." >&2
    exit 1
}

modify_network_config() {
    [ $# -ge 3 ] || return 1
    [ -z "$1" ] || [ -z "$2" ] || [ -z "$3" ] && return 1
    local filename=$1
    local primary_nic=$2
    local bridge=$3
    local primary_nic_is_bonded=$4
    echo "$python_script" > /tmp/juju-add-bridge.py
    if [ $primary_nic_is_bonded -eq 1 ]; then
	python /tmp/juju-add-bridge.py --filename "$filename" --primary-nic "$primary_nic" --bridge-name "$bridge" --primary-nic-is-bonded
    else
	python /tmp/juju-add-bridge.py --filename "$filename" --primary-nic "$primary_nic" --bridge-name "$bridge"
    fi
    return $?
}

# Discover the needed IPv4/IPv6 configuration for $BRIDGE (if any).
#
# Arguments:
#
#   $1: the first argument to ip(1) (e.g. "-6" or "-4")
#
# Outputs the discovered default gateway and primary NIC, separated
# with a space, if they could be discovered. The output is undefined
# otherwise.
get_gateway() {
    ip "$1" route list exact default | cut -d' ' -f3
}

get_primary_nic() {
    ip "$1" route list exact default | cut -d' ' -f5
}

# Display route table contents (IPv4 and IPv6), network devices, all
# configured IPv4 and IPv6 addresses, and the contents of $CONFIGFILE
# for diagnosing connectivity issues.
dump_network_config() {
    # Note: Use the simplest command and options to be compatible with
    # precise.

    echo "======================================================="
    echo "${1} Network Configuration"
    echo "======================================================="
    echo
    cat "$CONFIGFILE"

    echo "-------------------------------------------------------"
    echo "Route table contents:"
    echo "-------------------------------------------------------"
    ip route show
    echo

    echo "-------------------------------------------------------"
    echo "Network devices:"
    echo "-------------------------------------------------------"
    ifconfig -a
}
