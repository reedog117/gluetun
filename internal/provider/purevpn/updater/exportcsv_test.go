//go:build integration

package updater

import (
	"context"
	"encoding/csv"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/qdm12/gluetun/internal/constants"
)

func Test_exportLocalDataCSV(t *testing.T) {
	t.Parallel()

	const outPath = "artifacts/purevpn_connections_extracted.csv"
	if err := os.MkdirAll(filepath.Dir(outPath), 0o755); err != nil {
		t.Fatalf("creating artifacts directory: %v", err)
	}

	ctx := context.Background()
	debURL, err := fetchDebURL(ctx, http.DefaultClient)
	if err != nil {
		t.Fatalf("fetching deb URL: %v", err)
	}
	debContent, err := fetchURL(ctx, http.DefaultClient, debURL)
	if err != nil {
		t.Fatalf("fetching deb content: %v", err)
	}
	asarContent, err := extractAsarFromDeb(debContent)
	if err != nil {
		t.Fatalf("extracting app.asar from deb: %v", err)
	}
	localDataContent, err := extractFileFromAsar(asarContent, localDataAsarPath)
	if err != nil {
		t.Fatalf("extracting local-data.js from asar: %v", err)
	}

	hts, err := parseLocalData(localDataContent)
	if err != nil {
		t.Fatalf("parsing local-data.js: %v", err)
	}

	hosts := hts.toHostsSlice()
	sort.Strings(hosts)

	file, err := os.Create(outPath)
	if err != nil {
		t.Fatalf("creating csv file: %v", err)
	}
	defer file.Close()

	w := csv.NewWriter(file)
	defer w.Flush()

	header := []string{
		"hostname",
		"country_code",
		"country",
		"region",
		"city",
		"tcp",
		"udp",
		"tcp_ports",
		"udp_ports",
	}
	if err := w.Write(header); err != nil {
		t.Fatalf("writing csv header: %v", err)
	}

	countryCodeToName := constants.CountryCodes()
	for _, host := range hosts {
		server := hts[host]
		countryCode := ""
		country := ""
		if len(host) >= 2 {
			countryCode = strings.ToLower(host[:2])
			country = countryCodeToName[countryCode]
		}
		_, city, _ := parseHostname(host)
		row := []string{
			host,
			countryCode,
			country,
			"", // region is not directly available from local-data.js without DNS/IP enrichment
			city,
			strconv.FormatBool(server.TCP),
			strconv.FormatBool(server.UDP),
			joinPortsCSV(server.TCPPorts),
			joinPortsCSV(server.UDPPorts),
		}
		if err := w.Write(row); err != nil {
			t.Fatalf("writing csv row: %v", err)
		}
	}

	if err := w.Error(); err != nil {
		t.Fatalf("finalizing csv writer: %v", err)
	}

	t.Logf("wrote %d connection rows to %s", len(hosts), outPath)
}

func joinPortsCSV(ports []uint16) string {
	if len(ports) == 0 {
		return ""
	}
	parts := make([]string, len(ports))
	for i, port := range ports {
		parts[i] = strconv.Itoa(int(port))
	}
	return strings.Join(parts, "|")
}
