#!/usr/bin/env python

from __future__ import print_function
import re
import sys
import subprocess as proc
import argparse
import uuid
import os.path
import shutil
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
        return copy(self._options)

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
            if line == "\n":
                continue
            if line.startswith("#"):
                continue
            if self.is_stanza(line):
                iterable.seek(-1, True)
                break
            if line.strip() != "":
                stanza_options.append(line.strip())
        return Stanza(stanza_line.strip(), stanza_options)

    def stanzas(self):
        for s in self._stanzas:
            yield s


def print_stanza(s, stream=sys.stdout):
    print(s.definition(), file=stream)
    for o in s.options():
        print("   ", o, file=stream)
    if s.is_logical_interface():
        print(file=stream)

def print_stanzas(stanzas, stream=sys.stdout):
    for s in stanzas:
        print_stanza(s, stream)

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
                options.append("bridge_ports {}".format(primary_nic))
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
                options.append("bridge_ports {}".format(primary_nic))
                stanzas.append(Stanza(" ".join(words), options))
            continue

        # Aliases, hence the 'eth0' in 'auto eth0:1'.

        if primary_nic in s.definition():
            definition = s.definition().replace(primary_nic, bridge_name)
            stanzas.append(Stanza(definition, s.options()))

    return stanzas

def shcmd(args):
    return proc.check_output(args, shell=True).strip().decode("utf-8")

def get_gateway(ver='-4'):
    return shcmd("ip {} route list exact default | cut -d' ' -f3".format(ver))

def get_primary_nic(ver='-4'):
    return shcmd("ip {} route list exact default | cut -d' ' -f5".format(ver))

def is_nic_bonded(name):
    bonding_masters="/sys/class/net/bonding_masters"
    # Checking for existence is not racy for what we're trying to do.
    if not os.path.isfile(bonding_masters):
        return False
    print(type(name))
    return shcmd('grep {} {}'.format(str(name), bonding_masters)) == name

parser = argparse.ArgumentParser()

parser.add_argument('--filename',
                    help='filename to re-render',
                    required=False,
                    default="/etc/network/interfaces")

parser.add_argument('--dry-run',
                    help='print re-rendered interface file',
                    action='store_true',
                    required=False)

parser.add_argument('--bridge-name',
                    help="bridge name",
                    required=False,
                    default='juju-br0')

parser.add_argument('--primary-nic',
                    help="primary NIC name",
                    type=str,
                    required=False)

parser.add_argument('--primary-nic-is-bonded',
                    help="primary NIC is bonded",
                    action='store_true',
                    required=False)

args = parser.parse_args()

if not args.primary_nic:
    args.primary_nic = get_primary_nic()
    args.primary_nic_is_bonded = is_nic_bonded(args.primary_nic)

print(args)

bridged_stanzas = render(args.filename,
                         args.bridge_name,
                         args.primary_nic,
                         args.primary_nic_is_bonded)

if args.dry_run:
    print_stanzas(bridged_stanzas)
    sys.exit(0)


#
# Take a one-time copy of the original file.
#
orig_filename = "{}-original".format(args.filename)
if not os.path.isfile(orig_filename):
    shutil.copy(args.filename, orig_filename)

#
# Write the new stanzas to a temporary file.
#
tmp_filename = "{}-juju-{}".format(args.filename, str(uuid.uuid4()))

with open(tmp_filename, 'w') as f:
    print(tmp_filename)
    print_stanzas(bridged_stanzas, f)
    f.close()

def check_shcmd(args):
    print(args)
    proc.check_call(args, shell=True)

check_shcmd("cat {}".format(tmp_filename))
check_shcmd("ifdown -v -i {} {}".format(args.filename, args.primary_nic))
check_shcmd("/etc/init.d/networking stop || true")
check_shcmd("cp {} {}".format(tmp_filename, args.filename))
check_shcmd("/etc/init.d/networking start || true")
check_shcmd("ifup -a -v")
check_shcmd("/etc/init.d/networking restart || true")
