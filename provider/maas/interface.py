#!/usr/bin/env python

from __future__ import print_function

import re
import sys

class SeekableIterator(object):
    """An iterator that supports seeking backwards or forwards."""

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
    def __init__(self, ident, options):
        self._ident = ident
        self._options = options

    def __str__(self):
        return self._ident

    def isPhysicalInterface(self):
        return self._ident.startswith("auto ")

    def isLogicalInterface(self):
        return self._ident.startswith("iface ")

    def options(self):
        return self._options

    def ident(self):
        return self._ident

    def ifaceName(self):
        if self.isPhysicalInterface():
            return self._ident.split()[1]
        if self.isLogicalInterface():
            return self._ident.split()[1]
        return None

class ENIParser(object):
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

    def __str__(self):
        return self._filename

    def dump(self):
        for s in self.stanzas():
            print(s)
            for line in s.options():
                print("    {}".format(line))

    def stanzas(self):
        return self._stanzas

def needsBridgeWork(stanza):
    return stanza.isLogicalInterface() or stanza.isPhysicalInterface()

ifaceName = "eth0"
bridgeName = "juju-br0"
bonded = False

def printStanza(s):
    print(s)
    for o in s.options():
        print("   ", o)
    print
    if s.isLogicalInterface(): print()

def main(filename):
    eni = ENIParser(filename)
    stanzas = eni.stanzas()
    newStanzas = []
    for s in stanzas:
        if not s.isLogicalInterface() and not s.isPhysicalInterface():
            newStanzas.append(s)
            continue

        if ifaceName != s.ifaceName() and ifaceName not in s.ifaceName():
            newStanzas.append(s)
            continue

        if bonded:
            newStanzas.append(s)
            continue

        if ifaceName == s.ifaceName():
            if s.isPhysicalInterface():
                # This changes:
                #   auto eth0
                # to:
                #   auto $bridgeName
                words = s.ident().split()
                words[1] = bridgeName
                newStanzas.append(Stanza(" ".join(words), []))
            else:
                # The net change is:
                #   auto eth0
                #   iface eth0 inet <config>
                # to:
                #   iface eth0 inet manual
                #
                #   auto $bridgeName
                #   iface $bridgeName inet <config>
                words = s.ident().split()
                words[3] = "manual"
                lastStanza = newStanzas.pop()
                newStanzas.append(Stanza(" ".join(words), []))
                newStanzas.append(lastStanza)
                # Replace existing line with new $bridgeName
                words = s.ident().split()
                words[1] = bridgeName
                options = s.options()
                options.append("bridge_ports {}".format(ifaceName))
                newStanzas.append(Stanza(" ".join(words), options))
            continue

        alias_re = re.compile(r'{}:(?P<num>\d+)'.format(ifaceName))

        if ifaceName in s.ident():
            match = alias_re.search(s.ident())
            newStanzas.append(Stanza(s.ident().replace(ifaceName, bridgeName), s.options()))

    for s in newStanzas:
        printStanza(s)
        
main(sys.argv[1])
