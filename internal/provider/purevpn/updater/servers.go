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

	localDataContent, err := extractFileFromAsar(asarContent, localDataAsarPath)
	if err != nil {
		return nil, fmt.Errorf("extracting %q from app.asar: %w", localDataAsarPath, err)
	}

	hts, err := parseLocalData(localDataContent)
	if err != nil {
		return nil, fmt.Errorf("parsing %q: %w", localDataAsarPath, err)
	} else if len(hts) < minServers {
		return nil, fmt.Errorf("%w: %d and expected at least %d",
			common.ErrNotEnoughServers, len(hts), minServers)
	}

	hosts := hts.toHostsSlice()
	resolveSettings := parallelResolverSettings(hosts)
	hostToIPs, warnings, err := u.parallelResolver.Resolve(ctx, resolveSettings)
	for _, warning := range warnings {
		u.warner.Warn(warning)
	}
	if err != nil {
		return nil, err
	}

	if len(hostToIPs) < minServers {
		return nil, fmt.Errorf("%w: %d and expected at least %d",
			common.ErrNotEnoughServers, len(servers), minServers)
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
