package updater

import (
	"fmt"
	"strconv"
	"strings"
)

const localDataAsarPath = "node_modules/atom-sdk/lib/offline-data/local-data.js"

var (
	hostNeedle = `name:"`
	portNeedle = `port_number:`
)

func parseLocalData(content []byte) (hostToServer, error) {
	raw := string(content)
	hts := make(hostToServer)
	const protocolNeedle = `protocol:"`
	blocksFound := 0
	for index := 0; index < len(raw); {
		start := strings.Index(raw[index:], protocolNeedle)
		if start == -1 {
			break
		}
		start += index
		protocolStart := start + len(protocolNeedle)
		protocolEnd := strings.IndexByte(raw[protocolStart:], '"')
		if protocolEnd == -1 {
			break
		}
		protocolEnd += protocolStart
		protocol := strings.ToUpper(raw[protocolStart:protocolEnd])
		tcp := protocol == "TCP"
		udp := protocol == "UDP"
		index = protocolEnd + 1
		if !tcp && !udp {
			continue
		}
		blocksFound++

		dnsMarker := strings.Index(raw[index:], `dns:[`)
		if dnsMarker == -1 {
			continue
		}
		dnsMarker += index
		arrayStart := dnsMarker + len(`dns:`)
		dnsArray, arrayEnd, err := extractBracketContent(raw, arrayStart, '[', ']')
		if err != nil {
			continue
		}
		index = arrayEnd + 1

		for _, entry := range splitObjectEntries(dnsArray) {
			host, port, ok := parseHostPortFromDNSEntry(entry)
			if !ok {
				continue
			}
			hts.add(host, tcp, udp, port)
		}
	}

	if blocksFound == 0 {
		return nil, fmt.Errorf("no TCP/UDP protocol blocks found in local-data payload")
	}
	if len(hts) == 0 {
		return nil, fmt.Errorf("no OpenVPN TCP/UDP DNS hosts found in local-data payload")
	}

	return hts, nil
}

func extractBracketContent(s string, openIndex int, open, close byte) (content string, closeIndex int, err error) {
	if openIndex < 0 || openIndex >= len(s) || s[openIndex] != open {
		return "", -1, fmt.Errorf("opening bracket not found at index %d", openIndex)
	}
	depth := 0
	for i := openIndex; i < len(s); i++ {
		switch s[i] {
		case open:
			depth++
		case close:
			depth--
			if depth == 0 {
				return s[openIndex+1 : i], i, nil
			}
		}
	}
	return "", -1, fmt.Errorf("closing bracket not found for index %d", openIndex)
}

func splitObjectEntries(s string) (entries []string) {
	entryStart := -1
	depth := 0
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '{':
			if depth == 0 {
				entryStart = i
			}
			depth++
		case '}':
			if depth == 0 {
				continue
			}
			depth--
			if depth == 0 && entryStart >= 0 {
				entries = append(entries, s[entryStart:i+1])
				entryStart = -1
			}
		}
	}
	return entries
}

func parseHostPortFromDNSEntry(entry string) (host string, port uint16, ok bool) {
	hostStart := strings.Index(entry, hostNeedle)
	if hostStart == -1 {
		return "", 0, false
	}
	hostStart += len(hostNeedle)
	hostEnd := strings.IndexByte(entry[hostStart:], '"')
	if hostEnd == -1 {
		return "", 0, false
	}
	hostEnd += hostStart
	host = strings.TrimSpace(entry[hostStart:hostEnd])
	if host == "" {
		return "", 0, false
	}

	portStart := strings.Index(entry, portNeedle)
	if portStart == -1 {
		return "", 0, false
	}
	portStart += len(portNeedle)
	portEnd := portStart
	for ; portEnd < len(entry) && entry[portEnd] >= '0' && entry[portEnd] <= '9'; portEnd++ {
	}
	if portEnd == portStart {
		return "", 0, false
	}
	port64, err := strconv.ParseUint(entry[portStart:portEnd], 10, 16)
	if err != nil || port64 == 0 {
		return "", 0, false
	}
	return host, uint16(port64), true
}
