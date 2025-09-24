package main

/*
To run this code, save it in a file named main.go and execute the following commands:
	1. go mod init app
	2. go mod tidy
	3. go run main.go (or src/go/main.go)
*/

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"strings"
	"time"
)

// region Provider and Config Structs
type Provider struct {
	Uri        string `json:"uri"`
	Username   string `json:"username,omitempty"`
	Password   string `json:"passwd,omitempty"`
	Domain     string `json:"domain,omitempty"`
	Iid6       string `json:"iid6,omitempty"`
	Iid6Masked net.IP `json:"-"` // will be set later if Iid6 is valid
}

type Config struct {
	Username   string     // env.USER_NAME
	Password   string     // env.USER_PASSWORD
	Domain     string     // env.USER_DOMAIN_NAME
	Providers  []Provider // env.PROVIDERS (JSON-Array)
	LogVerbose bool       // env.LOG_VERBOSE (optional, default: false)
}

// Loads environment variables and deserializes them into a Config struct
func LoadConfigFromEnv() (*Config, error) {
	cfg := &Config{}
	cfg.Username = os.Getenv("USER_NAME")
	if cfg.Username == "" {
		cfg.Username = "user"
	}

	cfg.Password = os.Getenv("USER_PASSWORD")
	if cfg.Password == "" {
		return nil, fmt.Errorf("USER_PASSWORD is required and must not be empty")
	}

	cfg.Domain = os.Getenv("USER_DOMAIN_NAME")
	if cfg.Domain == "" {
		cfg.Domain = "any.domain"
	}

	providersJson := os.Getenv("PROVIDERS")
	if providersJson != "" {
		err := json.Unmarshal([]byte(providersJson), &cfg.Providers)
		if err != nil {
			return nil, err
		}
	}

	// LOG_VERBOSE: "true" (case-insensitive) => true, else false
	logVerboseEnv := strings.ToLower(os.Getenv("LOG_VERBOSE"))
	cfg.LogVerbose = logVerboseEnv == "true"

	if len(cfg.Providers) == 0 {
		return nil, fmt.Errorf("no provider defined (PROVIDERS is empty or missing)")
	}
	for i, p := range cfg.Providers {
		if strings.TrimSpace(p.Uri) == "" {
			return nil, fmt.Errorf("provider at index %d is missing a URI", i)
		} else {
			if p.Iid6 != "" {
				//Parse and validate the interface ID.
				ifaceIP := net.ParseIP("::" + p.Iid6)
				if ifaceIP == nil || ifaceIP.To16() == nil {
					return nil, fmt.Errorf("invalid interface ID: %s", p.Iid6)
				} else {
					p.Iid6Masked = ifaceIP
					cfg.Providers[i] = p // Update the slice with the modified provider

					if cfg.LogVerbose {
						log.Printf("Provider[%d]: Parsed IID6 %s to %s\n", i, p.Iid6, p.Iid6Masked.String())
					}
				}
			}
		}
	}
	return cfg, nil
}

// endregion

// region main
var (
	config    *Config
	globalErr error
)

func main() {
	config, globalErr = LoadConfigFromEnv()
	if globalErr != nil {
		log.Printf("Config error: %v", globalErr)
	} else {
		if config.LogVerbose {
			log.Println("Verbose logging enabled. WARNING: This may expose sensitive information in logs. Use it with caution.")
		} else {
			log.Println("Verbose logging disabled")
		}

		// Log provider attributes without username and password
		for i, p := range config.Providers {
			var iid6Parsed string
			if p.Iid6Masked != nil {
				iid6Parsed = p.Iid6Masked.String()
			} else {
				iid6Parsed = ""
			}

			log.Printf("Provider[%d]: uri=%s, domain=%s, iid6=%s", i, p.Uri, p.Domain, iid6Parsed)
		}
	}

	http.HandleFunc("/health", healthEndpoint)
	http.HandleFunc("/update", dyndnsHandler)

	port := "8080"
	log.Printf("app started on :%s\n", port)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}

