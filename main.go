package main

import (
	"context"
	"fmt"
	"time"

	"github.com/libp2p/go-libp2p"
	connmgr "github.com/libp2p/go-libp2p-connmgr"
	"github.com/libp2p/go-libp2p-core/crypto"
	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/peer"
	routing "github.com/libp2p/go-libp2p-core/routing"
	dht "github.com/libp2p/go-libp2p-kad-dht"
	libp2pquic "github.com/libp2p/go-libp2p-quic-transport"
	secio "github.com/libp2p/go-libp2p-secio"
	libp2ptls "github.com/libp2p/go-libp2p-tls"
)

func main() {
	// the context governs the lifetime of the libp2p node
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// to construct a simple host wiht all the default setings, just use `New`
	h, err := libp2p.New(ctx)
	if err != nil {
		panic(err)
	}

	fmt.Printf("Hello World, my host ID is %s\n", h.ID())

	// set your own keypair
	priv, _, err := crypto.GenerateKeyPair(
		crypto.Ed25519, // Select key type
		-1,             // Select key length
	)
	if err != nil {
		panic(err)
	}

	var idht *dht.IpfsDHT

	h2, err := libp2p.New(ctx,
		// use generated keypair
		libp2p.Identity(priv),
		// multiple listen addresses
		libp2p.ListenAddrStrings(
			"/ip4/0.0.0.0/tcp/9000",      // regular TCP connection
			"/ip4/0.0.0.0/udp/9000/quic", // a UDP endpoint for the QUIC transport
		),
		// support TLS connections
		libp2p.Security(libp2ptls.ID, libp2ptls.New),
		// secio connections
		libp2p.Security(secio.ID, secio.New),
		// support QUIC
		libp2p.Transport(libp2pquic.NewTransport),
		// support any other default transports (TCP)
		libp2p.DefaultTransports,
		// Let's prevent our peer from having too many
		// connections by attaching a connection manager
		libp2p.ConnectionManager(connmgr.NewConnManager(
			100,         // LowWater
			400,         // HighWater
			time.Minute, // GracePeriod
		)),
		// attempt ot open ports using uPNP for NATed hosts
		libp2p.NATPortMap(),
		// let this host use the DHT to find other hosts
		libp2p.Routing(func(h host.Host) (routing.PeerRouting, error) {
			idht, err = dht.New(ctx, h)
			return idht, err
		}),
		// Let this host use relay the advertise itself on relays if
		// it finds it is behind NAT. Use libp2p.Relay(options...) to
		// enable active relays and more.
		libp2p.EnableAutoRelay(),
	)

	if err != nil {
		panic(err)
	}

	fmt.Printf("Hello World, my configured host ID is %s\n", h2.ID())

	// last step to fully set up is to connect to bootstrap peers
	for _, addr := range dht.DefaultBootstrapPeers {
		pi, _ := peer.AddrInfoFromP2pAddr(addr)
		h2.Connect(ctx, *pi)
		fmt.Println(*pi)

	}
}
