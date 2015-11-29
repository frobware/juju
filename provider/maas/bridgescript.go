// This file is auto generated. Edits will be lost.

package maas

//go:generate make

const bridgeScriptCommon = `# Print message with function and line number info from perspective of
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
python_script=$(cat <<'PYTHON_SCRIPT'
#!/usr/bin/env python

from __future__ import print_function

import argparse
import re
import subprocess
import sys
from copy import copy


class SeekableIterator(object):
    """An iterator that supports relative seeking."""

    def __init__(self, iterable):
        self.iterable = iterable
        self.index = 0

    def __iter__(self):
        return self

    def next(self):  # Python 2
        try:
            value = self.iterable[self.index]
            self.index += 1
            return value
        except IndexError:
            raise StopIteration

    def __next__(self):  # Python 3
        return self.next()

    def seek(self, n, relative=False):
        if relative:
            self.index += n
        else:
            self.index = n
        if self.index < 0 or self.index >= len(self.iterable):
            raise IndexError


class Stanza(object):
    def __init__(self, definition, options=None):
        if not options:
            options = []
        self._definition = definition
        self._options = options

    def is_physical_interface(self):
        return self._definition.startswith('auto ')

    def is_logical_interface(self):
        return self._definition.startswith('iface ')

    def options(self):
        return self._options

    def definition(self):
        return self._definition

    def interface_name(self):
        if self.is_physical_interface():
            return self._definition.split()[1]
        if self.is_logical_interface():
            return self._definition.split()[1]
        return None


class NetworkInterfaceParser(object):
    @classmethod
    def is_stanza(cls, s):
        return re.match(r'^(iface|mapping|auto|allow-|source|dns-)', s)

    def __init__(self, filename):
        self._filename = filename
        self._stanzas = []
        with open(filename) as f:
            lines = f.readlines()
        line_iterator = SeekableIterator(lines)
        for line in line_iterator:
            if self.is_stanza(line):
                stanza = self._parse_stanza(line, line_iterator)
                self._stanzas.append(stanza)

    def _parse_stanza(self, stanza_line, iterable):
        stanza_options = []
        for line in iterable:
            line = line.strip()
            if line.startswith('#'):
                continue
            if self.is_stanza(line):
                iterable.seek(-1, True)
                break
            if line.strip() != "":
                stanza_options.append(line.strip())
        return Stanza(stanza_line.strip(), stanza_options)

    def stanzas(self):
        for s in copy(self._stanzas):
            yield s


def print_stanza(s, stream=sys.stdout):
    print(s.definition(), file=stream)
    for o in s.options():
        print("   ", o, file=stream)


def print_stanzas(stanzas, stream=sys.stdout):
    n = len(stanzas)
    for i, s in enumerate(stanzas):
        print_stanza(s, stream)
        if s.is_logical_interface() and i + 1 < n:
            print(file=stream)


def render(filename, bridge_name, primary_nic, bonded):
    stanzas = []

    for s in NetworkInterfaceParser(filename).stanzas():
        if not s.is_logical_interface() and not s.is_physical_interface():
            stanzas.append(s)
            continue

        if primary_nic != s.interface_name() and \
                        primary_nic not in s.interface_name():
            stanzas.append(s)
            continue

        if bonded:
            if s.is_physical_interface():
                stanzas.append(s)
            else:
                words = s.definition().split()
                words[3] = "manual"
                stanzas.append(Stanza(" ".join(words), s.options()))

                # new auto <bridge_name>
                stanzas.append(Stanza("auto {}".format(bridge_name)))

                # new iface <bridge_name> ...
                words = s.definition().split()
                words[1] = bridge_name
                options = [x for x in s.options() if not x.startswith("bond")]
                options.insert(0, "bridge_ports {}".format(primary_nic))
                stanzas.append(Stanza(" ".join(words), options))
            continue

        if primary_nic == s.interface_name():
            if s.is_physical_interface():
                # The net change:
                #   auto eth0
                # to:
                #   auto <bridge_name>
                words = s.definition().split()
                words[1] = bridge_name
                stanzas.append(Stanza(" ".join(words)))
            else:
                # The net change is:
                #   auto eth0
                #   iface eth0 inet <config>
                # to:
                #   iface eth0 inet manual
                #
                #   auto <bridge_name>
                #   iface <bridge_name> inet <config>
                words = s.definition().split()
                words[3] = "manual"
                last_stanza = stanzas.pop()
                stanzas.append(Stanza(" ".join(words)))
                stanzas.append(last_stanza)
                # Replace existing 'iface' line with new <bridge_name>
                words = s.definition().split()
                words[1] = bridge_name
                options = s.options()
                options.insert(0, "bridge_ports {}".format(primary_nic))
                stanzas.append(Stanza(" ".join(words), options))
            continue

        # Aliases, hence the 'eth0' in 'auto eth0:1'.

        if primary_nic in s.definition():
            definition = s.definition().replace(primary_nic, bridge_name)
            stanzas.append(Stanza(definition, s.options()))

    return stanzas


def check_shoutput(args):
    return subprocess.check_output(args, shell=True).strip().decode("utf-8")


parser = argparse.ArgumentParser()

parser.add_argument('--filename',
                    help='filename to re-render',
                    required=False,
                    default="/etc/network/interfaces")

parser.add_argument('--dry-run',
                    help='print re-rendered interface file',
                    action='store_true',
                    required=False)

parser.add_argument('--render-only',
                    help='render interface file only; no network restart',
                    action='store_true',
                    required=False)

parser.add_argument('--bridge-name',
                    help="bridge name",
                    required=False,
                    default='juju-br0')

parser.add_argument('--primary-nic',
                    help="primary NIC name",
                    type=str,
                    required=True)

parser.add_argument('--primary-nic-is-bonded',
                    help="primary NIC is bonded",
                    action='store_true',
                    required=False)

args = parser.parse_args()

bridged_stanzas = render(args.filename,
                         args.bridge_name,
                         args.primary_nic,
                         args.primary_nic_is_bonded)

if args.dry_run:
    print_stanzas(bridged_stanzas)
    sys.exit(0)

with open(args.filename, 'w') as f:
    print_stanzas(bridged_stanzas, f)
    f.close()

sys.exit(0)
PYTHON_SCRIPT
)`
const bridgeScriptMain = bridgeScriptCommon + `
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
`
