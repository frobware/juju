: ${CONFIGFILE:={{.Config}}}
: ${BRIDGE:={{.Bridge}}}

set -u

main() {
    local orig_config_file="$CONFIGFILE"
    local new_config_file="${CONFIGFILE}-juju"

    # In case we already created the bridge, don't do it again.
    grep -q "iface $BRIDGE inet" "$orig_config_file" && return 0

    # We're going to do all our mods against a new file.
    cp -a "$CONFIGFILE" "$new_config_file" || fatal "cp failed"

    # Take a one-time reference of the original file
    if [ ! -f "${CONFIGFILE}-orig" ]; then
	cp -a "$CONFIGFILE" "${CONFIGFILE}-orig" || fatal "cp failed"
    fi

    # determine whether to configure $bridge for ipv4, ipv6(TODO), or both.
    local ipv4_gateway=$(get_gateway -4)
    local ipv4_primary_nic=$(get_primary_nic -4)

    echo "ipv4 gateway = $ipv4_gateway"
    echo "ipv4 primary nic = $ipv4_primary_nic"

    if [ -z "$ipv4_gateway" ]; then
	fatal "cannot discover ipv4 gateway"
    fi

    local bonding_masters_file=/sys/class/net/bonding_masters
    local ipv4_primary_nic_is_bonded=0

    if [ -f $bonding_masters_file ] && grep $ipv4_primary_nic $bonding_masters_file; then
	ipv4_primary_nic_is_bonded=1
    fi

    if [ -n "$ipv4_gateway" ]; then
	modify_network_config "$new_config_file" "$ipv4_primary_nic" "$BRIDGE" $ipv4_primary_nic_is_bonded
	if [ $? -ne 0 ]; then
	    fatal "failed to add $BRIDGE to $new_config_file"
	fi
    fi

    if ! ip link list "$BRIDGE"; then
	ip link add dev "$ipv4_primary_nic" name "$BRIDGE" type bridge
	if [ $? -ne 0 ]; then
	    fatal "cannot add $BRIDGE bridge"
	fi
    fi

    ifdown --exclude=lo $ipv4_primary_nic
    cp "$new_config_file" "$orig_config_file" || fatal "cp failed"
    ifup -a
    return 0
}

passwd -d ubuntu
trap 'dump_network_config "Active"' EXIT
dump_network_config "Current"
main
