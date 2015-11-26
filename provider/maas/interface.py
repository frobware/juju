#!/usr/bin/env python

from __future__ import print_function
import re
import sys
from copy import copy


class SeekableIterator(object):
    """An iterator that supports seeking."""

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
        return self._definition.startswith("auto ")

    def is_logical_interface(self):
        return self._definition.startswith("iface ")

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

    def __str__(self):
        return self._definition


class EtcNetworkInterfaceParser(object):
    @classmethod
    def is_stanza(cls, s):
        return re.match(r'^(iface|mapping|auto|allow-|source|dns-)', s)

    def __init__(self, filename):
        self._filename = filename
        self._stanzas = []
        with open(filename) as f:
            lines = f.readlines()
        lineiter = SeekableIterator(lines)
        for line in lineiter:
            if self.is_stanza(line):
                stanza = self._parse_stanza(line, lineiter)
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


def print_stanza(s):
    print(s)
    for o in s.options():
        print("   ", o)
    if s.is_logical_interface():
        print()


def main(args):
    filename = args[0]
    primary_nic = args[1]
    bridge_name = args[2]
    bonded = str(args[3]).lower() in ('true', '1')
    stanzas = []

    for s in EtcNetworkInterfaceParser(filename).stanzas():
        if not s.is_logical_interface() and not s.is_physical_interface():
            stanzas.append(s)
            continue

        if primary_nic != s.interface_name() and primary_nic not in s.interface_name():
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
                #   auto $bridge_name
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
                #   auto $bridge_name
                #   iface $bridge_name inet <config>
                words = s.definition().split()
                words[3] = "manual"
                last_stanza = stanzas.pop()
                stanzas.append(Stanza(" ".join(words)))
                stanzas.append(last_stanza)
                # Replace existing 'iface' line with new $bridge_name
                words = s.definition().split()
                words[1] = bridge_name
                options = s.options()
                options.append("bridge_ports {}".format(primary_nic))
                stanzas.append(Stanza(" ".join(words), options))
            continue

        # Aliases, hence the 'eth0' in 'eth0:1'.

        if primary_nic in s.definition():
            stanzas.append(Stanza(s.definition().replace(primary_nic, bridge_name), s.options()))

    for s in stanzas:
        print_stanza(s)


if len(sys.argv) < 5:
    print("usage: <filename> <primary-nic> <bridge-name> <is-bonded>")
    sys.exit(1)

main(sys.argv[1:])
