//go:build integration

package updater

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/qdm12/gluetun/internal/publicip/api"
	"github.com/qdm12/gluetun/internal/updater/resolver"
)

type exportWarner struct{}

func (w exportWarner) Warn(s string) {
	fmt.Fprintln(os.Stderr, "WARN:", s)
}

func Test_exportEnrichedServersCSV(t *testing.T) {
	t.Parallel()

	const outPath = "artifacts/purevpn_servers_enriched.csv"
	if err := os.MkdirAll(filepath.Dir(outPath), 0o755); err != nil {
		t.Fatalf("creating artifacts directory: %v", err)
	}

	httpClient := &http.Client{Timeout: 20 * time.Second}
	nameTokenPairs := []api.NameToken{
		{Name: string(api.IPInfo)},
		{Name: string(api.IP2Location)},
		{Name: string(api.IfConfigCo)},
	}
	fetchers, err := api.New(nameTokenPairs, httpClient)
	if err != nil {
		t.Fatalf("creating public IP fetchers: %v", err)
	}
	ipFetcher := api.NewResilient(fetchers, exportWarner{})
	parallelResolver := resolver.NewParallelResolver("1.1.1.1")

	u := New(ipFetcher, nil, exportWarner{}, parallelResolver)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	servers, err := u.FetchServers(ctx, 1)
	if err != nil {
		t.Fatalf("fetching enriched servers: %v", err)
	}

	file, err := os.Create(outPath)
	if err != nil {
		t.Fatalf("creating CSV file: %v", err)
	}
	defer file.Close()

	w := csv.NewWriter(file)
	defer w.Flush()

	header := []string{
		"hostname",
		"country",
		"region",
		"city",
		"tcp",
		"udp",
		"tcp_ports",
		"udp_ports",
		"ip_count",
		"ips",
	}
	if err := w.Write(header); err != nil {
		t.Fatalf("writing CSV header: %v", err)
	}

	for _, server := range servers {
		ips := make([]string, len(server.IPs))
		for i, ip := range server.IPs {
			ips[i] = ip.String()
		}
		row := []string{
			server.Hostname,
			server.Country,
			server.Region,
			server.City,
			fmt.Sprintf("%t", server.TCP),
			fmt.Sprintf("%t", server.UDP),
			joinPortsCSV(server.TCPPorts),
			joinPortsCSV(server.UDPPorts),
			fmt.Sprintf("%d", len(server.IPs)),
			strings.Join(ips, "|"),
		}
		if err := w.Write(row); err != nil {
			t.Fatalf("writing CSV row: %v", err)
		}
	}

	if err := w.Error(); err != nil {
		t.Fatalf("finalizing CSV: %v", err)
	}

	t.Logf("wrote %d enriched rows to %s", len(servers), outPath)

	username := strings.TrimSpace(firstNonEmpty(
		os.Getenv("PUREVPN_USER"),
		os.Getenv("PUREVPN_USERNAME"),
		os.Getenv("OPENVPN_USER")))
	password := strings.TrimSpace(firstNonEmpty(
		os.Getenv("PUREVPN_PASSWORD"),
		os.Getenv("OPENVPN_PASSWORD")))
	if username == "" || password == "" {
		t.Fatalf("PUREVPN credentials are required to export OpenVPN templates")
	}

	debURL, err := fetchDebURL(ctx, http.DefaultClient)
	if err != nil {
		t.Fatalf("fetching deb URL for templates: %v", err)
	}
	debContent, err := fetchURL(ctx, http.DefaultClient, debURL)
	if err != nil {
		t.Fatalf("fetching deb content for templates: %v", err)
	}
	asarContent, err := extractAsarFromDeb(debContent)
	if err != nil {
		t.Fatalf("extracting app.asar for templates: %v", err)
	}

	endpointsContent, endpointsPath, err := extractFirstFileFromAsar(asarContent,
		inventoryEndpointsAsarPath,
		"node_modules/atom-sdk/node_modules/inventory/node_modules/utils/lib/constants/end-points.js")
	if err != nil {
		t.Fatalf("extracting inventory endpoints file from app.asar: %v", err)
	}
	inventoryURLTemplate, err := parseInventoryURLTemplate(endpointsContent)
	if err != nil {
		t.Fatalf("parsing inventory URL template from %q: %v", endpointsPath, err)
	}

	offlineInventoryContent, offlineInventoryPath, err := extractFirstFileFromAsar(asarContent,
		inventoryOfflineAsarPath,
		"node_modules/atom-sdk/node_modules/inventory/src/offline-data/inventory-data.js")
	if err != nil {
		t.Fatalf("extracting inventory offline data from app.asar: %v", err)
	}
	resellerUID, err := parseResellerUIDFromInventoryOffline(offlineInventoryContent)
	if err != nil {
		t.Fatalf("parsing reseller UID from %q: %v", offlineInventoryPath, err)
	}
	inventoryURL, err := buildInventoryURL(inventoryURLTemplate, resellerUID)
	if err != nil {
		t.Fatalf("building inventory URL: %v", err)
	}
	inventoryContent, err := fetchURL(ctx, http.DefaultClient, inventoryURL)
	if err != nil {
		t.Fatalf("fetching inventory JSON %q: %v", inventoryURL, err)
	}

	templates, err := fetchOpenVPNTemplates(ctx, http.DefaultClient, asarContent, inventoryContent, username, password)
	if err != nil {
		t.Fatalf("fetching OpenVPN templates: %v", err)
	}
	if len(templates) == 0 {
		t.Fatalf("no OpenVPN templates found")
	}

	const templatesOutPath = "artifacts/purevpn_openvpn_templates.json"
	templatesData, err := json.MarshalIndent(templates, "", "  ")
	if err != nil {
		t.Fatalf("marshalling templates artifact: %v", err)
	}
	if err := os.WriteFile(templatesOutPath, append(templatesData, '\n'), 0o600); err != nil {
		t.Fatalf("writing templates artifact: %v", err)
	}
	t.Logf("wrote %d OpenVPN templates to %s", len(templates), templatesOutPath)
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
