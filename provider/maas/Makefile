all: bridgescript.go add-bridge.sh

dollar='$'

bridgescript.go: add-bridge-common.sh add-bridge.py add-bridge-main.sh Makefile
	$(RM) $@
	echo -n '// This file is auto generated. Edits will be lost.\n\n' >> $@
	echo -n 'package maas\n\n' >> $@
	echo -n '//go:generate make\n\n' >> $@
	echo -n 'const bridgeScriptCommon = `' >> $@
	cat add-bridge-common.sh >> $@
	echo -n "python_script=\$$(cat <<'PYTHON_SCRIPT'\n" >> $@
	cat add-bridge.py >> $@
	echo -n 'PYTHON_SCRIPT\n)`\n' >> $@
	echo -n 'const bridgeScriptMain = bridgeScriptCommon + `\n' >> $@
	cat add-bridge-main.sh >> $@
	echo -n '`\n' >> $@

add-bridge.sh: add-bridge-common.sh add-bridge.py add-bridge-main.sh Makefile
	$(RM) $@
	echo -n '#!/bin/bash\n' >> $@
	cat add-bridge-common.sh >> $@
	echo -n "python_script=\$$(cat <<'PYTHON_SCRIPT'\n" >> $@
	cat add-bridge.py >> $@
	echo -n 'PYTHON_SCRIPT\n)\n' >> $@
	cat add-bridge-main.sh >> $@
	chmod 755 $@

clean:
	$(RM) add-bridge.sh bridgescript.go
