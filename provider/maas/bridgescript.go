// This file is auto generated. Edits will be lost.

package maas

const bridgeScriptBashFilename = "bridge-interface"
const bridgeScriptPythonFilename = "add-bridge.py"

const bridgeScriptPythonContent = `#!/usr/bin/env python

# Copyright 2015 Canonical Ltd.
# Licensed under the AGPLv3, see LICENCE file for details.

#
# This file has been and should be formatted using pyfmt(1).
#

from __future__ import print_function
import argparse
import re
import sys

# These options are to be removed from a sub-interface and applied to
# the new bridged interface.

BRIDGE_ONLY_OPTIONS = {'address', 'gateway', 'netmask', 'dns-nameservers', 'dns-search', 'dns-sortlist'}


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


class PhysicalInterface(object):
    """Represents a physical ('auto') interface."""

    def __init__(self, definition):
        self.name = definition.split()[1]

    def __str__(self):
        return self.name


class LogicalInterface(object):
    """Represents a logical ('iface') interface."""

    def __init__(self, definition, options=None):
        if not options:
            options = []
        _, self.name, self.family, self.method = definition.split()
        self.options = options
        self.is_loopback = self.method == 'loopback'
        self.is_bonded = [x for x in self.options if "bond-" in x]
        self.has_bond_master_option, self.bond_master_options = self.has_option(['bond-master'])
        self.is_alias = ":" in self.name
        self.is_vlan = [x for x in self.options if x.startswith("vlan-raw-device")]
        self.is_bridged, self.bridge_ports = self.has_option(['bridge_ports'])
        self.has_auto_stanza = None
        self.parent = None

    def __str__(self):
        return self.name

    def has_option(self, options):
        for o in self.options:
            words = o.split()
            ident = words[0]
            if ident in options:
                return True, words[1:]
        return False, []

    @classmethod
    def prune_options(cls, options, invalid_options):
        result = []
        for o in options:
            words = o.split()
            if words[0] not in invalid_options:
                result.append(o)
        return result

    # Returns an ordered set of stanzas to bridge this interface.
    def _bridge(self, prefix, bridge_name):
        if bridge_name is None:
            bridge_name = prefix + self.name
        # Note: the testing order here is significant.
        if self.is_loopback or self.is_bridged or self.has_bond_master_option:
            return self._bridge_unchanged()
        elif self.is_alias:
            if self.parent and self.parent.iface and self.parent.iface.is_bridged:
                # if we didn't change the parent interface
                # then we don't change the aliases neither.
                return self._bridge_unchanged()
            else:
                return self._bridge_alias(bridge_name)
        elif self.is_vlan:
            return self._bridge_vlan(bridge_name)
        elif self.is_bonded:
            return self._bridge_bond(bridge_name)
        else:
            return self._bridge_device(bridge_name)

    def _bridge_device(self, bridge_name):
        stanzas = []
        if self.has_auto_stanza:
            stanzas.append(AutoStanza(self.name))
        options = self.prune_options(self.options, BRIDGE_ONLY_OPTIONS)
        stanzas.append(IfaceStanza(self.name, self.family, "manual", options))
        stanzas.append(AutoStanza(bridge_name))
        options = list(self.options)
        options.append("bridge_ports {}".format(self.name))
        options = self.prune_options(options, ['mtu'])
        stanzas.append(IfaceStanza(bridge_name, self.family, self.method, options))
        return stanzas

    def _bridge_vlan(self, bridge_name):
        stanzas = []
        if self.has_auto_stanza:
            stanzas.append(AutoStanza(self.name))
        options = self.prune_options(self.options, BRIDGE_ONLY_OPTIONS)
        stanzas.append(IfaceStanza(self.name, self.family, "manual", options))
        stanzas.append(AutoStanza(bridge_name))
        options = list(self.options)
        options.append("bridge_ports {}".format(self.name))
        options = self.prune_options(options, ['mtu', 'vlan_id', 'vlan-raw-device'])
        stanzas.append(IfaceStanza(bridge_name, self.family, self.method, options))
        return stanzas

    def _bridge_alias(self, bridge_name):
        stanzas = []
        if self.has_auto_stanza:
            stanzas.append(AutoStanza(bridge_name))
        stanzas.append(IfaceStanza(bridge_name, self.family, self.method, list(self.options)))
        return stanzas

    def _bridge_bond(self, bridge_name):
        stanzas = []
        if self.has_auto_stanza:
            stanzas.append(AutoStanza(self.name))
        options = self.prune_options(self.options, BRIDGE_ONLY_OPTIONS)
        stanzas.append(IfaceStanza(self.name, self.family, "manual", options))
        stanzas.append(AutoStanza(bridge_name))
        options = [x for x in self.options if not x.startswith("bond")]
        options = self.prune_options(options, ['mtu'])
        options.append("bridge_ports {}".format(self.name))
        stanzas.append(IfaceStanza(bridge_name, self.family, self.method, options))
        return stanzas

    def _bridge_unchanged(self):
        stanzas = []
        if self.has_auto_stanza:
            stanzas.append(AutoStanza(self.name))
        stanzas.append(IfaceStanza(self.name, self.family, self.method, list(self.options)))
        return stanzas


class Stanza(object):
    """Represents one stanza together with all of its options."""

    def __init__(self, definition, options=None):
        if not options:
            options = []
        self.definition = definition
        self.options = options
        self.is_logical_interface = definition.startswith('iface ')
        self.is_physical_interface = definition.startswith('auto ')
        self.iface = None
        self.phy = None
        if self.is_logical_interface:
            self.iface = LogicalInterface(definition, self.options)
        if self.is_physical_interface:
            self.phy = PhysicalInterface(definition)

    def __str__(self):
        return self.definition


class NetworkInterfaceParser(object):
    """Parse a network interface file into a set of stanzas."""

    @classmethod
    def is_stanza(cls, s):
        return re.match(r'^(iface|mapping|auto|allow-|source)', s)

    def __init__(self, filename):
        self._stanzas = []
        with open(filename, 'r') as f:
            lines = f.readlines()
        line_iterator = SeekableIterator(lines)
        for line in line_iterator:
            if self.is_stanza(line):
                stanza = self._parse_stanza(line, line_iterator)
                self._stanzas.append(stanza)
        physical_interfaces = self._physical_interfaces()
        for s in self._stanzas:
            if not s.is_logical_interface:
                continue
            s.iface.has_auto_stanza = s.iface.name in physical_interfaces

        self._connect_aliases()
        self._bridged_interfaces = self._find_bridged_ifaces()

    def _parse_stanza(self, stanza_line, iterable):
        stanza_options = []
        for line in iterable:
            line = line.strip()
            if line.startswith('#') or line == "":
                continue
            if self.is_stanza(line):
                iterable.seek(-1, True)
                break
            stanza_options.append(line)
        return Stanza(stanza_line.strip(), stanza_options)

    def stanzas(self):
        return [x for x in self._stanzas]

    def _connect_aliases(self):
        """Set a reference in the alias interfaces to its related interface"""
        ifaces = {}
        aliases = []
        for stanza in self._stanzas:
            if stanza.iface is None:
                continue

            if stanza.iface.is_alias:
                aliases.append(stanza)
            else:
                ifaces[stanza.iface.name] = stanza

        for alias in aliases:
            parent_name = alias.iface.name.split(':')[0]
            if parent_name in ifaces:
                alias.iface.parent = ifaces[parent_name]

    def _find_bridged_ifaces(self):
        bridged_ifaces = {}
        for stanza in self._stanzas:
            if not stanza.is_logical_interface:
                continue
            if stanza.iface.is_bridged:
                bridged_ifaces[stanza.iface.name] = stanza.iface
        return bridged_ifaces

    def _physical_interfaces(self):
        return {x.phy.name: x.phy for x in [y for y in self._stanzas if y.is_physical_interface]}

    def __iter__(self):  # class iter
        for s in self._stanzas:
            yield s

    def _is_already_bridged(self, name, bridge_port):
        iface = self._bridged_interfaces.get(name, None)
        if iface:
            return bridge_port in iface.bridge_ports
        return False

    def bridge(self, interface_names_to_bridge, bridge_prefix, bridge_name):
        bridged_stanzas = []
        for s in self.stanzas():
            if s.is_logical_interface:
                if s.iface.name not in interface_names_to_bridge:
                    if s.iface.has_auto_stanza:
                        bridged_stanzas.append(AutoStanza(s.iface.name))
                    bridged_stanzas.append(s)
                else:
                    existing_bridge_name = bridge_prefix + s.iface.name
                    if self._is_already_bridged(existing_bridge_name, s.iface.name):
                        if s.iface.has_auto_stanza:
                            bridged_stanzas.append(AutoStanza(s.iface.name))
                        bridged_stanzas.append(s)
                    else:
                        bridged_stanzas.extend(s.iface._bridge(bridge_prefix, bridge_name))
            elif not s.is_physical_interface:
                bridged_stanzas.append(s)
        return bridged_stanzas


def uniq_append(dst, src):
    for x in src:
        if x not in dst:
            dst.append(x)
    return dst


def IfaceStanza(name, family, method, options):
    """Convenience function to create a new "iface" stanza.

Maintains original options order but removes duplicates with the
exception of 'dns-*' options which are normalised as required by
resolvconf(8) and all the dns-* options are moved to the end.

    """

    dns_search = []
    dns_nameserver = []
    dns_sortlist = []
    unique_options = []

    for o in options:
        words = o.split()
        ident = words[0]
        if ident == "dns-nameservers":
            dns_nameserver = uniq_append(dns_nameserver, words[1:])
        elif ident == "dns-search":
            dns_search = uniq_append(dns_search, words[1:])
        elif ident == "dns-sortlist":
            dns_sortlist = uniq_append(dns_sortlist, words[1:])
        elif o not in unique_options:
            unique_options.append(o)

    if dns_nameserver:
        option = "dns-nameservers " + " ".join(dns_nameserver)
        unique_options.append(option)

    if dns_search:
        option = "dns-search " + " ".join(dns_search)
        unique_options.append(option)

    if dns_sortlist:
        option = "dns-sortlist " + " ".join(dns_sortlist)
        unique_options.append(option)

    return Stanza("iface {} {} {}".format(name, family, method), unique_options)


def AutoStanza(name):
    # Convenience function to create a new "auto" stanza.
    return Stanza("auto {}".format(name))


def print_stanza(s, stream=sys.stdout):
    print(s.definition, file=stream)
    for o in s.options:
        print("   ", o, file=stream)


def print_stanzas(stanzas, stream=sys.stdout):
    n = len(stanzas)
    for i, stanza in enumerate(stanzas):
        print_stanza(stanza, stream)
        if stanza.is_logical_interface and i + 1 < n:
            print(file=stream)


def arg_parser():
    parser = argparse.ArgumentParser(formatter_class=argparse.ArgumentDefaultsHelpFormatter)
    parser.add_argument('--bridge-prefix', help="bridge prefix", type=str, required=False, default='br-')
    parser.add_argument('--bridge-name', help="bridge name", type=str, required=False)
    parser.add_argument('--output', help="output file", type=str, required=False)
    parser.add_argument('filename', type=str)
    parser.add_argument('interfaces', nargs=argparse.REMAINDER)
    return parser


def main(args):
    if len(args.interfaces) == 0:
        sys.stderr.write("error: no interfaces specified\n")
        exit(2)
    if args.bridge_name and len(args.interfaces) > 1:
        sys.stderr.write("error: cannot use single bridge name '{}' against multiple interface names\n".format(args.bridge_name))
        exit(1)
    parser = NetworkInterfaceParser(args.filename)
    stanzas = parser.bridge(args.interfaces, args.bridge_prefix, args.bridge_name)
    output = sys.stdout
    if args.output:
        output = open(args.output, "w")
    print_stanzas(stanzas, output)

# This script renders an interfaces(5) file to create bridges for named
# interfaces. Activation of the bridge is not handled by this script.

if __name__ == '__main__':
    main(arg_parser().parse_args())
`