// endregion

// region healthEndpoint

func healthEndpoint(w http.ResponseWriter, r *http.Request) {
	if globalErr == nil {
		w.WriteHeader(http.StatusOK)
		fmt.Fprintln(w, "OK")
	} else {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintln(w, "UNHEALTHY: config error. "+globalErr.Error())
	}
}

// endregion

// region dyndnsHandler

// region QueryParams
// Pair of partially resolved URI and Provider
type QueryParams struct {
	Username      string     // mandatory
	Password      string     // mandatory
	Domain        string     // mandatory
	IpAddr        string     // optional, one of IpAddr or Ip6Addr must be set
	Ip6Addr       string     // optional, one of IpAddr or Ip6Addr must be set
	Ip6LanPrefix  string     // optional
	Ip6LanNetwork *net.IPNet // optional, derived from Ip6LanPrefix
	Dualstack     string     // optional
}

// Parse and validate QueryParams from http.Request
func ParseQueryParams(r *http.Request) (*QueryParams, error) {
	q := r.URL.Query()
	params := &QueryParams{
		Username:      q.Get("username"),
		Password:      q.Get("passwd"),
		Domain:        q.Get("domain"),
		IpAddr:        q.Get("ipaddr"),
		Ip6Addr:       q.Get("ip6addr"),
		Ip6LanPrefix:  q.Get("ip6lanprefix"),
		Ip6LanNetwork: nil, // will be set later if Ip6LanPrefix is valid
		Dualstack:     q.Get("dualstack"),
	}
	// Validate mandatory fields
	if params.Username == "" {
		return nil, fmt.Errorf("missing mandatory query param: username")
	}
	if params.Password == "" {
		return nil, fmt.Errorf("missing mandatory query param: passwd")
	}
	if params.Domain == "" {
		return nil, fmt.Errorf("missing mandatory query param: domain")
	}
	// At least one of IpAddr or Ip6Addr must be set
	if params.IpAddr == "" && params.Ip6Addr == "" {
		return nil, fmt.Errorf("either ipaddr or ip6addr must be set")
	}

	// parse ip6lanprefix if set
	if params.Ip6LanPrefix != "" {
		//e.g. "cafe:babe:dead:beef::/64" or "babe:beef::/32"
		_, network, err := net.ParseCIDR(params.Ip6LanPrefix)
		if err != nil {
			return nil, fmt.Errorf("invalid CIDR prefix: %v", err)
		} else if network.IP.To16() == nil {
			// Ensure the prefix is for IPv6.
			return nil, fmt.Errorf("the provided CIDR %s is not an IPv6 prefix", params.Ip6LanPrefix)
		} else {
			params.Ip6LanNetwork = network
		}
	}

	return params, nil
}

// endregion

// region StatusTracker
// Tracks status and severity for DynDNS responses
type StatusTracker struct {
	SeverityMap map[string]int
	Highest     int
	FinalStatus string
	ResponseIp  string
}

func NewStatusTracker(defaultIp string) *StatusTracker {
	// Severity values according to DynDNS v2 protocol (https://help.dyn.com/remote-access-api/return-codes/)
	return &StatusTracker{
		SeverityMap: map[string]int{
			"badauth":  12,
			"notfqdn":  11,
			"nohost":   10,
			"numhost":  9,
			"abuse":    8,
			"badagent": 7,
			"!yours":   6,
			"!donator": 5,
			"911":      4,
			"dnserr":   3,
			"unknown":  2,
			"good":     1,
			"ok":       0,
			"nochg":    -1,
		},
		Highest:     -1,
		FinalStatus: "nochg " + defaultIp,
		ResponseIp:  defaultIp,
	}
}

