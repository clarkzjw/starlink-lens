package main

import (
	"encoding/csv"
	"io"
	"net"
	"net/http"
	"os/exec"
	"strings"
	"sync"
	"time"
)

type PopInfo struct {
	CIDR string
	Pop  string
	City string
}

type GeoIPClient struct {
	mu sync.RWMutex

	PopCsvLastUpdated int64

	// map from cidr -> PopInfo
	CIDRMap map[string]PopInfo
}

const (
	popCsvURL = "https://geoip.starlinkisp.net/pops.csv"
)

// cidr,pop,city
// 14.1.64.0/24,mnlaphl1,mnl
// 14.1.65.0/24,mnlaphl1,mnl
// 14.1.66.0/24,mnlaphl1,mnl
// 14.1.67.0/24,mnlaphl1,mnl
// 14.1.72.0/24,mlbeaus1,mel

// fetchPoPCsv downloads and parses the POP CSV, returning the map on success.
func fetchPoPCsv() (map[string]PopInfo, error) {
	resp, err := http.Get(popCsvURL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, err
	}

	r := csv.NewReader(resp.Body)
	newMap := make(map[string]PopInfo)
	for {
		record, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		if len(record) < 3 {
			continue
		}
		cidr := strings.TrimSpace(record[0])
		pop := strings.TrimSpace(record[1])
		city := strings.TrimSpace(record[2])
		if cidr == "" {
			continue
		}
		newMap[cidr] = PopInfo{
			CIDR: cidr,
			Pop:  pop,
			City: city,
		}
	}
	return newMap, nil
}

// NewGeoIPClient creates a GeoIPClient, downloads the CSV from PopCsvUrl,
// parses lines of the form cidr,pop,city and stores them in CIDRMap.
// On success PopCsvLastUpdated is set to the current Unix timestamp.
// On any error an empty client is returned (PopCsvLastUpdated == 0).
func NewGeoIPClient() *GeoIPClient {
	client := &GeoIPClient{
		CIDRMap: make(map[string]PopInfo),
	}

	// attempt initial download once
	if newMap, err := fetchPoPCsv(); err == nil {
		client.mu.Lock()
		client.CIDRMap = newMap
		client.PopCsvLastUpdated = time.Now().Unix()
		client.mu.Unlock()
	}

	go func(c *GeoIPClient) {
		ticker := time.NewTicker(10 * time.Minute)
		defer ticker.Stop()
		for range ticker.C {
			c.UpdatePoPCsv()
		}
	}(client)

	return client
}

// check current time and compare with PopCsvLastUpdated, if more than 12 hours, re-download, and update PopCsvLastUpdated
func (g *GeoIPClient) UpdatePoPCsv() {
	if g == nil {
		return
	}

	now := time.Now().Unix()

	g.mu.RLock()
	last := g.PopCsvLastUpdated
	g.mu.RUnlock()

	// if updated within 12 hours, nothing to do
	if now-last < 3600 {
		return
	}

	newMap, err := fetchPoPCsv()
	if err != nil {
		return
	}

	g.mu.Lock()
	g.CIDRMap = newMap
	g.PopCsvLastUpdated = time.Now().Unix()
	g.mu.Unlock()
}

// GetPopByCIDR returns the best-matching PopInfo for the given IP string
func (g *GeoIPClient) GetPopByCIDR(cidr string) (PopInfo, bool) {
	if g == nil || len(g.CIDRMap) == 0 {
		return PopInfo{}, false
	}

	parsedIP := net.ParseIP(cidr)
	if parsedIP == nil {
		_, ipnet, err := net.ParseCIDR(strings.TrimSpace(cidr))
		if err != nil || ipnet == nil {
			return PopInfo{}, false
		}
		parsedIP = ipnet.IP
	}

	// Choose the longest-prefix match when multiple CIDRs contain the IP.
	bestOnes := -1
	var best PopInfo

	for cidr, info := range g.CIDRMap {
		if info.City == "staging" {
			continue
		}
		_, ipnet, err := net.ParseCIDR(strings.TrimSpace(cidr))
		if err != nil || ipnet == nil {
			continue
		}
		if ipnet.Contains(parsedIP) {
			ones, _ := ipnet.Mask.Size()
			if ones > bestOnes {
				bestOnes = ones
				best = info
			}
		}
	}

	if bestOnes >= 0 {
		return best, true
	}
	return PopInfo{}, false
}

// GetDNSPtrFromDig returns the PTR record for the given IP using dig command
func (*GeoIPClient) GetDNSPtrFromDig(ip string) (string, error) {
	cmd := exec.Command("dig", "@1.1.1.1", "-x", ip, "+trace", "+short")
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}

	output := strings.TrimSpace(string(out))
	// e.g.
	// NS a.root-servers.net. from server 1.1.1.1 in 12 ms.
	// ...
	// NS m.root-servers.net. from server 1.1.1.1 in 12 ms.
	// RRSIG NS 8 0 518400 20251121170000 20251108160000 61809 . 4G4C++oCv6qB ... from server 1.1.1.1 in 12 ms.
	// PTR customer.sttlwax1.isp.starlink.com. from server 195.134.238.37 in 80 ms.

	lines := strings.Split(output, "\n")
	var ptr string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "PTR ") {
			parts := strings.SplitN(line, " ", 3)
			if len(parts) >= 2 {
				ptr = strings.TrimSuffix(parts[1], ".")
				break
			}
		}
	}
	return ptr, nil
}
