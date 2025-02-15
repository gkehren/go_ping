package ping

import (
	"fmt"
	"net"
	"os"
	"sync"
	"time"

	"golang.org/x/net/icmp"
	"golang.org/x/net/ipv4"
	"golang.org/x/net/ipv6"
)

type Pinger struct {
	addr      string
	ipAddr    *net.IPAddr
	interval  time.Duration
	timeout   time.Duration
	count     int
	floodMode bool
	sequence  int
	conn      *icmp.PacketConn
	done      chan bool
	OnRecv    func(*Packet)
	OnFinish  func(*Statistics)
	Stats     Statistics
}

type Packet struct {
	Seq    int
	IPAddr *net.IPAddr
	Rtt    time.Duration
	NBytes int
}

type Statistics struct {
	Addr        string
	PacketsSent int
	PacketsRecv int
	PacketLoss  float64
	MinRtt      time.Duration
	MaxRtt      time.Duration
	AvgRtt      time.Duration
}

func NewPinger(addr string) (*Pinger, error) {
	ipAddr, err := net.ResolveIPAddr("ip", addr)
	if err != nil {
		return nil, err
	}

	return &Pinger{
		addr:     addr,
		ipAddr:   ipAddr,
		interval: time.Second,
		timeout:  time.Second * 5,
		count:    -1, // infinite
		done:     make(chan bool),
	}, nil
}

func (p *Pinger) Run() error {
	conn, err := icmp.ListenPacket(p.network(), "")
	if err != nil {
		return err
	}
	p.conn = conn

	var wg sync.WaitGroup
	wg.Add(1)
	go p.recvICMP(&wg)

	err = p.sendICMP()
	if err != nil {
		p.Stop()
	}

	wg.Wait()
	conn.Close()
	return err
}

func (p *Pinger) Stop() {
	select {
	case <-p.done:
	default:
		close(p.done)
	}
}

func (p *Pinger) protocol() int {
	if p.ipAddr.IP.To4() != nil {
		return 1 // ICMPv4
	}
	return 58 // ICMPv6
}

func (p *Pinger) network() string {
	if p.ipAddr.IP.To4() != nil {
		return "ip4:icmp"
	}
	return "ip6:ipv6-icmp"
}

func (p *Pinger) calculatePacketLoss() {
	if p.Stats.PacketsSent > 0 {
		p.Stats.PacketLoss = float64(p.Stats.PacketsSent-p.Stats.PacketsRecv) / float64(p.Stats.PacketsSent) * 100
	}
}

func (p *Pinger) sendICMP() error {
	icmpType := getICMPType(p.ipAddr)

	if p.floodMode {
		return p.sendICMPFlood(icmpType)
	}

	for {
		select {
		case <-p.done:
			return nil
		default:
			if p.count == 0 {
				p.Stop()
				return nil
			}

			p.sequence++
			msg := icmp.Message{
				Type: icmpType,
				Code: 0,
				Body: &icmp.Echo{
					ID:   os.Getegid() & 0xffff,
					Seq:  p.sequence,
					Data: append(timeToBytes(time.Now()), make([]byte, 56)...),
				},
			}

			wb, err := msg.Marshal(nil)
			if err != nil {
				return err
			}

			checksum := calculateChecksum(wb)
			wb[2] = byte(checksum >> 8)
			wb[3] = byte(checksum & 0xff)

			if _, err := p.conn.WriteTo(wb, p.ipAddr); err != nil {
				return err
			}

			p.Stats.PacketsSent++
			if p.count > 0 {
				p.count--
			}

			time.Sleep(p.interval)
		}
	}
}

