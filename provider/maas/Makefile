all: bridgescript.go

# TODO add bridgescript_doc.go that explains (in a package comment)
# what this script (add-bridge.py) does, and why, and how to
# change it.

bridgescript.go: add-bridge.py bridge-interface Makefile
	$(RM) $@
	echo -n '// This file is auto generated. Edits will be lost.\n\n' >> $@
	echo -n 'package maas\n\n' >> $@
	echo -n 'const bridgeScriptBashFilename = "bridge-interface"\n' >> $@
	echo -n 'const bridgeScriptPythonFilename = "add-bridge.py"\n\n' >> $@
	echo -n "const bridgeScriptPythonContent = \`" >> $@
	cat add-bridge.py >> $@
	echo -n '`\n\n' >> $@
	echo -n "const bridgeScriptBashContent = \`" >> $@
	cat bridge-interface >> $@
	echo -n '`\n' >> $@

format:
	pyfmt -i add-bridge.py

clean:
	$(RM) bridgescript.go
