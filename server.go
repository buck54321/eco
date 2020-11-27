package eco

import (
	"bytes"
	"context"
	"crypto/elliptic"
	"crypto/tls"
	"crypto/x509"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"os"
	"strings"
	"time"

	"github.com/buck54321/eco/encode"
	"github.com/decred/dcrd/certgen"
)

const (
	// rpcTimeoutSeconds is the number of seconds a connection to the
	// RPC server is allowed to stay open without authenticating before it
	// is closed.
	rpcTimeoutSeconds = 10

	routeServiceStatus   = "service_status"
	routeInit            = "init"
	routeSync            = "sync"
	routeStartDecrediton = "start_decrediton"
)

type Server struct {
	listener net.Listener
	eco      *Eco
	ctx      context.Context
}

// NewServer is a constructor for an Server.
func NewServer(netAddr *NetAddr, eco *Eco) (*Server, error) {
	// Find or create the key pair.
	keyExists := fileExists(KeyPath)
	certExists := fileExists(CertPath)
	if certExists == !keyExists {
		return nil, fmt.Errorf("missing cert pair file")
	}
	if !keyExists && !certExists {
		err := genCertPair(CertPath, KeyPath)
		if err != nil {
			return nil, err
		}
	}
	keypair, err := tls.LoadX509KeyPair(CertPath, KeyPath)
	if err != nil {
		return nil, err
	}

	// Prepare the TLS configuration.
	tlsConfig := tls.Config{
		Certificates: []tls.Certificate{keypair},
		MinVersion:   tls.VersionTLS12,
	}

	// TODO: Fire up a UDP server too so other machines on the network can find
	// this Eco.

	if netAddr.Net == "unix" {
		if err := os.RemoveAll(netAddr.Addr); err != nil {
			return nil, fmt.Errorf("error removing old unix socket at %s: %v", netAddr.Addr, err)
		}
	}

	listener, err := tls.Listen(netAddr.Net, netAddr.Addr, &tlsConfig)
	if err != nil {
		return nil, fmt.Errorf("Can't listen on %s %s: %w", netAddr.Net, netAddr.Addr, err)
	}

	return &Server{
		listener: listener,
		eco:      eco,
	}, nil
}

// Run starts the server. Run should be called only after all routes are
// registered.
func (s *Server) Run(ctx context.Context) {
	log.Trace("Starting Eco server")

	go func() {
		<-ctx.Done()
		s.listener.Close()
	}()

	s.ctx = ctx
	// Start serving.
	log.Infof("Eco server running")
	for {
		conn, err := s.listener.Accept()
		if err != nil {
			var opErr *net.OpError
			if errors.As(err, &opErr) && strings.Contains(opErr.Error(), "use of closed network connection") {
				// Probably a normal shutdown
				return
			}
			log.Errorf("Accept error: %v", err)
			return
		}
		if ctx.Err() != nil {
			return
		}
		go s.handleRequest(conn)
	}
}

func readN(conn net.Conn, n int) ([]byte, error) {
	buf := make([]byte, n)
	m, err := io.ReadAtLeast(conn, buf, n)
	if err == nil && m != n {
		return nil, fmt.Errorf("ReadAtLeast read the wrong number of bytes. wanted %d, got %d", n, m)
	}
	return buf, err
}

func (s *Server) handleRequest(conn net.Conn) {
	packet, err := nextPacket(conn)
	if err != nil {
		log.Error(err)
		return
	}

	route, payload := popRoute(packet)
	if route == "" {
		log.Errorf("could not decode route from request from %s", conn.RemoteAddr())
		return
	}

	fmt.Println("--route", route)

	defer conn.Close()
	switch route {
	case routeServiceStatus:
		s.handleServiceRequest(conn, payload)
	case routeInit:
		s.handleInitRequest(conn, payload)
	case routeSync:
		s.handleSyncRequest(conn)
	case routeStartDecrediton:
		s.handleStartDecrediton(conn)
	default:
		log.Errorf("unknown route: %s", route)
	}
}

func nextPacket(conn net.Conn) ([]byte, error) {
	packetLenB, err := readN(conn, 4)
	if err != nil {
		return nil, fmt.Errorf("readN error (packet length): %v", err)
	}

	packetLen := binary.BigEndian.Uint32(packetLenB)
	if packetLen == 0 {
		return nil, fmt.Errorf("0 length packet")
	}
	packet, err := readN(conn, int(packetLen))
	if err != nil {
		return nil, fmt.Errorf("readN error (packet): %v", err)
	}
	return packet, nil
}

type stateRequest struct {
	Service string
}

