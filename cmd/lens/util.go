package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path"
	"regexp"
	"strconv"
	"strings"
	"time"

	http "github.com/hashicorp/go-retryablehttp"
	"github.com/phuslu/log"
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
		log.Error().Err(err).Msg("Error getting interface addresses")
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
		log.Error().Err(err).Msg("Error creating directory")
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
	// Normally, when zstd is installed,
	// tar --zstd
	// tar: You must specify one of the '-Acdtrux', '--delete' or '--test-label' options
	// Try 'tar --help' or 'tar --usage' for more information.
	// return code is $? = 2
	// no need to check err, but to check output whether zstd is supported by this version of tar
	if err != nil && strings.Contains(string(output), "unrecognized option") {
		return errors.New("zstd is not supported")
	}
	return nil
}

func validResult(directory, filename string) error {
	fullPath := path.Join(directory, filename)
	data, err := os.ReadFile(fullPath)
	if err != nil {
		return fmt.Errorf("error reading %s: %w", fullPath, err)
	}
	content := string(data)

	// Match variants like:
	// [1763071200.038727] 64 bytes from 100.64.0.1: icmp_seq=1 ttl=63 time=33.1 ms
	// 64 bytes from 100.64.0.1: icmp_seq=1 ttl=63 time=33.1 ms
	re := regexp.MustCompile(`(?m)(?:\[\d+(?:\.\d+)?\]\s*)?\d+\s+bytes\s+from\s+[0-9a-fA-F:\.]+:.*\btime=[0-9.]+(?:\s*ms)?`)
	if re.FindStringIndex(content) != nil {
		return nil
	}

	return fmt.Errorf("%s contains no valid ping results", fullPath)
}

func compress(directory, filename string) (string, error) {
	fullFilename := path.Join(directory, filename)
	fileInfo, err := os.Stat(fullFilename)
	if err != nil {
		return "", fmt.Errorf("error stating file %s: %w", fullFilename, err)
	}
	if fileInfo.Size() == 0 {
		return "", fmt.Errorf("%s is empty, skipping compression", fullFilename)
	}
	if err := validResult(directory, filename); err != nil {
		return "", fmt.Errorf("no valid results in %s, skipping compression", fullFilename)
	}

	var cmd *exec.Cmd
	if err := checkZstd(); err != nil {
		cmd = exec.Command("tar", "-C", directory, "-cf", path.Join(directory, fmt.Sprintf("%s.tar.gz", filename)), filename, "--remove-files")
		fullFilename = fmt.Sprintf("%s.tar.gz", fullFilename)
	} else {
		cmd = exec.Command("tar", "--zstd", "-C", directory, "-cf", path.Join(directory, fmt.Sprintf("%s.tar.zst", filename)), filename, "--remove-files")
		fullFilename = fmt.Sprintf("%s.tar.zst", fullFilename)
	}
	log.Debug().Msgf("Compression command: %s", cmd.String())
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return fullFilename, cmd.Run()
}

