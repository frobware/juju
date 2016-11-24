all: bridgescript.go

# TODO add bridgescript_doc.go that explains (in a package comment)
# what this script (bridge-interface.py) does, and why, and how to
# change it.

bridgescript.go: bridge-interface.py bridge-interface Makefile
	$(RM) $@
	echo -n '// This file is auto generated. Edits will be lost.\n\n' >> $@
	echo -n 'package maas\n\n' >> $@
	echo -n 'const bridgeScriptBashFilename = "bridge-interface"\n' >> $@
	echo -n 'const bridgeScriptPythonFilename = "bridge-interface.py"\n\n' >> $@
	echo -n "const bridgeScriptPythonContent = \`" >> $@
	cat bridge-interface.py >> $@
	echo -n '`\n\n' >> $@
	echo -n "const bridgeScriptBashContent = \`" >> $@
	cat bridge-interface >> $@
	echo -n '`\n' >> $@

format:
	pyfmt -i bridge-interface.py

clean:
	$(RM) bridgescript.go
