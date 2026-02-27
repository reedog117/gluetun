package updater

import (
	"context"
	"fmt"
	"net/http"
	"net/netip"
	"sort"

	"github.com/qdm12/gluetun/internal/models"
	"github.com/qdm12/gluetun/internal/provider/common"
	"github.com/qdm12/gluetun/internal/publicip/api"
	"github.com/qdm12/gluetun/internal/updater/resolver"
)

func (u *Updater) FetchServers(ctx context.Context, minServers int) (
	servers []models.Server, err error,
) {
	if !u.ipFetcher.CanFetchAnyIP() {
		return nil, fmt.Errorf("%w: %s", common.ErrIPFetcherUnsupported, u.ipFetcher.String())
	}

	debURL, err := fetchDebURL(ctx, http.DefaultClient)
	if err != nil {
		return nil, fmt.Errorf("fetching .deb URL: %w", err)
	}

	debContent, err := fetchURL(ctx, http.DefaultClient, debURL)
	if err != nil {
		return nil, fmt.Errorf("fetching PureVPN .deb file %q: %w", debURL, err)
	}

	asarContent, err := extractAsarFromDeb(debContent)
	if err != nil {
		return nil, fmt.Errorf("extracting app.asar from .deb: %w", err)
	}

	endpointsContent, endpointsPath, err := extractFirstFileFromAsar(asarContent,
		inventoryEndpointsAsarPath,
		"node_modules/atom-sdk/node_modules/inventory/node_modules/utils/lib/constants/end-points.js")
	if err != nil {
		return nil, fmt.Errorf("extracting inventory endpoints file from app.asar: %w", err)
	}

	inventoryURLTemplate, err := parseInventoryURLTemplate(endpointsContent)
	if err != nil {
		return nil, fmt.Errorf("parsing inventory URL template from %q: %w", endpointsPath, err)
	}

	offlineInventoryContent, offlineInventoryPath, err := extractFirstFileFromAsar(asarContent,
		inventoryOfflineAsarPath,
		"node_modules/atom-sdk/node_modules/inventory/src/offline-data/inventory-data.js")
	if err != nil {
		return nil, fmt.Errorf("extracting inventory offline data from app.asar: %w", err)
	}

	resellerUID, err := parseResellerUIDFromInventoryOffline(offlineInventoryContent)
	if err != nil {
		return nil, fmt.Errorf("parsing reseller UID from %q: %w", offlineInventoryPath, err)
	}

	inventoryURL, err := buildInventoryURL(inventoryURLTemplate, resellerUID)
	if err != nil {
		return nil, fmt.Errorf("building inventory URL: %w", err)
	}

	inventoryContent, err := fetchURL(ctx, http.DefaultClient, inventoryURL)
	if err != nil {
		return nil, fmt.Errorf("fetching inventory JSON %q: %w", inventoryURL, err)
	}

	hts, hostToFallbackIPs, err := parseInventoryJSON(inventoryContent)
	if err != nil {
		return nil, fmt.Errorf("parsing inventory JSON from %q: %w", inventoryURL, err)
	}
	if len(hts) < minServers {
		return nil, fmt.Errorf("%w: %d and expected at least %d",
			common.ErrNotEnoughServers, len(hts), minServers)
	}

	hosts := hts.toHostsSlice()
	resolveSettings := parallelResolverSettings(hosts)
	hostToIPs, warnings, err := resolveWithMultipleResolvers(ctx, u.parallelResolver, resolveSettings)
	warnAll(u.warner, warnings)
	if err != nil {
		return nil, err
	}

	applyFallbackIPs(hostToIPs, hostToFallbackIPs, hosts)

	if len(hostToIPs) < minServers {
		return nil, fmt.Errorf("%w: %d and expected at least %d",
			common.ErrNotEnoughServers, len(hostToIPs), minServers)
	}

	hts.adaptWithIPs(hostToIPs)

	servers = hts.toServersSlice()

	// Get public IP address information
	ipsToGetInfo := make([]netip.Addr, len(servers))
	for i := range servers {
		ipsToGetInfo[i] = servers[i].IPs[0]
	}
	ipsInfo, err := api.FetchMultiInfo(ctx, u.ipFetcher, ipsToGetInfo)
	if err != nil {
		return nil, err
	}

	for i := range servers {
		parsedCountry, parsedCity, warnings := parseHostname(servers[i].Hostname)
		for _, warning := range warnings {
			u.warner.Warn(warning)
		}
		servers[i].Country = parsedCountry
		if servers[i].Country == "" {
			servers[i].Country = ipsInfo[i].Country
		}

		countryMatchesGeolocation := shouldUseGeolocation(parsedCountry, ipsInfo[i].Country)

		servers[i].City = parsedCity
		if servers[i].City == "" && countryMatchesGeolocation {
			servers[i].City = ipsInfo[i].City
		}

		if countryMatchesGeolocation &&
			(parsedCity == "" ||
				comparePlaceNames(parsedCity, ipsInfo[i].City)) {
			servers[i].Region = ipsInfo[i].Region
		}
	}

	sort.Sort(models.SortableServers(servers))

	return servers, nil
}

func shouldUseGeolocation(parsedCountry, geolocationCountry string) (use bool) {
	return parsedCountry == "" || comparePlaceNames(parsedCountry, geolocationCountry)
}

func resolveWithMultipleResolvers(ctx context.Context, primary common.ParallelResolver,
	settings resolver.ParallelSettings,
) (hostToIPs map[string][]netip.Addr, warnings []string, err error) {
	hostToIPs = make(map[string][]netip.Addr, len(settings.Hosts))

	mergeResult := func(newHostToIPs map[string][]netip.Addr) {
		for host, ips := range newHostToIPs {
			existing := hostToIPs[host]
			for _, ip := range ips {
				existing = appendIPIfMissing(existing, ip)
			}
			hostToIPs[host] = existing
		}
	}

	primaryHostToIPs, primaryWarnings, primaryErr := primary.Resolve(ctx, settings)
	warnings = append(warnings, primaryWarnings...)
	if primaryErr == nil {
		mergeResult(primaryHostToIPs)
	} else {
		warnings = append(warnings, primaryErr.Error())
	}

	// Try multiple DNS resolvers to recover hosts that are flaky or resolver-specific.
	for _, dnsAddress := range []string{"1.1.1.1", "8.8.8.8", "9.9.9.9"} {
		parallelResolver := resolver.NewParallelResolver(dnsAddress)
		hostToIPsCandidate, candidateWarnings, candidateErr := parallelResolver.Resolve(ctx, settings)
		warnings = append(warnings, candidateWarnings...)
		if candidateErr != nil {
			warnings = append(warnings, candidateErr.Error())
			continue
		}
		mergeResult(hostToIPsCandidate)
	}

	if len(hostToIPs) == 0 {
		return nil, warnings, fmt.Errorf("%w", common.ErrNotEnoughServers)
	}

	return hostToIPs, warnings, nil
}

func applyFallbackIPs(hostToIPs map[string][]netip.Addr, hostToFallbackIPs map[string][]netip.Addr, hosts []string) {
	if len(hostToFallbackIPs) == 0 {
		return
	}
	for _, host := range hosts {
		if len(hostToIPs[host]) > 0 {
			continue
		}
		fallbackIPs := hostToFallbackIPs[host]
		if len(fallbackIPs) == 0 {
			continue
		}
		hostToIPs[host] = append([]netip.Addr(nil), fallbackIPs...)
	}
}

func warnAll(warner common.Warner, warnings []string) {
	for _, warning := range warnings {
		warner.Warn(warning)
	}
}
