package utils

import (
	"fmt"
	"strings"

	"github.com/qdm12/gluetun/internal/configuration/settings"
	"github.com/qdm12/gluetun/internal/models"
)

func filterServers(servers []models.Server,
	selection settings.ServerSelection,
) (filtered []models.Server) {
	for _, server := range servers {
		if filterServer(server, selection) {
			continue
		}

		filtered = append(filtered, server)
	}

	return filtered
}

func filterServer(server models.Server,
	selection settings.ServerSelection,
) (filtered bool) {
	// Note each condition is split to make sure
	// we have full testing coverage.
	if server.VPN != selection.VPN {
		return true
	}

	if filterByProtocol(selection, server.TCP, server.UDP) {
		return true
	}

	if *selection.MultiHopOnly && !server.MultiHop {
		return true
	}

	if *selection.FreeOnly && !server.Free {
		return true
	}

	if *selection.PremiumOnly && !server.Premium {
		return true
	}

	if *selection.StreamOnly && !server.Stream {
		return true
	}

	if *selection.OwnedOnly && !server.Owned {
		return true
	}

	if *selection.PortForwardOnly && !server.PortForward {
		return true
	}

	if filterByPureVPNServerTypes(server, selection.PureVPNServerTypes) {
		return true
	}
	if filterByPureVPNLocationCodes(server, selection.PureVPNCountryCodes, selection.PureVPNLocationCodes) {
		return true
	}

	if *selection.SecureCoreOnly && !server.SecureCore {
		return true
	}

	if *selection.TorOnly && !server.Tor {
		return true
	}

	if filterByPossibilities(server.Country, selection.Countries) {
		return true
	}

	if filterAllByPossibilities(server.Categories, selection.Categories) {
		return true
	}

	if filterByPossibilities(server.City, selection.Cities) {
		return true
	}

	if filterByPossibilities(server.ISP, selection.ISPs) {
		return true
	}

	if filterByPossibilities(server.Number, selection.Numbers) {
		return true
	}

	if filterByPossibilities(server.ServerName, selection.Names) {
		return true
	}

	if filterByPossibilities(server.Hostname, selection.Hostnames) {
		return true
	}

	// TODO filter port forward server for PIA

	return false
}

func filterByPureVPNServerTypes(server models.Server, serverTypes []string) (filtered bool) {
	isP2P := containsCategory(server.Categories, "p2p")
	if len(serverTypes) == 0 {
		return server.Obfuscated
	}

	allowObfuscated := containsCategory(serverTypes, "obfuscation")
	if server.Obfuscated && !allowObfuscated {
		return true
	}

	for _, serverType := range serverTypes {
		switch serverType {
		case "regular":
			if server.PortForward || server.QuantumResistant || server.Obfuscated || isP2P {
				return true
			}
		case "portforwarding":
			if !server.PortForward {
				return true
			}
		case "quantumresistant":
			if !server.QuantumResistant {
				return true
			}
		case "obfuscation":
			if !server.Obfuscated {
				return true
			}
		case "p2p":
			if !isP2P {
				return true
			}
		default:
			return false
		}
	}
	return false
}

func containsCategory(categories []string, category string) bool {
	for _, existingCategory := range categories {
		if strings.EqualFold(existingCategory, category) {
			return true
		}
	}
	return false
}

func filterByPureVPNLocationCodes(server models.Server,
	countryCodes, locationCodes []string,
) (filtered bool) {
	if len(countryCodes) == 0 && len(locationCodes) == 0 {
		return false
	}

	countryCode, locationCode := parsePureVPNLocationCodes(server.Hostname)
	if len(countryCodes) > 0 && filterByPossibilities(countryCode, countryCodes) {
		return true
	}
	if len(locationCodes) > 0 && filterByPossibilities(locationCode, locationCodes) {
		return true
	}
	return false
}

func parsePureVPNLocationCodes(hostname string) (countryCode, locationCode string) {
	firstLabel := hostname
	if dotIndex := strings.IndexByte(hostname, '.'); dotIndex > -1 {
		firstLabel = hostname[:dotIndex]
	}

	twoMinusIndex := strings.Index(firstLabel, "2-")
	if twoMinusIndex <= 0 {
		return "", ""
	}

	locationCode = strings.ToLower(firstLabel[:twoMinusIndex])
	if len(locationCode) < 2 {
		return "", ""
	}

	countryCode = locationCode[:2]
	return countryCode, locationCode
}

func filterByPossibilities[T string | uint16](value T, possibilities []T) (filtered bool) {
	if len(possibilities) == 0 {
		return false
	}
	for _, possibility := range possibilities {
		if strings.EqualFold(fmt.Sprint(value), fmt.Sprint(possibility)) {
			return false
		}
	}
	return true
}

func filterAnyByPossibilities(values, possibilities []string) (filtered bool) {
	if len(possibilities) == 0 {
		return false
	}

	for _, value := range values {
		if !filterByPossibilities(value, possibilities) {
			return false // found a valid value
		}
	}

	return true
}

func filterAllByPossibilities(values, possibilities []string) (filtered bool) {
	if len(possibilities) == 0 {
		return false
	}
	for _, possibility := range possibilities {
		if !containsCategory(values, possibility) {
			return true
		}
	}
	return false
}
