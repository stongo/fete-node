package main

import (
	"bufio"
	"context"
	"crypto/rand"
	"flag"
	"fmt"
	"os"
	"time"

	p2pgrpc "github.com/birros/go-libp2p-grpc"
	"google.golang.org/grpc"

	libp2p "github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/protocol"
	"github.com/multiformats/go-multiaddr"

	"github.com/stongo/fetenode/common"
)

const pid protocol.ID = "/grpc/1.0.0"

func main() {
	c := &config{}
	log := common.Logger
	flag.StringVar(&c.ListenHost, "host", "0.0.0.0", "The bootstrap node host listen address\n")
	flag.IntVar(&c.ListenPort, "port", 4001, "Node listen port")
	flag.StringVar(&c.PeerKeyPath, "key-path", "", "Private key path")
	flag.StringVar(&c.PeerList, "peer-list", "", "Path to file containing peer multiaddrs")
	flag.StringVar(&c.Repo, "repo", "", "Sets a protocol id for stream headers")
	flag.Parse()

	repo := c.Repo
	if repo == "" {
		hd, err := os.UserHomeDir()
		if err != nil {
			log.Fatalf("problem getting home dir: %s", err)
			os.Exit(1)
		}
		repo = hd + "/.fetenode"
		log.Info("creating repo", repo)
		// TODO: better error handling
		if err = os.Mkdir(repo, 0700); err != nil && os.IsNotExist(err) {
			log.Warn("default repo exists; skipping directory creation")
		}
	} else {
		log.Infof("Creating repo %s if it doesn't exist", repo)
		if _, err := os.Stat(repo); err != nil && os.IsNotExist(err) {
			log.Info("creating repo", repo)
			if err = os.Mkdir(repo, 0700); err != nil {
				log.Fatal(err)
			}
		}

	}
	// Generate libp2p keypair if it doesn't exist
	var privKey crypto.PrivKey
	privKeyPath := repo + "/private_key.pem"
	if c.PeerKeyPath == "" {
		pkp, err := os.Open(privKeyPath)
		if err != nil {
			log.Info("Generating new peer keys at default location")
			privKey, err := genLibp2pKey()
			if err != nil {
				log.Fatalf("Key generation issue: %s", err)
				os.Exit(1)
				return
			}

			// Convert private key to bytes
			privKeyBytes, err := crypto.MarshalPrivateKey(privKey)
			if err != nil {
				fmt.Println("Error marshaling private key:", err)
				return
			}

			// Save private key to a file
			privKeyFile, err := os.Create(privKeyPath)
			if err != nil {
				fmt.Println("Error creating private key file:", err)
				return
			}
			defer privKeyFile.Close()

			_, err = privKeyFile.Write(privKeyBytes)
			if err != nil {
				fmt.Println("Error writing private key to file:", err)
				return
			}

			log.Info("Private key saved to", privKeyPath)

		} else {
			log.Info("Peer key exists, skipping creation")
		}
		defer pkp.Close()
	}
	if privKey == nil {
		if c.PeerKeyPath != "" {
			privKeyPath = c.PeerKeyPath
		}
		log.Info("loading peerkey", privKeyPath)
		var err error
		privKey, err = loadPrivateKey(privKeyPath)
		if err != nil {
			log.Fatalf("Unmarshalling key error: %s", err)
			os.Exit(1)
		}
		peerKeyID, err := peer.IDFromPrivateKey(privKey)
		if err != nil {
			log.Fatalf("Load peer id error: %s", err)
			os.Exit(1)
		}
		log.Info("Loaded peerkey:", peerKeyID)
	}
	/*
		// Get Peer ID
		peerID, err := peer.IDFromPublicKey(pubKey)
		if err != nil {
			return nil, err
		}
	*/
	sourceMultiAddr, _ := multiaddr.NewMultiaddr(fmt.Sprintf("/ip4/%s/tcp/%d", c.ListenHost, c.ListenPort))
	h, err := libp2p.New(
		libp2p.Identity(privKey),
		libp2p.Ping(true),
		libp2p.ListenAddrs(sourceMultiAddr),
	)
	if err != nil {
		log.Fatal("Error create new libp2p host:", err)
		os.Exit(1)
	}
	peerInfo := peer.AddrInfo{
		ID:    h.ID(),
		Addrs: h.Addrs(),
	}
	addrs, err := peer.AddrInfoToP2pAddrs(&peerInfo)
	if err != nil {
		log.Fatal("Error create new libp2p host:", err)
		os.Exit(1)
	}
	log.Info("PeerID:", addrs[0])
	log.Info("Successfully created node")

	// grpc server
	s := grpc.NewServer(p2pgrpc.WithP2PCredentials())
	//proto.RegisterGreeterServer(s, &greeter.Server{})

	// serve grpc server over libp2p host
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	l := p2pgrpc.NewListener(ctx, h, pid)
	go s.Serve(l)
	log.Info("Started GRPC Server")
	pl, err := os.OpenFile(c.PeerList, os.O_RDONLY, os.ModePerm)
	if err != nil {
		log.Warn("no peers found", err)
	} else {
		pls := bufio.NewScanner(pl)
		if err = connectToPeers(h, ctx, pls); err != nil {
			log.Fatal(err)
			os.Exit(1)
		}
		log.Info("Connected to all peers")
	}
	select {}
}

func connectToPeers(h host.Host, ctx context.Context, pls *bufio.Scanner) error {
	time.Sleep(20 * time.Second)
	pls.Split(bufio.ScanLines)
	for pls.Scan() {
		ma, err := multiaddr.NewMultiaddr(pls.Text())
		if err != nil {
			return nil
		}
		p, err := peer.AddrInfosFromP2pAddrs(ma)
		if err != nil {
			return nil
		}
		if p[0].ID != h.ID() {
			if err := h.Connect(ctx, p[0]); err != nil {
				return err
			}
		}
	}
	return nil
}

func genLibp2pKey() (crypto.PrivKey, error) {
	pk, _, err := crypto.GenerateEd25519Key(rand.Reader)
	if err != nil {
		return nil, err
	}
	return pk, nil
}

func loadPrivateKey(path string) (crypto.PrivKey, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return crypto.UnmarshalPrivateKey(data)
}

type config struct {
	RendezvousString string
	ProtocolID       string
	ListenHost       string
	ListenPort       int
	Repo             string
	PeerKeyPath      string
	PeerList         string
}
