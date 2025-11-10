package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net"
	"os"
	"os/exec"
	"path"
	"strconv"
	"strings"
	"time"
)

func datetimeString() string {
	return time.Now().UTC().Format("2006-01-02-15-04-05")
}

func CheckDeps() error {
	cmds := []string{"ping", "mtr", "traceroute", "dig", "curl", "tar"}
	if ENABLE_IRTT {
		cmds = append(cmds, "irtt")
	}
	for _, c := range cmds {
		if _, err := exec.LookPath(c); err != nil {
			if _, err := os.Stat(c); err != nil {
				return fmt.Errorf("%s is not installed", c)
			}
		}
	}
	return nil
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

func getStarlinkPoP(ip string) string {
	pop, ok := geoipClient.GetPopByCIDRFrom(ip)
	if !ok {
		return ""
	}
	return pop.Pop
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

func getGateway() string {
	gateway_ip := ""
	external_ip := ""

	if MANUAL_GW != "" {
		if net.ParseIP(MANUAL_GW).To4() != nil {
			IPVersion = 4
		} else if len(net.ParseIP(MANUAL_GW)) == net.IPv6len {
			IPVersion = 6
		}
		gateway_ip = MANUAL_GW
	} else {
		// Active dish, probe IPv6 active gateway through mtr or traceroute
		external_ip6 = getExternalIP(6)
		if ipExist(external_ip6) {
			// If external IPv6 address exists on the interface
			IPVersion = 6

			log.Println("External IPv6: ", external_ip6)
			external_ip = external_ip6
			gateway_ip = getStarlinkIPv6ActiveGateway()
		} else {
			external_ip4 = getExternalIP(4)
			if net.ParseIP(external_ip4).To4() != nil {
				// CGNAT IPv4 does not exist on the interface locally
				IPVersion = 4

				log.Println("External IPv4: ", external_ip4)
				external_ip = external_ip4
				gateway_ip = defaultIPv4CGNATGateway
			}
		}
	}

	PoP = getStarlinkPoP(external_ip)
	return gateway_ip
}
