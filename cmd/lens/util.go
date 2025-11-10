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

func datetimeString() string {
	return time.Now().UTC().Format("2006-01-02-15-04-05")
}

func CheckPkgsInstalled() {
	cmds := []string{"ping", "mtr", "traceroute", "dig", "curl", "tar"}
	if ENABLE_IRTT {
		cmds = append(cmds, "irtt")
	}
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

func checkZstd() error {
	cmds := []string{"zstd"}
	for _, c := range cmds {
		if _, err := exec.LookPath(c); err != nil {
			if _, err := os.Stat(c); err != nil {
				return fmt.Errorf("%s is not installed", c)
			}
		}
	}

	cmd := exec.Command("tar", "--zstd")
	output, _ := cmd.CombinedOutput()
	// Normally, when zstd is installed,
	// tar --zstd
	// tar: You must specify one of the '-Acdtrux', '--delete' or '--test-label' options
	// Try 'tar --help' or 'tar --usage' for more information.
	// return code is $? = 2
	// no need to check err, but to check output whether zstd is supported by this version of tar
	if strings.Contains(string(output), "unrecognized option") {
		return fmt.Errorf("zstd is not supported")
	}
	return nil
}

func compress(directory, filename string) error {
	cmd := exec.Command("tar", "--zstd", "-C", directory, "-cf", path.Join(directory, fmt.Sprintf("%s.tar.zst", filename)), filename, "--remove-files")
	if err := checkZstd(); err != nil {
		cmd = exec.Command("tar", "-C", directory, "-cf", path.Join(directory, fmt.Sprintf("%s.tar.gz", filename)), filename, "--remove-files")
	}
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
	output, err := exec.Command("curl", fmt.Sprintf("-%d", IPVersion), "-m", "5", "-s", "--interface", IFACE, "ifconfig.io").CombinedOutput()
	if err != nil {
		log.Printf("get external IP%d addresses failed: %s", IPVersion, err)
		log.Println("output: ", string(output))
		return ""
	}
	return strings.Trim(string(output), "\n")
}

func getReverseDNS(ip string) string {
	cmd := exec.Command("dig", "@1.1.1.1", "+short", "-x", ip)
	output, err := cmd.CombinedOutput()
	if err != nil {
		log.Println(err)
		return ""
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

// Deprecated: Now Starlink is moving inactive dishes to stand-by mode,
// which can still reach the Internet, but at ~500 Kbps
// and even inactive dishes can reach 100.64.0.1 now.
// func getInactiveIPv6PoP() string {
// 	ifces, _ := net.Interfaces()
// 	for _, iface := range ifces {
// 		if iface.Name == IFACE {
// 			addrs, _ := iface.Addrs()
// 			for _, addr := range addrs {
// 				ip, _, _ := net.ParseCIDR(addr.String())
// 				pop := getStarlinkPoP(getReverseDNS(ip.String()))
// 				if pop != "" {
// 					return pop
// 				}
// 			}
// 		}
// 	}
// 	return ""
// }

func getGateway() string {
	if MANUAL_GW != "" {
		return MANUAL_GW
	}
	// Active dish, probe IPv6 active gateway through mtr or traceroute
	external_ip6 = getExternalIP(6)
	if ipExist(external_ip6) {
		// If external IPv6 address exists on the interface
		log.Println("External IPv6: ", external_ip6)
		dns_ptr := getReverseDNS(external_ip6)
		if dns_ptr == "" {
			log.Println("get external IPv6 reverse DNS failed")
		} else {
			log.Println("External IPv6 reverse DNS: ", dns_ptr)
		}
		PoP = getStarlinkPoP(dns_ptr)
		if PoP == "" {
			log.Println("get IPv6 PoP code failed")
		} else {
			log.Println("IPv6 PoP code: ", PoP)
		}
		IPVersion = 6
		// If a IPv6 gateway is manually specified
		// if GW6 != "fe80::200:5eff:fe00:101" {
		// 	return GW6
		// }
		return getStarlinkIPv6ActiveGateway()
	} else {
		external_ip4 = getExternalIP(4)

		if net.ParseIP(external_ip4).To4() != nil {
			// CGNAT IPv4 does not exist on the interface locally
			log.Println("External IPv4: ", external_ip4)

			PoP = getStarlinkPoP(getReverseDNS(external_ip4))
			IPVersion = 4
			return defaultIPv4CGNATGateway
		}
	}
	log.Fatal("GW not detected, get external IP failed")
	return ""
}
