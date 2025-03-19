package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net"
	"os"
	"os/exec"
	"path"
	"regexp"
	"strconv"
	"strings"
	"time"
)

func getTimeString() string {
	return time.Now().UTC().Format("2006-01-02-15-04-05")
}

func checkInstalled() {
	cmds := []string{"ping", "mtr", "traceroute", "dig", "curl"}
	for _, c := range cmds {
		if _, err := exec.LookPath(c); err != nil {
			if _, err := os.Stat(c); err != nil {
				log.Fatalf("%s is not installed", c)
			}
		}
	}
}

func ipExist(ip string) bool {
	addrs, _ := net.InterfaceAddrs()
	for _, a := range addrs {
		if ipnet, ok := a.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
			if ipnet.IP.To16() != nil {
				if ipnet.IP.To16().String() == ip {
					return true
				}
			}
		}
	}
	return false
}

func checkDirectory() string {
	today := time.Now().UTC().Format("2006-01-02")
	err := os.MkdirAll(path.Join("data", today), os.ModePerm)
	if err != nil {
		log.Println("Error creating directory: ", err)
	}
	return today
}

func compress(directory, filename string) error {
	cmd := exec.Command("tar", "--zstd", "-C", directory, "-cf", path.Join(directory, fmt.Sprintf("%s.tar.zst", filename)), filename, "--remove-files")
	log.Println(cmd.String())
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return err
	}
	return nil
}

func getExternalIP(IPVersion int) string {
	if IPVersion != 4 && IPVersion != 6 {
		IPVersion = 6
	}
	output, err := exec.Command("curl", fmt.Sprintf("-%d", IPVersion), "-m", "5", "-s", "--interface", IFACE, "ipconfig.io").CombinedOutput()
	if err != nil {
		log.Println("get external IP failed: ", err)
		return ""
	}
	return strings.Trim(string(output), "\n")
}

func getReverseDNS(ip string) string {
	cmd := exec.Command("dig", "@1.1.1.1", "+short", "-x", ip)
	output, err := cmd.CombinedOutput()
	if err != nil {
		log.Println(err)
	}
	return strings.Trim(string(output), "\n")
}

func getStarlinkPoP(rdns string) string {
	// rdns: customer.sttlwax1.pop.starlinkisp.net.
	// PoP: sttlwax1

	regex := `^customer\.(?P<pop>[a-z0-9]+)\.pop\.starlinkisp\.net\.$`
	re := regexp.MustCompile(regex)
	match := re.FindStringSubmatch(rdns)
	if len(match) == 0 {
		return ""
	}
	return match[1]
}

type MTRResult struct {
	Report struct {
		Hubs []struct {
			Count int    `json:"count"`
			Host  string `json:"host"`
		}
	}
}

func getStarlinkIPv6ActiveGateway() string {
	fmt.Println("Getting Starlink IPv6 active gateway")
	cmd, err := exec.Command("mtr", "ipv6.google.com", "-n", "-m", IPv6GWHop, "-I", IFACE, "-c", "1", "--json").CombinedOutput()
	if err != nil {
		log.Panic(err)
	}

	var mtrOutput MTRResult
	err = json.Unmarshal([]byte(string(cmd)), &mtrOutput)
	if err != nil {
		log.Println("Error unmarshalling mtr output: ", err)
	}

	for _, h := range mtrOutput.Report.Hubs {
		if strconv.Itoa(h.Count) == IPv6GWHop {
			return h.Host
		}
	}

	fmt.Println("GW not detected using mtr")
	fmt.Println("Trying traceroute")

	output, err := exec.Command("traceroute", "-6", "-i", IFACE, "ipv6.google.com", "-n", "-m", IPv6GWHop, "-f", IPv6GWHop, "-q", "1").CombinedOutput()
	if err != nil {
		log.Panic(err)
	}
	tracerouteResult := ""
	tracerouteResult = string(output)
	GW := strings.Split(tracerouteResult, "\n")[len(strings.Split(tracerouteResult, "\n"))-2]
	GW = strings.Split(GW, " ")[3]
	if GW == "*" || net.ParseIP(GW).To16() == nil {
		log.Fatal("traceroute failed")
	}
	return GW
}

func getInactiveIPv6PoP() string {
	ifces, _ := net.Interfaces()
	for _, iface := range ifces {
		if iface.Name == IFACE {
			addrs, _ := iface.Addrs()
			for _, addr := range addrs {
				ip, _, _ := net.ParseCIDR(addr.String())
				pop := getStarlinkPoP(getReverseDNS(ip.String()))
				if pop != "" {
					return pop
				}
			}
		}
	}
	return ""
}

func getGateway() string {
	// Inactive dish, return default IPv6 inactive gateway
	// Router Bypass mode has to be set through the Starlink mobile app
	if !ACTIVE {
		PoP = getInactiveIPv6PoP()
		return defaultIPv6InactiveGateway
	}
	// Active dish, probe IPv6 active gateway through mtr or traceroute
	external_ip6 = getExternalIP(6)
	external_ip4 = getExternalIP(4)
	if ipExist(external_ip6) {
		log.Println("External IPv6: ", external_ip6)
		PoP = getStarlinkPoP(getReverseDNS(external_ip6))
		IPVersion = 6
		if GW6 != "fe80::200:5eff:fe00:101" {
			return GW6
		}
		return getStarlinkIPv6ActiveGateway()
	} else if net.ParseIP(external_ip4).To4() != nil {
		log.Println("External IPv4: ", external_ip4)
		PoP = getStarlinkPoP(getReverseDNS(external_ip4))
		IPVersion = 4
		return GW4
	}
	log.Fatal("GW not detected, get external IP failed")
	return ""
}