// Checks and updates severity and finalStatus
func (s *StatusTracker) CheckStatus(result string, exactReturnCodeMatch bool) {
	status := "unknown"
	sev := s.SeverityMap[status] // fallback
	if exactReturnCodeMatch {
		for k := range s.SeverityMap {
			if result == k {
				sev = s.SeverityMap[k]
				status = k
				break
			}
		}
	} else {
		for k := range s.SeverityMap {
			if strings.HasPrefix(result, k) || strings.Contains(result, k) {
				sev = s.SeverityMap[k]
				status = k
				break
			}
		}
	}
	log.Println("Matched return code: " + status)
	if sev > s.Highest {
		s.Highest = sev
		switch status {
		case "good", "nochg":
			s.FinalStatus = status + " " + s.ResponseIp
		default:
			s.FinalStatus = status
		}
	}
}

// endregion

// region IPv6 Helper
// combineIPv6 combines an IPv6 CIDR prefix with an interface ID.
func combinePrefixAndIID6(network net.IPNet, ifaceIP net.IP) (string, error) {
	//  Validate that the interface ID doesn't overlap with the prefix.
	// We do this by masking the interface IP with the network mask.
	// If the result is not '::', it means the interface ID has bits
	// set in the prefix part, which is an invalid input.
	maskedIfaceIP := ifaceIP.Mask(network.Mask)
	if maskedIfaceIP.String() != "::" {
		return "", fmt.Errorf("interface ID contains bits that overlap with the prefix")
	}

	// Get the prefix length in bits and calculate the byte start index.
	prefixLen, _ := network.Mask.Size()
	startIndex := prefixLen / 8

	// Combine the two parts at the binary level.
	finalIP := make(net.IP, net.IPv6len)

	// Copy the network prefix bytes.
	copy(finalIP, network.IP)

	// Bitwise OR the interface ID bytes with the final IP.
	// This efficiently combines the two parts.
	ifaceIP16 := ifaceIP.To16()
	for i := startIndex; i < net.IPv6len; i++ {
		finalIP[i] = finalIP[i] | ifaceIP16[i]
	}

	return finalIP.String(), nil
}

// endregion

