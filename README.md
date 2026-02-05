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

Control request rate with `-rate` (requests per second):

```
▶ cat domains.txt | httprobe -rate 5
```

Quick reference:

| `-rate` | Requests per minute | Delay between requests |
|---------|---------------------|------------------------|
| `10`    | 600/min             | 100ms                  |
| `5`     | 300/min             | 200ms                  |
| `1`     | 60/min              | 1s                     |
| `0.5`   | 30/min              | 2s                     |
| `0.1`   | 6/min               | 10s                    |

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
  -status
        show HTTP status code
  -t int
        timeout (milliseconds) (default 10000)
  -title
        show page title
```