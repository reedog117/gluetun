//go:build integration

package updater

import (
	"context"
	"encoding/csv"
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
}
