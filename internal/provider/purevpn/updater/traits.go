package updater

import "strings"

func inferPureVPNTraits(hostname string) (portForward, quantumResistant, obfuscated bool) {
	labels := strings.Split(strings.ToLower(hostname), ".")
	if len(labels) == 0 {
		return false, false, false
	}

	for _, token := range strings.Split(labels[0], "-") {
		switch token {
		case "pf":
			portForward = true
		case "qr":
			quantumResistant = true
		case "obf":
			obfuscated = true
		}
	}

	return portForward, quantumResistant, obfuscated
}