type stateResponse struct {
	State []byte
}

func (s *Server) handleServiceRequest(conn net.Conn, payload []byte) {
	req := new(stateRequest)
	err := encode.GobDecode(payload, req)
	if err != nil {
		log.Errorf("decodeRequest error: %v", err)
		return
	}

	var stateI interface{}
	switch req.Service {
	case dcrd:
		stateI = s.eco.dcrdState()
	case "eco":
		stateI = s.eco.metaState()
	default:
		log.Errorf("status request received for unkown service: %s", req.Service)
	}

	var stateB []byte
	if stateI != nil {
		stateB, err = encode.GobEncode(stateI)
		if err != nil {
			log.Errorf("gobEncode(stateI) error: %v", err)
			return
		}
	}
	resp, err := encode.GobEncode(&stateResponse{
		State: stateB,
	})
	if err != nil {
		log.Errorf("gobEncode(resp) error: %v", err)
		return
	}
	err = writeConn(conn, resp)
	if err != nil {
		log.Errorf("Write error: %v", err)
	}
}

type initRequest struct {
	SyncMode SyncMode
	PW       []byte
}

func sendProgress(conn net.Conn, svc, status, errStr string, progress float32) error {
	return sendPacket(conn, &Progress{
		Service:  svc,
		Status:   status,
		Err:      errStr,
		Progress: progress,
	})
}

func sendPacket(conn net.Conn, contents interface{}) error {
	packet := encodePacket(contents)
	if packet == nil {
		return fmt.Errorf("failed to encode ProgressUpdate")
	}

	err := writeConn(conn, packet)
	if err != nil {
		return fmt.Errorf("Write error: %v", err)
	}
	return nil
}

func (s *Server) handleInitRequest(conn net.Conn, payload []byte) {
	if s.eco.syncMode() != SyncModeUninitialized {
		sendProgress(conn, "eco", "", "Already initialized", 0)
		return
	}

	req := new(initRequest)
	err := encode.GobDecode(payload, req)
	if err != nil {
		log.Errorf("decodeRequest error: %v", err)
		return
	}

	switch req.SyncMode {
	case SyncModeFull, SyncModeSPV:
		s.eco.initEco(conn, req)
	default:
		log.Errorf("Unknown sync mode requested: %d", req.SyncMode)
		sendProgress(conn, "eco", "", "Unknown sync mode requested", 0)
	}
}

func (s *Server) handleSyncRequest(conn net.Conn) {
	// ch := make(chan *ecotypes.ProgressUpdate, 1)
	ch := s.eco.syncChan()
	defer s.eco.returnSyncChan(ch)
	for {
		select {
		case u := <-ch:
			err := sendPacket(conn, u)
			if err != nil {
				log.Errorf("error sending progress update: %v", err)
				return
			}
		case <-s.ctx.Done():
			return
		}
	}
}

func (s *Server) handleStartDecrediton(conn net.Conn) {
	err := s.eco.runDecrediton()
	resp := &EcoError{}
	if err != nil {
		resp.Msg = err.Error()
	}
	b, err := encode.GobEncode(resp)
	if err != nil {
		log.Errorf("GobEncode(resp) error: %v", err)
		return
	}
	writeConn(conn, b)

}

func writeConn(conn net.Conn, b []byte) error {
	_, err := io.Copy(conn, bytes.NewReader(b))
	return err
}

type Client struct {
	netAddr   *NetAddr
	tlsConfig *tls.Config
}

func NewClient() (*Client, error) {
	netAddr, err := retrieveNetAddr()
	if err != nil {
		return nil, fmt.Errorf("Error retreiving eco server address: %w", err)
	}

	pem, err := ioutil.ReadFile(CertPath)
	if err != nil {
		return nil, fmt.Errorf("ReadFile error: %v", err)
	}

	pool := x509.NewCertPool()
	if ok := pool.AppendCertsFromPEM(pem); !ok {
		return nil, fmt.Errorf("invalid certificate file: %v",
			CertPath)
	}
	tlsConfig := &tls.Config{
		RootCAs:    pool,
		ServerName: "localhost",
	}
	return &Client{
		netAddr:   netAddr,
		tlsConfig: tlsConfig,
	}, nil
}

