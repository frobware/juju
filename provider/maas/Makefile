all: add-bridge.sh bridgescript.go

bridgescript.go: add-bridge-common.sh add-bridge.py add-bridge-main.sh Makefile
	$(RM) $@
	echo -n '// This file is auto generated. Edits will be lost.\n\n' >> $@
	echo -n 'package maas\n\n' >> $@
	echo -n 'const bridgeScriptCommon = `' >> $@
	cat add-bridge-common.sh >> $@
	echo -n "\ndefvar python_script <<'EOF'\n" >> $@
	cat add-bridge.py >> $@
	echo -n 'EOF\n`\n' >> $@
	echo -n 'const bridgeScriptMain = bridgeScriptCommon + `\n' >> $@
	cat add-bridge-main.sh >> $@
	echo -n 'EOF\n`' >> $@

add-bridge.sh: add-bridge-common.sh add-bridge.py add-bridge-main.sh Makefile
	$(RM) $@
	echo -n '#!/bin/bash\n' >> $@
	cat add-bridge-common.sh >> $@
	echo -n "\ndefvar python_script <<'EOF'\n" >> $@
	cat add-bridge.py >> $@
	echo -n 'EOF\n\n' >> $@
	cat add-bridge-main.sh >> $@
	chmod 755 $@

clean:
	$(RM) add-bridge.sh bridgescript.go
