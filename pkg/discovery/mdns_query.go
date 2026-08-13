package discovery

import (
	"context"
	"errors"
	"net"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/miekg/dns"
)

const mdnsMulticastAddress = "224.0.0.251:5353"

type mdnsQueryRequest struct {
	ServiceName string
	Window      time.Duration
}

// mdnsQueryEntry keeps every link needed to prove the PTR -> SRV -> address
// chain. Conversion in MDNSResolver remains the admission trust boundary even
// if the production query implementation changes again.
type mdnsQueryEntry struct {
	PTRName      string
	InstanceName string
	SRVName      string
	HostName     string
	Port         int
	Addresses    []string
	TXT          []string
}

type mdnsQueryResult struct {
	Entries     []mdnsQueryEntry
	Rejected    int
	StartedAt   time.Time
	CompletedAt time.Time
	Outcome     ScanOutcome
	ErrorCode   string
	Err         error
}

func (r mdnsQueryResult) successful() bool {
	return r.Outcome == ScanOutcomeSuccess && r.Err == nil
}

type mdnsQuery interface {
	Scan(context.Context, mdnsQueryRequest) mdnsQueryResult
}

// udpMDNSQuery performs one bounded DNS-SD collection using unicast-response
// mDNS questions on every usable IPv4 interface. Each scan owns and closes its
// sockets; it retains no process-global listener or goroutine.
type udpMDNSQuery struct{}

func newUDPMDNSQuery() mdnsQuery {
	return &udpMDNSQuery{}
}

func (q *udpMDNSQuery) Scan(ctx context.Context, request mdnsQueryRequest) mdnsQueryResult {
	started := time.Now().UTC()
	result := mdnsQueryResult{StartedAt: started}
	finish := func(outcome ScanOutcome, code string, err error) mdnsQueryResult {
		result.CompletedAt = time.Now().UTC()
		result.Outcome = outcome
		result.ErrorCode = code
		result.Err = err
		return result
	}

	if err := ctx.Err(); err != nil {
		return finish(ScanOutcomeCancelled, ErrorCodeCancelled, err)
	}
	window := request.Window
	if window <= 0 {
		window = defaultQueryTimeout
	}
	serviceName := canonicalDNSName(request.ServiceName)
	if serviceName == "" {
		return finish(ScanOutcomeFailed, ErrorCodeQueryFailed, errors.New("empty DNS-SD service name"))
	}

	interfaces := interfaceIPv4Addresses()
	if len(interfaces) == 0 {
		interfaces = []net.IP{nil}
	}

	connections := make([]*net.UDPConn, 0, len(interfaces))
	for _, ip := range interfaces {
		conn, err := net.ListenUDP("udp4", &net.UDPAddr{IP: ip, Port: 0})
		if err != nil {
			continue
		}
		connections = append(connections, conn)
	}
	if len(connections) == 0 {
		return finish(ScanOutcomeFailed, ErrorCodeSocketOpenFailed, errors.New("no mDNS query socket could be opened"))
	}

	scanCtx, cancel := context.WithCancel(ctx)
	var readers sync.WaitGroup
	defer func() {
		cancel()
		for _, conn := range connections {
			_ = conn.Close()
		}
		readers.Wait()
	}()

	query := new(dns.Msg)
	query.Id = 0
	query.RecursionDesired = false
	query.Question = []dns.Question{{
		Name:   serviceName,
		Qtype:  dns.TypePTR,
		Qclass: dns.ClassINET | 0x8000, // request a unicast reply to our per-scan socket
	}}
	wire, err := query.Pack()
	if err != nil {
		return finish(ScanOutcomeFailed, ErrorCodeQueryFailed, err)
	}
	destination, err := net.ResolveUDPAddr("udp4", mdnsMulticastAddress)
	if err != nil {
		return finish(ScanOutcomeFailed, ErrorCodeQueryFailed, err)
	}

	sent := 0
	for _, conn := range connections {
		if _, err := conn.WriteToUDP(wire, destination); err == nil {
			sent++
		}
	}
	if sent == 0 {
		return finish(ScanOutcomeFailed, ErrorCodeSocketSendFailed, errors.New("mDNS query could not be sent"))
	}

	deadline := started.Add(window)
	packets := make(chan *dns.Msg, 128)
	readErrors := make(chan error, len(connections))
	rejectedPackets := make(chan struct{}, 128)
	for _, conn := range connections {
		readers.Add(1)
		go func(conn *net.UDPConn) {
			defer readers.Done()
			buffer := make([]byte, 64*1024)
			for {
				_ = conn.SetReadDeadline(deadline)
				n, _, err := conn.ReadFromUDP(buffer)
				if err != nil {
					if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
						return
					}
					select {
					case readErrors <- err:
					default:
					}
					return
				}
				message := new(dns.Msg)
				if err := message.Unpack(buffer[:n]); err != nil {
					select {
					case rejectedPackets <- struct{}{}:
					default:
					}
					continue
				}
				select {
				case packets <- message:
				case <-scanCtx.Done():
					return
				}
			}
		}(conn)
	}

	timer := time.NewTimer(time.Until(deadline))
	defer timer.Stop()
	records := newMDNSRecordSet()
	readFailureCount := 0
	for {
		select {
		case <-ctx.Done():
			outcome := ScanOutcomeCancelled
			code := ErrorCodeCancelled
			if errors.Is(ctx.Err(), context.DeadlineExceeded) {
				outcome = ScanOutcomeTimedOut
				code = ErrorCodeQueryTimedOut
			}
			return finish(outcome, code, ctx.Err())
		case err := <-readErrors:
			if err != nil {
				readFailureCount++
			}
		case <-rejectedPackets:
			records.rejected++
		case message := <-packets:
			if message != nil {
				records.addMessage(message)
			}
		case <-timer.C:
			entries, rejected := records.entries()
			result.Entries = entries
			result.Rejected = rejected
			if readFailureCount >= len(connections) && len(entries) == 0 {
				return finish(ScanOutcomeFailed, ErrorCodeSocketReadFailed, errors.New("all mDNS query sockets failed"))
			}
			return finish(ScanOutcomeSuccess, ErrorCodeNone, nil)
		}
	}
}