func (c *Client) request(ctx context.Context, route string, thing, resp interface{}) error {
	req := encodeRequest(route, thing)
	if req == nil {
		return fmt.Errorf("Could not encode request")
	}

	done := make(chan struct{})

	var err error
	go func() {
		defer close(done)

		var conn net.Conn
		conn, err = (&tls.Dialer{Config: c.tlsConfig}).DialContext(ctx, c.netAddr.Net, c.netAddr.Addr)
		if err != nil {
			err = fmt.Errorf("Dial error: %w", err)
			return
		}
		defer conn.Close()
		// write
		_, err = conn.Write(req)
		if err != nil {
			err = fmt.Errorf("Write error: %w", err)
			return
		}
		// read
		var buf bytes.Buffer
		_, err = io.Copy(&buf, conn)
		if err != nil {
			err = fmt.Errorf("Copy error: %v", err)
			return
		}
		if resp != nil {
			err = encode.GobDecode(buf.Bytes(), resp)
		}
	}()

	select {
	case <-done:
	case <-ctx.Done():
		return fmt.Errorf("Context canceled")
	}

	return err
}

func (c *Client) subscribe(ctx context.Context, route string, subscription interface{}) (<-chan []byte, error) {
	req := encodeRequest(route, subscription)
	if req == nil {
		return nil, fmt.Errorf("Could not encode subscription request")
	}

	conn, err := (&tls.Dialer{Config: c.tlsConfig}).DialContext(ctx, c.netAddr.Net, c.netAddr.Addr)
	if err != nil {
		return nil, fmt.Errorf("Dial error: %w", err)
	}
	// write
	_, err = conn.Write(req)
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("Write error: %w", err)
	}

	ch := make(chan []byte, 1)
	go func() {
		defer conn.Close()
		for {
			if ctx.Err() != nil {
				return
			}
			packet, err := nextPacket(conn)
			if err != nil {
				log.Errorf("Subscription feed error: %v", err)
				return
			}
			select {
			case ch <- packet:
			default:
				return
			}
		}
	}()

	return ch, nil
}

func serviceStatus(ctx context.Context, svc string, state interface{}) error {
	resp := new(stateResponse)
	err := request(ctx, routeServiceStatus, &stateRequest{
		Service: svc,
	}, resp)
	if err != nil {
		return fmt.Errorf("Error encoding request: %w", err)
	}
	if len(resp.State) == 0 {
		return nil
	}
	return encode.GobDecode(resp.State, state)
}

func request(ctx context.Context, route string, thing, resp interface{}) error {
	cl, err := NewClient()
	if err != nil {
		return err
	}
	return cl.request(ctx, route, thing, resp)
}

// filesExists reports whether the named file or directory exists.
func fileExists(name string) bool {
	_, err := os.Stat(name)
	return !os.IsNotExist(err)
}

// genCertPair generates a key/cert pair to the paths provided.
func genCertPair(certFile, keyFile string) error {
	log.Infof("Generating TLS certificates...")

	org := "dcrdex autogenerated cert"
	validUntil := time.Now().Add(10 * 365 * 24 * time.Hour)
	cert, key, err := certgen.NewTLSCertPair(elliptic.P521(), org,
		validUntil, nil)
	if err != nil {
		return err
	}

	// Write cert and key files.
	if err = ioutil.WriteFile(certFile, cert, 0644); err != nil {
		return err
	}
	if err = ioutil.WriteFile(keyFile, key, 0600); err != nil {
		os.Remove(certFile)
		return err
	}

	log.Infof("Done generating TLS certificates")
	return nil
}

func encodeRequest(route string, thing interface{}) []byte {
	routeB := []byte(route)
	routeLen := len(routeB)

	b, err := encode.GobEncode(thing)
	if err != nil {
		log.Errorf("Gob encoding error: %v", err)
		return nil
	}

	bLen := len(b)
	packetLen := 1 + routeLen + bLen
	req := make([]byte, 4+packetLen)

	lenB := make([]byte, 4)
	binary.BigEndian.PutUint32(lenB, uint32(packetLen))

	copy(req, lenB)
	copy(req[4:], []byte{byte(routeLen)})
	copy(req[4+1:], routeB)
	copy(req[4+1+routeLen:], b)
	return req
}

func encodePacket(thing interface{}) []byte {
	b, err := encode.GobEncode(thing)
	if err != nil {
		log.Errorf("Gob encoding error: %v", err)
		return nil
	}
	bLen := len(b)
	packet := make([]byte, bLen+4)
	lenB := make([]byte, 4)
	binary.BigEndian.PutUint32(lenB, uint32(bLen))
	copy(packet, lenB)
	copy(packet[4:], b)
	return packet
}

func popRoute(req []byte) (route string, payload []byte) {
	if len(req) < 2 {
		return "", nil
	}
	routeLen := int(req[:1][0])
	if len(req) < routeLen+1 {
		return "", nil
	}
	route = string(req[1 : 1+routeLen])
	return route, req[1+routeLen:]
}
