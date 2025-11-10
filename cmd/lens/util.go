package main

import (
	"encoding/json"
	"errors"
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
	if EnableIRTT {
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
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		log.Println("Error getting interface addresses: ", err)
		return false
	}
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
	err := os.MkdirAll(path.Join("data", today), 0755)
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
	output, err := cmd.CombinedOutput()
	if err != nil {
		log.Printf("tar --zstd returned error: %s", err)
	}
	// Normally, when zstd is installed,
	// tar --zstd
	// tar: You must specify one of the '-Acdtrux', '--delete' or '--test-label' options
	// Try 'tar --help' or 'tar --usage' for more information.
	// return code is $? = 2
	// no need to check err, but to check output whether zstd is supported by this version of tar
	if strings.Contains(string(output), "unrecognized option") {
		return errors.New("zstd is not supported")
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

	return cmd.Run()
}

func getExternalIP(version int) string {
	if version != 4 && version != 6 {
		version = 6
	}
	output, err := exec.Command("curl", fmt.Sprintf("-%d", version), "-m", "5", "-s", "--interface", Iface, "ifconfig.io").CombinedOutput()
	if err != nil {
		log.Printf("get external IP%d addresses failed: %s", version, err)
		log.Println("output: ", string(output))
		return ""
	}
	return strings.Trim(string(output), "\n")
}

func getStarlinkPoP(ip string) string {
	pop, ok := geoipClient.GetPopByCIDR(ip)
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
	cmd, err := exec.Command("mtr", "ipv6.google.com", "-n", "-m", IPv6GatewayHopCount, "-I", Iface, "-c", "1", "--json").CombinedOutput()
	if err != nil {
		log.Println("mtr failed: ", err)
		return ""
	}

	var mtrOutput MTRResult
	err = json.Unmarshal([]byte(string(cmd)), &mtrOutput)
	if err != nil {
		log.Println("Error unmarshalling mtr output: ", err)
		return ""
	}

	for _, h := range mtrOutput.Report.Hubs {
		if strconv.Itoa(h.Count) == IPv6GatewayHopCount {
			return h.Host
		}
	}

	log.Println("GW not detected using mtr")
	log.Println("Trying traceroute")

	output, err := exec.Command("traceroute",
		"-6",
		"-i", Iface,
		"ipv6.google.com",
		"-n",
		"-m", IPv6GatewayHopCount,
		"-f", IPv6GatewayHopCount,
		"-q", "1").CombinedOutput()
	if err != nil {
		log.Printf("traceroute failed: %s", err)
		return ""
	}
	tracerouteResult := ""
	tracerouteResult = string(output)
	gateway := strings.Split(tracerouteResult, "\n")[len(strings.Split(tracerouteResult, "\n"))-2]
	gateway = strings.Split(gateway, " ")[3]
	if gateway == "*" || net.ParseIP(gateway).To16() == nil {
		log.Printf("traceroute failed")
		return ""
	}
	return gateway
}

func getGateway() string {
	gatewayIP := ""
	externalIP := ""

	if ManualSpecifiedGateway != "" {
		if net.ParseIP(ManualSpecifiedGateway).To4() != nil {
			IPVersion = 4
		} else if len(net.ParseIP(ManualSpecifiedGateway)) == net.IPv6len {
			IPVersion = 6
		}
		gatewayIP = ManualSpecifiedGateway
	} else {
		// Active dish, probe IPv6 active gateway through mtr or traceroute
		externalIPv6 = getExternalIP(6)
		if ipExist(externalIPv6) {
			// If external IPv6 address exists on the interface
			IPVersion = 6

			log.Println("External IPv6:", externalIPv6)
			externalIP = externalIPv6
			gatewayIP = getStarlinkIPv6ActiveGateway()
		} else {
			externalIPv4 = getExternalIP(4)
			if net.ParseIP(externalIPv4).To4() != nil {
				// CGNAT IPv4 does not exist on the interface locally
				IPVersion = 4

				log.Println("External IPv4: ", externalIPv4)
				externalIP = externalIPv4
				gatewayIP = defaultIPv4CGNATGateway
			}
		}
	}

	PoP = getStarlinkPoP(externalIP)
	return gatewayIP
}
