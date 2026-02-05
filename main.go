package main

import (
	"bufio"
	"context"
	"crypto/tls"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"golang.org/x/net/proxy"
	"golang.org/x/time/rate"
)

type probeArgs []string

func (p *probeArgs) Set(val string) error {
	*p = append(*p, val)
	return nil
}

func (p probeArgs) String() string {
	return strings.Join(p, ",")
}

func main() {

	// concurrency flag
	var concurrency int
	flag.IntVar(&concurrency, "c", 20, "set the concurrency level (split equally between HTTPS and HTTP requests)")

	// probe flags
	var probes probeArgs
	flag.Var(&probes, "p", "add additional probe (e.g. -p proto:port or -p <small|large|xlarge>)")

	// skip default probes flag
	var skipDefault bool
	flag.BoolVar(&skipDefault, "s", false, "skip the default probes (http:80 and https:443)")

	// timeout flag
	var to int
	flag.IntVar(&to, "t", 10000, "timeout (milliseconds)")

	// prefer https
	var preferHTTPS bool
	flag.BoolVar(&preferHTTPS, "prefer-https", false, "only try plain HTTP if HTTPS fails")

	// HTTP method to use
	var method string
	flag.StringVar(&method, "method", "GET", "HTTP method to use")

	// HTTP User-Agent to use
	var userAgent string
	flag.StringVar(&userAgent, "A", "httprobe", "HTTP User-Agent to use")

	// HTTP/SOCKS5 proxy to use
	var proxyURL string
	flag.StringVar(&proxyURL, "proxy", "", "proxy URL (e.g., http://proxy:8080 or socks5://proxy:1080)")

	// extra output flags
	var showStatus bool
	flag.BoolVar(&showStatus, "status", false, "show HTTP status code")

	var showServer bool
	flag.BoolVar(&showServer, "server", false, "show Server header")

	var showTitle bool
	flag.BoolVar(&showTitle, "title", false, "show page title")

	// rate limiting
	var rateLimit float64
	flag.Float64Var(&rateLimit, "rate", 0, "requests per second (0 = unlimited)")

	flag.Parse()

	// make an actual time.Duration out of the timeout
	timeout := time.Duration(to * 1000000)

	var tr = &http.Transport{
		MaxIdleConns:      30,
		IdleConnTimeout:   time.Second,
		DisableKeepAlives: true,
		TLSClientConfig:   &tls.Config{InsecureSkipVerify: true},
		DialContext: (&net.Dialer{
			Timeout:   timeout,
			KeepAlive: time.Second,
		}).DialContext,
	}

	// Configure proxy if provided
	if proxyURL != "" {
		proxyParsed, err := url.Parse(proxyURL)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Invalid proxy URL: %s\n", err)
			os.Exit(1)
		}

		if proxyParsed.Scheme == "socks5" {
			// SOCKS5 proxy - use custom dialer
			dialer, err := proxy.FromURL(proxyParsed, proxy.Direct)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Failed to create SOCKS5 dialer: %s\n", err)
				os.Exit(1)
			}
			if contextDialer, ok := dialer.(proxy.ContextDialer); ok {
				tr.DialContext = contextDialer.DialContext
			} else {
				tr.DialContext = func(ctx context.Context, network, addr string) (net.Conn, error) {
					return dialer.Dial(network, addr)
				}
			}
		} else {
			// HTTP/HTTPS proxy
			tr.Proxy = http.ProxyURL(proxyParsed)
		}
	}

	re := func(req *http.Request, via []*http.Request) error {
		return http.ErrUseLastResponse
	}

	client := &http.Client{
		Transport:     tr,
		CheckRedirect: re,
		Timeout:       timeout,
	}

	// set up rate limiter (nil if unlimited)
	var limiter *rate.Limiter
	if rateLimit > 0 {
		limiter = rate.NewLimiter(rate.Limit(rateLimit), 1)
	}

	// domain/port pairs are initially sent on the httpsURLs channel.
	// If they are listening and the --prefer-https flag is set then
	// no HTTP check is performed; otherwise they're put onto the httpURLs
	// channel for an HTTP check.
	httpsURLs := make(chan string)
	httpURLs := make(chan string)
	output := make(chan string)

	// HTTPS workers
	var httpsWG sync.WaitGroup
	for i := 0; i < concurrency/2; i++ {
		httpsWG.Add(1)

		go func() {
			for u := range httpsURLs {
				if limiter != nil {
					limiter.Wait(context.Background())
				}

				// always try HTTPS first
				withProto := "https://" + u
				result := probeURL(client, withProto, method, userAgent, showTitle)
				if result.success {
					output <- formatOutput(withProto, result, showStatus, showServer, showTitle)

					// skip trying HTTP if --prefer-https is set
					if preferHTTPS {
						continue
					}
				}

				httpURLs <- u
			}

			httpsWG.Done()
		}()
	}

	// HTTP workers
	var httpWG sync.WaitGroup
	for i := 0; i < concurrency/2; i++ {
		httpWG.Add(1)

		go func() {
			for u := range httpURLs {
				if limiter != nil {
					limiter.Wait(context.Background())
				}
				withProto := "http://" + u
				result := probeURL(client, withProto, method, userAgent, showTitle)
				if result.success {
					output <- formatOutput(withProto, result, showStatus, showServer, showTitle)
				}
			}

			httpWG.Done()
		}()
	}

	// Close the httpURLs channel when the HTTPS workers are done
	go func() {
		httpsWG.Wait()
		close(httpURLs)
	}()

	// Output worker
	var outputWG sync.WaitGroup
	outputWG.Add(1)
	go func() {
		for o := range output {
			fmt.Println(o)
		}
		outputWG.Done()
	}()

	// Close the output channel when the HTTP workers are done
	go func() {
		httpWG.Wait()
		close(output)
	}()

	// accept domains on stdin
	sc := bufio.NewScanner(os.Stdin)
	for sc.Scan() {
		domain := strings.ToLower(sc.Text())

		// submit standard port checks
		if !skipDefault {
			httpsURLs <- domain
		}

		// Adding port templates
		xlarge := []string{"81", "300", "591", "593", "832", "981", "1010", "1311", "2082", "2087", "2095", "2096", "2480", "3000", "3128", "3333", "4243", "4567", "4711", "4712", "4993", "5000", "5104", "5108", "5800", "6543", "7000", "7396", "7474", "8000", "8001", "8008", "8014", "8042", "8069", "8080", "8081", "8088", "8090", "8091", "8118", "8123", "8172", "8222", "8243", "8280", "8281", "8333", "8443", "8500", "8834", "8880", "8888", "8983", "9000", "9043", "9060", "9080", "9090", "9091", "9200", "9443", "9800", "9981", "12443", "16080", "18091", "18092", "20720", "28017"}
		large := []string{"81", "591", "2082", "2087", "2095", "2096", "3000", "8000", "8001", "8008", "8080", "8083", "8443", "8834", "8888"}
		small := []string{"7000", "7001", "8000", "8001", "8008", "8080", "8083", "8443", "8834", "8888", "10000"}

		// submit any additional proto:port probes
		for _, p := range probes {
			switch p {
			case "xlarge":
				for _, port := range xlarge {
					httpsURLs <- fmt.Sprintf("%s:%s", domain, port)
				}
			case "large":
				for _, port := range large {
					httpsURLs <- fmt.Sprintf("%s:%s", domain, port)
				}
			case "small":
				for _, port := range small {
					httpsURLs <- fmt.Sprintf("%s:%s", domain, port)
				}
			default:
				pair := strings.SplitN(p, ":", 2)
				if len(pair) != 2 {
					continue
				}

				// This is a little bit funny as "https" will imply an
				// http check as well unless the --prefer-https flag is
				// set. On balance I don't think that's *such* a bad thing
				// but it is maybe a little unexpected.
				if strings.ToLower(pair[0]) == "https" {
					httpsURLs <- fmt.Sprintf("%s:%s", domain, pair[1])
				} else {
					httpURLs <- fmt.Sprintf("%s:%s", domain, pair[1])
				}
			}
		}
	}

	// once we've sent all the URLs off we can close the
	// input/httpsURLs channel. The workers will finish what they're
	// doing and then call 'Done' on the WaitGroup
	close(httpsURLs)

	// check there were no errors reading stdin (unlikely)
	if err := sc.Err(); err != nil {
		fmt.Fprintf(os.Stderr, "failed to read input: %s\n", err)
	}

	// Wait until the output waitgroup is done
	outputWG.Wait()
}

