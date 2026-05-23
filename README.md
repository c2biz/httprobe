# httprobe

Take a list of domains and probe for working http and https servers.

## Install

```
▶ go install github.com/tomnomnom/httprobe@latest
```

## Basic Usage

httprobe accepts line-delimited domains on `stdin`:

```
▶ cat recon/example/domains.txt
example.com
example.edu
example.net
▶ cat recon/example/domains.txt | httprobe
http://example.com
http://example.net
http://example.edu
https://example.com
https://example.edu
https://example.net
```

## Extra Probes

By default httprobe checks for HTTP on port 80 and HTTPS on port 443. You can add additional
probes with the `-p` flag by specifying a protocol and port pair:

```
▶ cat domains.txt | httprobe -p http:81 -p https:8443
```

## Concurrency

You can set the concurrency level with the `-c` flag:

```
▶ cat domains.txt | httprobe -c 50
```

Note: concurrency is split evenly between HTTPS and HTTP workers. With `--prefer-https`, HTTP workers only handle HTTPS failures, so effective concurrency is roughly `c/2`. To get 50 concurrent probes with `--prefer-https`, use `-c 100`.

## Timeout

You can change the timeout by using the `-t` flag and specifying a timeout in milliseconds:

```
▶ cat domains.txt | httprobe -t 20000
```

## Skipping Default Probes

If you don't want to probe for HTTP on port 80 or HTTPS on port 443, you can use the
`-s` flag. You'll need to specify the probes you do want using the `-p` flag:

```
▶ cat domains.txt | httprobe -s -p https:8443
```

## Prefer HTTPS

Sometimes you don't care about checking HTTP if HTTPS is working. You can do that with the `--prefer-https` flag:

```
▶ cat domains.txt | httprobe --prefer-https
```

## Response Info

You can include extra information in the output with `-status`, `-server`, and `-title`:

```
▶ cat domains.txt | httprobe -status -server -title
https://example.com [200] [nginx] [Example Domain]
```

## Proxy

Route requests through an HTTP or SOCKS5 proxy:

```
▶ cat domains.txt | httprobe -proxy http://proxy:8080
▶ cat domains.txt | httprobe -proxy socks5://proxy:1080
```

## Rate Limiting

Control request rate with `-rate` (requests per second). The limit is global across both the HTTPS and HTTP worker pools, so each `--prefer-https` failure that falls back to HTTP consumes two tokens (one for the HTTPS attempt, one for the HTTP fallback).

```
▶ cat domains.txt | httprobe -rate 5
```

Quick reference (mean interval — actual interval per probe depends on `-jitter`):

| `-rate` | Mean interval | /24 scan (worst case) |
|---------|---------------|------------------------|
| `10`    | 100ms         | ~1 min                 |
| `5`     | 200ms         | ~2 min                 |
| `1`     | 1s            | ~9 min                 |
| `0.5`   | 2s            | ~17 min                |
| `0.1`   | 10s           | ~85 min (~1.4 h)       |
| `0.05`  | 20s           | ~170 min (~2.8 h)      |
| `0.025` | 40s           | ~5.7 h                 |
| `0.01`  | 100s          | ~14.2 h                |

Wall-clock for any list is roughly `probes ÷ rate` seconds, where `probes = list_size × 2` in the worst case (every host needs both an HTTPS attempt and an HTTP fallback) or `list_size × 1` in the best case (every host responds to HTTPS with `--prefer-https`).

### Jitter

Strict rate limiting produces near-perfectly-periodic traffic, which is itself a detection signal for behavioural analytics (beaconing detection). Use `-jitter` to randomize the inter-probe interval while preserving the mean rate. The value is a fraction in `[0, 1]`:

```
▶ cat domains.txt | httprobe -rate 0.05 -jitter 0.5
```

With `-rate R -jitter J`, each interval is drawn uniformly from `[(1-J)/R, (1+J)/R]`. The mean stays at `1/R`, so the headline `-rate` number remains meaningful. `-jitter` requires `-rate` to be set.

| Setting                        | Per-probe interval | Mean    |
|--------------------------------|--------------------|---------|
| `-rate 0.05`                   | exactly 20s        | 20s     |
| `-rate 0.05 -jitter 0.5`       | uniform `[10s, 30s]` | 20s   |
| `-rate 0.05 -jitter 1.0`       | uniform `[0s, 40s]`  | 20s   |

### Stats

Use `-stats` to print probe count and effective rate to stderr — both periodically (every 60s) during a run and once at the end:

```
▶ cat domains.txt | httprobe -rate 0.05 -stats
...
[stats] 30 probes in 600.0s = 0.050 req/s
[stats] 60 probes in 1200.0s = 0.050 req/s
[stats] 474 probes in 9461.2s = 0.050 req/s (final)
```

Counts every outbound probe — including HTTPS attempts that fail and fall back to HTTP — which is the right number for stealth budgeting.

## Docker

Build the docker container:

```
▶ docker build -t httprobe .
```

Run the container, passing the contents of a file into stdin of the process inside the container. `-i` is required to correctly map `stdin` into the container and to the `httprobe` binary.

```
▶ cat domains.txt | docker run -i httprobe <args>
```

## Usage

```
Usage of ./httprobe:
  -A string
        HTTP User-Agent to use (default "httprobe")
  -c int
        set the concurrency level (split equally between HTTPS and HTTP requests) (default 20)
  -jitter float
        randomize interval as fraction of 1/rate (0..1, requires -rate)
  -method string
        HTTP method to use (default "GET")
  -p value
        add additional probe (e.g. -p proto:port or -p <small|large|xlarge>)
  -prefer-https
        only try plain HTTP if HTTPS fails
  -proxy string
        proxy URL (e.g., http://proxy:8080 or socks5://proxy:1080)
  -rate float
        requests per second (0 = unlimited)
  -s    skip the default probes (http:80 and https:443)
  -server
        show Server header
  -stats
        print probe count and effective rate to stderr (periodic + final)
  -status
        show HTTP status code
  -t int
        timeout (milliseconds) (default 10000)
  -title
        show page title
```