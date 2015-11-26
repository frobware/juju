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

    def next(self):             # Python 2
        try:
            value = self.iterable[self.index]
            self.index += 1
            return value
        except IndexError:
            raise StopIteration

    def __next__(self):         # Python 3
        return self.next()

    def seek(self, n, relative=False):
        if relative:
            self.index += n
        else:
            self.index = n
        if self.index < 0 or self.index >= len(self.iterable):
            raise IndexError

class Stanza(object):
    def __init__(self, definition, options = []):
        self._definition = definition
        self._options = options

    def __str__(self):
        return self._definition

    def isPhysicalInterface(self):
        return self._definition.startswith("auto ")

    def isLogicalInterface(self):
        return self._definition.startswith("iface ")

    def options(self):
        return copy(self._options)

    def definition(self):
        return self._definition

    def __str__(self):
        return self._definition

    def ifaceName(self):
        if self.isPhysicalInterface():
            return self._definition.split()[1]
        if self.isLogicalInterface():
            return self._definition.split()[1]
        return None

class EtcNetworkInterfaceParser(object):
    @classmethod
    def is_stanza(cls, s):
        return re.match(r'^(iface|mapping|auto|allow-|source|dns-)', s)

    def __init__(self, filename):
        self._filename = filename
        self._stanzas = []
        with open(filename) as f:
            self._lines = f.readlines()
        lineiter = SeekableIterator(self._lines)
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

def str2bool(v):
    return str(v).lower() in ("true", "1")

def printStanza(s):
    print(s)
    for o in s.options():
        print("   ", o)
    print
    if s.isLogicalInterface(): print()

def main(args):
    filename = args[0]
    primaryNic = args[1]
    bridgeName = args[2]
    bonded = str2bool(args[3])
    stanzas = []

    for s in EtcNetworkInterfaceParser(filename).stanzas():
        if not s.isLogicalInterface() and not s.isPhysicalInterface():
            stanzas.append(s)
            continue

        if primaryNic != s.ifaceName() and primaryNic not in s.ifaceName():
            stanzas.append(s)
            continue

        if bonded:
            if s.isPhysicalInterface():
                stanzas.append(s)
            else:
                words = s.definition().split()
                words[3] = "manual"
                printStanza(s)
                stanzas.append(Stanza(" ".join(words), s.options()))

                # new auto <bridgeName>
                stanzas.append(Stanza("auto {}".format(bridgeName)))

                # new iface <bridgeName> ...
                words = s.definition().split()
                words[1] = bridgeName
                options = [ x for x in s.options() if not x.startswith("bond") ]
                options.append("bridge_ports {}".format(primaryNic))
                stanzas.append(Stanza(" ".join(words), options))
            continue

        if primaryNic == s.ifaceName():
            if s.isPhysicalInterface():
                # The net change:
                #   auto eth0
                # to:
                #   auto $bridgeName
                words = s.definition().split()
                words[1] = bridgeName
                stanzas.append(Stanza(" ".join(words)))
            else:
                # The net change is:
                #   auto eth0
                #   iface eth0 inet <config>
                # to:
                #   iface eth0 inet manual
                #
                #   auto $bridgeName
                #   iface $bridgeName inet <config>
                words = s.definition().split()
                words[3] = "manual"
                autoStanza = stanzas.pop()
                stanzas.append(Stanza(" ".join(words)))
                stanzas.append(autoStanza)
                # Replace existing iface line with new $bridgeName
                words = s.definition().split()
                words[1] = bridgeName
                options = s.options()
                # And add the bridge ports to the existing options
                options.append("bridge_ports {}".format(primaryNic))
                stanzas.append(Stanza(" ".join(words), options))
            continue

        # Aliases, which can be separated by '.' or ':'.

        alias_re = re.compile(r'{}.(?P<num>\d+)'.format(primaryNic))

        if primaryNic in s.definition():
            match = alias_re.search(s.definition())
            stanzas.append(Stanza(s.definition().replace(primaryNic, bridgeName), s.options()))

    for s in stanzas:
        printStanza(s)

if len(sys.argv) < 5:
    print("usage: <filename> <primary-nic> <bridge-name> <is-bonded>")
    sys.exit(1)

main(sys.argv[1:])