type probeResult struct {
	success bool
	status  int
	server  string
	title   string
}

func probeURL(client *http.Client, url, method, userAgent string, needBody bool) probeResult {
	result := probeResult{}

	req, err := http.NewRequest(method, url, nil)
	if err != nil {
		return result
	}
	req.Header.Add("User-Agent", userAgent)
	req.Header.Add("Connection", "close")
	req.Close = true

	resp, err := client.Do(req)
	if err != nil {
		return result
	}
	defer resp.Body.Close()

	result.success = true
	result.status = resp.StatusCode
	result.server = resp.Header.Get("Server")

	if needBody {
		// read limited body for title extraction
		body, err := ioutil.ReadAll(io.LimitReader(resp.Body, 4096))
		if err == nil {
			result.title = extractTitle(string(body))
		}
	} else {
		io.Copy(ioutil.Discard, resp.Body)
	}

	return result
}

func extractTitle(body string) string {
	lower := strings.ToLower(body)
	start := strings.Index(lower, "<title>")
	if start == -1 {
		return ""
	}
	start += 7
	end := strings.Index(lower[start:], "</title>")
	if end == -1 {
		return ""
	}
	title := strings.TrimSpace(body[start : start+end])
	// collapse whitespace
	title = strings.Join(strings.Fields(title), " ")
	return title
}

func formatOutput(url string, r probeResult, showStatus, showServer, showTitle bool) string {
	out := url
	if showStatus {
		out += fmt.Sprintf(" [%d]", r.status)
	}
	if showServer {
		server := r.server
		if server == "" {
			server = "-"
		}
		out += fmt.Sprintf(" [%s]", server)
	}
	if showTitle {
		title := r.title
		if title == "" {
			title = "-"
		}
		out += fmt.Sprintf(" [%s]", title)
	}
	return out
}