func interfaceIPv4Addresses() []net.IP {
	interfaces, err := net.Interfaces()
	if err != nil {
		return nil
	}
	seen := make(map[string]struct{})
	var out []net.IP
	for _, iface := range interfaces {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagMulticast == 0 {
			continue
		}
		addresses, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, address := range addresses {
			ip, _, err := net.ParseCIDR(address.String())
			if err != nil || ip == nil || ip.To4() == nil || ip.IsLoopback() {
				continue
			}
			key := ip.String()
			if _, ok := seen[key]; ok {
				continue
			}
			seen[key] = struct{}{}
			out = append(out, ip)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].String() < out[j].String() })
	return out
}

type mdnsRecordSet struct {
	ptrs      []*dns.PTR
	srvs      map[string][]*dns.SRV
	txts      map[string][]string
	addresses map[string][]string
	rejected  int
}

func newMDNSRecordSet() *mdnsRecordSet {
	return &mdnsRecordSet{
		srvs:      make(map[string][]*dns.SRV),
		txts:      make(map[string][]string),
		addresses: make(map[string][]string),
	}
}

func (s *mdnsRecordSet) addMessage(message *dns.Msg) {
	if message == nil || !message.Response {
		s.rejected++
		return
	}
	records := make([]dns.RR, 0, len(message.Answer)+len(message.Ns)+len(message.Extra))
	records = append(records, message.Answer...)
	records = append(records, message.Ns...)
	records = append(records, message.Extra...)
	for _, record := range records {
		switch value := record.(type) {
		case *dns.PTR:
			s.ptrs = append(s.ptrs, value)
		case *dns.SRV:
			key := comparableDNSName(value.Hdr.Name)
			s.srvs[key] = append(s.srvs[key], value)
		case *dns.TXT:
			key := comparableDNSName(value.Hdr.Name)
			s.txts[key] = append(s.txts[key], value.Txt...)
		case *dns.A:
			key := comparableDNSName(value.Hdr.Name)
			s.addresses[key] = append(s.addresses[key], value.A.String())
		case *dns.AAAA:
			key := comparableDNSName(value.Hdr.Name)
			s.addresses[key] = append(s.addresses[key], value.AAAA.String())
		}
	}
}

func (s *mdnsRecordSet) entries() ([]mdnsQueryEntry, int) {
	entries := make([]mdnsQueryEntry, 0, len(s.ptrs))
	referencedSRV := make(map[string]struct{})
	for _, pointer := range s.ptrs {
		instanceKey := comparableDNSName(pointer.Ptr)
		services := s.srvs[instanceKey]
		if len(services) == 0 {
			entries = append(entries, mdnsQueryEntry{
				PTRName:      pointer.Hdr.Name,
				InstanceName: pointer.Ptr,
				TXT:          append([]string(nil), s.txts[instanceKey]...),
			})
			continue
		}
		referencedSRV[instanceKey] = struct{}{}
		for _, service := range services {
			hostKey := comparableDNSName(service.Target)
			entries = append(entries, mdnsQueryEntry{
				PTRName:      pointer.Hdr.Name,
				InstanceName: pointer.Ptr,
				SRVName:      service.Hdr.Name,
				HostName:     service.Target,
				Port:         int(service.Port),
				Addresses:    append([]string(nil), s.addresses[hostKey]...),
				TXT:          append([]string(nil), s.txts[instanceKey]...),
			})
		}
	}
	for instanceKey, services := range s.srvs {
		if _, ok := referencedSRV[instanceKey]; ok {
			continue
		}
		for _, service := range services {
			hostKey := comparableDNSName(service.Target)
			entries = append(entries, mdnsQueryEntry{
				InstanceName: service.Hdr.Name,
				SRVName:      service.Hdr.Name,
				HostName:     service.Target,
				Port:         int(service.Port),
				Addresses:    append([]string(nil), s.addresses[hostKey]...),
				TXT:          append([]string(nil), s.txts[instanceKey]...),
			})
		}
	}
	return dedupeMDNSEntries(entries), s.rejected
}

func dedupeMDNSEntries(entries []mdnsQueryEntry) []mdnsQueryEntry {
	seen := make(map[string]struct{}, len(entries))
	out := make([]mdnsQueryEntry, 0, len(entries))
	for _, entry := range entries {
		entry.Addresses = normalizeAddresses(entry.Addresses)
		key := strings.Join([]string{
			comparableDNSName(entry.PTRName), comparableDNSName(entry.InstanceName),
			comparableDNSName(entry.SRVName), comparableDNSName(entry.HostName),
		}, "|")
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, entry)
	}
	return out
}

func canonicalDNSName(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return ""
	}
	if !strings.HasSuffix(name, ".") {
		name += "."
	}
	return name
}

func comparableDNSName(name string) string {
	return strings.ToLower(canonicalDNSName(name))
}