func dyndnsHandler(w http.ResponseWriter, r *http.Request) {
	log.Println("[REQUESTOR] " + r.RemoteAddr)
	if globalErr != nil {
		log.Println("UNHEALTHY: config error. " + globalErr.Error())
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintln(w, "UNHEALTHY: config error. "+globalErr.Error())
		return
	}
	if config.LogVerbose {
		log.Printf("[REQUESTOR] Full URL: %s\n", r.URL.String())
	}

	query, err := ParseQueryParams(r)
	if err != nil {
		log.Println("[ERROR] " + err.Error())
		http.Error(w, "badauth", http.StatusBadRequest)
		return
	} else if config.LogVerbose {
		if query.Ip6LanNetwork != nil {
			log.Printf("[REQUEST] Parsed Ip6LanNetwork: %s\n", query.Ip6LanNetwork.String())
		}
	}
	// Check if query params match config
	if (query.Username != config.Username) || (query.Password != config.Password) || (query.Domain != config.Domain) {
		log.Println("[ERROR] Query parameters do not match configuration")
		if config.LogVerbose {
			if query.Username != config.Username {
				log.Printf("query.Username=%s, expected=%s", query.Username, config.Username)
			}
			if query.Password != config.Password {
				log.Printf("query.Password=%s, expected=%s", query.Password, config.Password)
			}
			if query.Domain != config.Domain {
				log.Printf("query.Domain=%s, expected=%s", query.Domain, config.Domain)
			}
		}
		http.Error(w, "badauth", http.StatusUnauthorized)
		return
	}

	responseIp := query.IpAddr
	if responseIp == "" {
		responseIp = query.Ip6Addr
	}

	tracker := NewStatusTracker(responseIp)

	for i, p := range config.Providers {
		uri := p.Uri
		uri = strings.ReplaceAll(uri, "<domain>", p.Domain)
		uri = strings.ReplaceAll(uri, "<ipaddr>", query.IpAddr)
		var ip6addr string
		lazyWarning := ""
		var lazyError error
		lazyError = nil
		if p.Iid6Masked != nil {
			if query.Ip6LanNetwork == nil {
				lazyWarning = "Provider requires IID6, but no ip6lanprefix was provided in the request. Using empty ip6addr for request."
				ip6addr = ""
			} else {
				ip6addr, lazyError = combinePrefixAndIID6(*query.Ip6LanNetwork, p.Iid6Masked)
				if config.LogVerbose && (ip6addr != "") && (lazyError != nil) {
					log.Printf("[REQUEST] Parsed Ip6LanNetwork: %s\n", query.Ip6LanNetwork.String())
				}
			}
		} else {
			ip6addr = query.Ip6Addr
		}
		uri = strings.ReplaceAll(uri, "<ip6addr>", ip6addr)
		uri = strings.ReplaceAll(uri, "<ip6lanprefix>", query.Ip6LanPrefix)
		uri = strings.ReplaceAll(uri, "<dualstack>", query.Dualstack)

		loggingUri := uri
		loggingUri = strings.ReplaceAll(loggingUri, "<username>", "*****")
		loggingUri = strings.ReplaceAll(loggingUri, "<passwd>", "*****")
		if lazyWarning != "" {
			log.Printf("[WARNING] Index=%d URL=%s Warning=%s\n", i, loggingUri, lazyWarning)
		}
		log.Printf("[REQUEST] Index=%d URL=%s\n", i, loggingUri)
		if lazyError != nil {
			log.Printf("[ERROR] Index=%d URL=%s Error=%v\n", i, loggingUri, lazyError)
			tracker.CheckStatus("911", true)
			continue
		}

		uri = strings.ReplaceAll(uri, "<username>", p.Username)
		uri = strings.ReplaceAll(uri, "<passwd>", p.Password)

		// Make HTTP GET request with 60s timeout
		httpClient := &http.Client{Timeout: 60 * time.Second}
		resp, err := httpClient.Get(uri)
		if err != nil {
			log.Printf("[ERROR] Index=%d URL=%s Error=%v\n", i, loggingUri, err)
			tracker.CheckStatus("911", true)
			continue
		}
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()

		if config.LogVerbose {
			//log response headers
			log.Printf("[HEADERS] Index=%d URL=%s Status=%d Headers:", i, loggingUri, resp.StatusCode)
			for k, v := range resp.Header {
				log.Printf("    %s: %s", k, strings.Join(v, ", "))
			}
		}

		var result string
		exactReturnCodeMatch := false
		// 1. check for exact return code match in header DDNSS-Response
		// Extended evaluation: Header "DDNSS-Response" and "DDNSS-Message"
		ddnssResponse := resp.Header.Get("DDNSS-Response")
		if ddnssResponse != "" {
			result = ddnssResponse
			exactReturnCodeMatch = true
			log.Printf("[RESPONSE] Index=%d URL=%s Status=%d DDNSS-Response=%s\n", i, loggingUri, resp.StatusCode, ddnssResponse)
			ddnssMessage := resp.Header.Get("DDNSS-Message")
			if ddnssMessage != "" {
				log.Printf("[DDNSS-Message] Index=%d Message=%s\n", i, ddnssMessage)
			}
		} else {
			// 2. Check if a severity attribute exists as a header
			severityFound := ""
			for sev := range tracker.SeverityMap {
				if val := resp.Header.Get(sev); val != "" {
					exactReturnCodeMatch = true
					severityFound = sev
					result = sev
					log.Printf("[RESPONSE] Index=%d URL=%s Status=%d SeverityHeader=%s\n", i, loggingUri, resp.StatusCode, sev)
					break
				}
			}
			if severityFound == "" {
				//3. Fallback to body content
				result = string(body)
				log.Printf("[RESPONSE] Index=%d URL=%s Status=%d Body=%s\n", i, loggingUri, resp.StatusCode, result)
			}
		}

		tracker.CheckStatus(result, exactReturnCodeMatch)
	}

	fmt.Fprintln(w, tracker.FinalStatus)
}

// endregion