const bridgeScriptBashContent = `#!/bin/bash

passwd -d ubuntu

set -u

PROGNAME=$(basename $0)
SCRIPTPATH="$(cd $(dirname "${BASH_SOURCE[0]}") && pwd -P)"

: ${BACKUP_INPUT_FILE_OPTIONS:=--backup=numbered}
: ${BRIDGE_PREFIX:="br-"}
: ${CHECK_PACKAGES_INSTALLED:=1}
: ${DEBUG:=0}
: ${DRY_RUN:=}
: ${IFUPDOWN_VERBOSE:=}
: ${NEW_ENI_FILE:=}
: ${PREFERRED_PYTHON_BINARY:=}
: ${LOG_STATE:=1}

[ $DEBUG -eq 1 ] && set -x

log_state() {
    [ $LOG_STATE -eq 1 ] || return 0
    local msg=$1
    local filename=$2
    echo "START: $msg"
    cat $filename
    ip link show up
    brctl show
    ip route
    echo "END: $msg"
    return 0
}

if [ $CHECK_PACKAGES_INSTALLED -eq 1 ]; then
    if ! [ -x "$(command -v brctl)" ]; then
	echo 'error: brctl is not installed; please install package bridge-utils' >&2
	exit 1
    fi
    if ! [ -x "$(command -v ifenslave)" ]; then
	echo 'error: ifenslave is not installed; please install package ifenslave' >&2
	exit 1
    fi
fi

if [ $# -lt 2 ]; then
    echo "usage: $PROGNAME: <interface-file> <interface>..."
    exit 2
fi

# For ubuntu series < xenial we prefer python2 over python3 as we
# don't want to invalidate lots of testing against known cloud-image
# contents.
#
# A summary of Ubuntu releases and python inclusion in the default
# install of Ubuntu Server is as follows:
#
# 12.04 precise:  python 2 (2.7.3)
# 14.04 trusty:   python 2 (2.7.5) and python3 (3.4.0)
# 14.10 utopic:   python 2 (2.7.8) and python3 (3.4.2)
# 15.04 vivid:    python 2 (2.7.9) and python3 (3.4.3)
# 15.10 wily:     python 2 (2.7.9) and python3 (3.4.3)
# 16.04 xenial:   python 3 only (3.5.1)
#
# going forward:  python 3 only

if [ -z "$PREFERRED_PYTHON_BINARY" ]; then
    if [ -x "$(command -v python2)" ]; then
	PREFERRED_PYTHON_BINARY=/usr/bin/python2
    elif [ -x "$(command -v python3)" ]; then
	PREFERRED_PYTHON_BINARY=/usr/bin/python3
    elif [ -x "$(command -v python)" ]; then
	PREFERRED_PYTHON_BINARY=/usr/bin/python
    fi
fi

if ! [ -x "$(command -v $PREFERRED_PYTHON_BINARY)" ]; then
    echo "error: $PREFERRED_PYTHON_BINARY not executable, or not a command" >&2
    exit 1
fi

orig_file="$1"; shift

if [ -z "$NEW_ENI_FILE" ]; then
    NEW_ENI_FILE=$(mktemp -t)
#    trap 'rm -f "$NEW_ENI_FILE"' EXIT
fi

$DRY_RUN $PREFERRED_PYTHON_BINARY "$SCRIPTPATH/add-bridge.py" --output="$NEW_ENI_FILE" --bridge-prefix="$BRIDGE_PREFIX" "$orig_file" "$@"

if [ $? -ne 0 ]; then
    echo "error: failed to add bridge stanzas to $orig_file"
    exit 1
fi

# Any error from here should be immediately fatal.
# set -e

if cmp -s "$orig_file" "$NEW_ENI_FILE"; then
    echo "nothing to bridge, or already bridged."
    exit 0
fi

log_state "**** Original configuration" $orig_file
ifquery=$(ifquery --exclude=lo --interfaces="$orig_file" --list)
$DRY_RUN ifdown $IFUPDOWN_VERBOSE --exclude=lo --interfaces="$orig_file" $ifquery

if grep -q 'bond-' "$orig_file"; then
    echo "sleeping to work around https://bugs.launchpad.net/ubuntu/+source/ifenslave/+bug/1269921"
    echo "sleeping to work around https://bugs.launchpad.net/juju/+bug/1594855"
    $DRY_RUN sleep ${BOND_SLEEP_DURATION:-10}
fi

declare -a bridge_ifnames=()

for i in "$@"; do
    bridge_ifnames=(${bridge_ifnames[@]+"${bridge_ifnames[@]}"} "${BRIDGE_PREFIX}${i}")
done

$DRY_RUN chmod 644 "$NEW_ENI_FILE"
$DRY_RUN cp $BACKUP_INPUT_FILE_OPTIONS "$NEW_ENI_FILE" "$orig_file"
$DRY_RUN ifup $IFUPDOWN_VERBOSE --exclude=lo --interfaces="$orig_file" $ifquery
`
