# Go Ping

A pure Go ping implementation that allows sending ICMP echo requests and receiving ICMP echo replies. This implementation supports both IPv4 and IPv6, and includes features like flood ping and customizable intervals.

## Features

- IPv4 and IPv6 support
- Configurable ping interval and timeout
- Packet count limitation
- Flood ping mode
- Statistics including packet loss and round-trip times
- Root privileges check
- Clean termination with Ctrl+C

## Installation

```bash
git clone https://github.com/gkehren/go_ping.git
cd go_ping
go build -o ping cmd/go_ping/main.go
```

## Usage

The program requires root privileges to run:

```bash
sudo ./ping [options] target
```

### Options

- `-c count`: Stop after sending `count` packets (default: infinite)
- `-i interval`: Wait `interval` between sending each packet (default: 1s)
- `-t timeout`: Timeout waiting for response (default: 100ms)
- `-f`: Flood ping mode (super-user only)
  - Prints a '.' for each request sent
  - Prints a backspace for each reply received
  - When used without `-i`, sends packets as fast as possible

### Examples

Regular ping to Google's DNS:
```bash
sudo ./ping 8.8.8.8
```

Send 5 packets with 500ms interval:
```bash
sudo ./ping -c 5 -i 500ms 8.8.8.8
```

Flood ping (requires root):
```bash
sudo ./ping -f 8.8.8.8
```

Custom timeout:
```bash
sudo ./ping -t 200ms 8.8.8.8
```

## Output Format

### Regular Mode
```
PING 8.8.8.8 (8.8.8.8):
64 bytes from 8.8.8.8: icmp_seq=1 time=15.566ms
64 bytes from 8.8.8.8: icmp_seq=2 time=14.223ms
64 bytes from 8.8.8.8: icmp_seq=3 time=13.897ms
^C
--- 8.8.8.8 ping statistics ---
3 packets transmitted, 3 received, 0% packet loss
rtt min/avg/max = 13.897ms/14.562ms/15.566ms
```

### Flood Mode
```
PING 8.8.8.8 (8.8.8.8):
.......
```

## Error Handling

- Displays an error if target is not specified
- Checks for root privileges
- Handles network errors gracefully
- Proper cleanup on Ctrl+C

## Contributing

Feel free to submit issues and pull requests.

## Requirements

- Go 1.23 or higher
- Root privileges for running the program
- `golang.org/x/net` package