func (p *Pinger) sendICMPFlood(icmpType icmp.Type) error {
	for {
		select {
		case <-p.done:
			return nil
		default:
			if p.count == 0 {
				p.Stop()
				return nil
			}

			msg := icmp.Message{
				Type: icmpType,
				Code: 0,
				Body: &icmp.Echo{
					ID:   os.Getegid() & 0xffff,
					Seq:  p.sequence,
					Data: append(timeToBytes(time.Now()), make([]byte, 56)...),
				},
			}

			wb, err := msg.Marshal(nil)
			if err != nil {
				return err
			}

			checksum := calculateChecksum(wb)
			wb[2] = byte(checksum >> 8)
			wb[3] = byte(checksum & 0xff)

			fmt.Print(".")
			if _, err := p.conn.WriteTo(wb, p.ipAddr); err != nil {
				return err
			}

			p.sequence++
			p.Stats.PacketsSent++
			if p.count > 0 {
				p.count--
			}

			if p.interval > 0 {
				time.Sleep(p.interval)
			} else {
				time.Sleep(time.Second / 1000)
			}
		}
	}
}

func (p *Pinger) recvICMP(wg *sync.WaitGroup) {
	defer wg.Done()

	for {
		select {
		case <-p.done:
			if p.OnFinish != nil {
				p.calculatePacketLoss()
				p.OnFinish(&p.Stats)
			}
			return
		default:
			err := p.conn.SetReadDeadline(time.Now().Add(p.timeout))
			if err != nil {
				continue
			}

			buf := make([]byte, 1500)
			n, _, err := p.conn.ReadFrom(buf)
			if err != nil {
				if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
					continue
				}
				return
			}

			recvTime := time.Now()
			rm, err := icmp.ParseMessage(p.protocol(), buf[:n])
			if err != nil {
				continue
			}

			switch rm.Type {
			case ipv4.ICMPTypeEchoReply, ipv6.ICMPTypeEchoReply:
				echo, ok := rm.Body.(*icmp.Echo)
				if !ok {
					continue
				}

				rtt := recvTime.Sub(bytesToTime(echo.Data[:8]))
				p.Stats.PacketsRecv++
				if p.OnRecv != nil {
					p.OnRecv(&Packet{
						Seq:    echo.Seq,
						IPAddr: p.ipAddr,
						Rtt:    rtt,
						NBytes: n,
					})
				}

				if rtt < p.Stats.MinRtt || p.Stats.MinRtt == 0 {
					p.Stats.MinRtt = rtt
				}
				if rtt > p.Stats.MaxRtt {
					p.Stats.MaxRtt = rtt
				}
				p.Stats.AvgRtt = (p.Stats.AvgRtt*time.Duration(p.Stats.PacketsRecv-1) + rtt) / time.Duration(p.Stats.PacketsRecv)
			}
		}
	}
}

// Setters
func (p *Pinger) SetInterval(interval time.Duration) {
	p.interval = interval
}

func (p *Pinger) SetTimeout(timeout time.Duration) {
	p.timeout = timeout
}

func (p *Pinger) SetCount(count int) {
	p.count = count
}

func (p *Pinger) SetFloodMode(enabled bool) {
	p.floodMode = enabled
}

// Getters
func (p *Pinger) Addr() string {
	return p.addr
}

func (p *Pinger) IPAddr() *net.IPAddr {
	return p.ipAddr
}

// Helpers
func getICMPType(ip *net.IPAddr) icmp.Type {
	if ip.IP.To4() != nil {
		return ipv4.ICMPTypeEcho
	}
	return ipv6.ICMPTypeEchoRequest
}

func timeToBytes(t time.Time) []byte {
	sec := t.UnixNano()
	b := make([]byte, 8)
	for i := uint8(0); i < 8; i++ {
		b[i] = byte(sec >> ((7 - i) * 8))
	}
	return b
}

func bytesToTime(b []byte) time.Time {
	var sec int64
	for i := uint8(0); i < 8; i++ {
		sec += int64(b[i]) << ((7 - i) * 8)
	}
	return time.Unix(sec/1e9, sec%1e9)
}

func calculateChecksum(b []byte) uint16 {
	b[2] = 0
	b[3] = 0

	var sum uint32
	for i := 0; i < len(b); i += 2 {
		if i+1 < len(b) {
			sum += uint32(b[i])<<8 | uint32(b[i+1])
		} else {
			sum += uint32(b[i]) << 8
		}
	}
	for sum>>16 != 0 {
		sum = (sum & 0xffff) + (sum >> 16)
	}
	return uint16(^sum)
}