func getExternalIP(version int) string {
	if version != 4 && version != 6 {
		version = 6
	}
	output, err := exec.Command("curl", fmt.Sprintf("-%d", version), "-m", "5", "-s", "--interface", Iface, "ifconfig.io").CombinedOutput()
	if err != nil {
		log.Error().Err(err).Msgf("get external IP%d addresses failed: %s", version, string(output))
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

// IntOrString is a helper to unmarshal JSON that may provide an integer
// either as a number or as a quoted string.
type IntOrString int

func (i *IntOrString) UnmarshalJSON(b []byte) error {
	// handle null/empty
	s := strings.TrimSpace(string(b))
	if s == "" || s == "null" {
		*i = 0
		return nil
	}
	// If it's a quoted string, trim quotes
	if len(s) >= 2 && s[0] == '"' && s[len(s)-1] == '"' {
		s = strings.Trim(s, "\"")
	}
	// Try parse as integer from string form
	if v, err := strconv.Atoi(s); err == nil {
		*i = IntOrString(v)
		return nil
	}
	// Fallback: try decoding as json.Number (handles numeric tokens)
	var num json.Number
	if err := json.Unmarshal(b, &num); err == nil {
		if iv, err := num.Int64(); err == nil {
			*i = IntOrString(iv)
			return nil
		}
	}
	return fmt.Errorf("cannot unmarshal %s", string(b))
}

type MTRResult struct {
	Report struct {
		Hubs []struct {
			// mtr 0.93 uses "count" as string
			// mtr 0.95 uses "count" as integer
			Count IntOrString `json:"count"`
			Host  string      `json:"host"`
		}
	}
}

func getStarlinkIPv6ActiveGateway() string {
	log.Info().Msg("Getting Starlink IPv6 active gateway")
	cmd, err := exec.Command("mtr", "ipv6.google.com", "-n", "-m", IPv6GatewayHopCount, "-I", Iface, "-c", "1", "--json").CombinedOutput()
	if err != nil {
		log.Error().Err(err).Msgf("mtr failed: %s", string(cmd))
	} else {
		var mtrOutput MTRResult
		err = json.Unmarshal([]byte(string(cmd)), &mtrOutput)
		if err != nil {
			log.Error().Err(err).Msg("Error unmarshalling mtr output")
			return ""
		}
		for _, h := range mtrOutput.Report.Hubs {
			if strconv.Itoa(int(h.Count)) == IPv6GatewayHopCount {
				return h.Host
			}
		}
	}

	log.Info().Msg("gateway not detected using mtr, trying traceroute")

	output, err := exec.Command("traceroute",
		"-6",
		"-i", Iface,
		"ipv6.google.com",
		"-n",
		"-m", IPv6GatewayHopCount,
		"-f", IPv6GatewayHopCount,
		"-q", "1").CombinedOutput()
	if err != nil {
		log.Error().Err(err).Msgf("traceroute failed: %s", string(output))
		return ""
	}
	tracerouteResult := ""
	tracerouteResult = string(output)
	gateway := strings.Split(tracerouteResult, "\n")[len(strings.Split(tracerouteResult, "\n"))-2]
	gateway = strings.Split(gateway, " ")[3]
	if gateway == "*" || net.ParseIP(gateway).To16() == nil {
		log.Error().Msg("traceroute failed to get gateway")
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
	} else if !ActiveDish {
		// With the rollout of standby mode, there are fewer inactive dishes.
		// Inactive dishes cannot reach the Internet, but they can reach 100.64.0.1 or 198.54.100.0 (pop.anycast.starlinkisp.net).
		if RouterGrpcAddrPort != "" {
			exporter, err := NewGrpcClient(RouterGrpcAddrPort)
			if err != nil {
				log.Error().Err(err).Msg("Error creating gRPC client to Starlink router")
				return defaultIPv4CGNATGateway
			}
			ipv6WanAddress := exporter.CollectIPv6WanAddress()
			log.Info().Msgf("IPv6 WAN CIDR from Starlink router: %s", ipv6WanAddress)
			_, ipnet, err := net.ParseCIDR(ipv6WanAddress)
			if err != nil {
				log.Error().Err(err).Msg("Error parsing IPv6 WAN address CIDR")
				return defaultIPv4CGNATGateway
			}
			IPVersion = 6
			// technically, this is not the external IP, but we use it to get the PoP info
			externalIP = ipnet.IP.String()
			// we still use the default CGNAT gateway for inactive dish
			gatewayIP = defaultIPv4CGNATGateway
			log.Info().Msgf("External IPv6 address from Starlink router: %s, gateway IP: %s", externalIP, gatewayIP)
		} else {
			// inactive dish, also bypassed, so no RouterGrpcAddrPort
			// in this case, we collect the IPv6 address from the interface
			addrs, err := net.InterfaceAddrs()
			if err != nil {
				log.Error().Err(err).Msg("Error getting interface addresses")
				// we cannot get interface addresses, so we assume IPv4 CGNAT
				IPVersion = 4
				gatewayIP = defaultIPv4CGNATGateway
			}
			for _, a := range addrs {
				if ipnet, ok := a.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
					if ipnet.IP.To16() != nil {
						// this is an IPv6 address
						_, ok := geoipClient.GetPopByCIDR(ipnet.IP.To16().String())
						if ok {
							// this is a Starlink IPv6 address
							IPVersion = 6
							externalIP = ipnet.IP.To16().String()
							// we still use the default CGNAT gateway for inactive dish
							gatewayIP = defaultIPv4CGNATGateway
							log.Info().Msgf("IPv6 address for inactive dish: %s", externalIP)
							break
						}
					}
				}
			}
		}
	} else {
		// Active dish, probe IPv6 active gateway through mtr or traceroute
		externalIPv6 = getExternalIP(6)
		if ipExist(externalIPv6) {
			// If external IPv6 address exists on the interface
			IPVersion = 6

			log.Info().Msgf("External IPv6 address: %s", externalIPv6)
			externalIP = externalIPv6
			gatewayIP = getStarlinkIPv6ActiveGateway()
		} else {
			externalIPv4 = getExternalIP(4)
			if net.ParseIP(externalIPv4).To4() != nil {
				// CGNAT IPv4 does not exist on the interface locally
				IPVersion = 4

				log.Info().Msgf("External IPv4 address: %s", externalIPv4)
				externalIP = externalIPv4
				gatewayIP = defaultIPv4CGNATGateway
			}
		}
	}

	if externalIP != "" {
		PoP = getStarlinkPoP(externalIP)
	} else {
		PoP = ""
	}
	StarlinkGateway = gatewayIP

	log.Info().Msgf("Starlink gateway: %s, PoP: %s, external IP: %s", gatewayIP, PoP, externalIP)
	return gatewayIP
}

func notify() {
	if NotifyURL == "" {
		return
	}
	client := http.NewClient()
	client.HTTPClient.Timeout = 10 * time.Second
	client.RetryMax = 3

	resp, err := client.Get(NotifyURL)
	if err != nil {
		log.Error().Err(err).Msg("Error sending notify request")
		return
	}
	defer resp.Body.Close()
	log.Debug().Msgf("Notify response status: %s", resp.Status)
}
