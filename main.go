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

	"github.com/stongo/fete-node/common"
	"github.com/stongo/fete-node/greeter"
	"github.com/stongo/fete-node/proto"
)

const pid protocol.ID = "/grpc/1.0.0"

func main() {
	c := &config{}
	log := common.Logger
	flag.StringVar(&c.ListenAdddress, "address", "0.0.0.0", "The bootstrap node host listen address\n")
	flag.IntVar(&c.ListenPort, "port", 4001, "Node listen port")
	flag.StringVar(&c.PeerKeyPath, "key-path", "", "Private key path")
	flag.StringVar(&c.PeerList, "peer-list", "", "Path to file containing peer multiaddrs")
	flag.StringVar(&c.Repo, "repo", "", "Sets a protocol id for stream headers")
	flag.Parse()

	repo, err := checkOrCreateRepo(c.Repo)
	if err != nil {
		log.Fatal(err)
		os.Exit(1)
	}

	privKey, err := checkOrCreatePrivateKey(c.PeerKeyPath, repo)
	if err != nil {
		log.Fatal(err)
		os.Exit(1)
	}

	sourceMultiAddr, _ := multiaddr.NewMultiaddr(fmt.Sprintf("/ip4/%s/tcp/%d", c.ListenAdddress, c.ListenPort))
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
	proto.RegisterGreeterServer(s, &greeter.Server{})

	// serve grpc server over libp2p host
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	l := p2pgrpc.NewListener(ctx, h, pid)
	errCh := make(chan error, 1)
	go func() { errCh <- s.Serve(l) }()
	log.Info("Started GRPC Server")
	pl, err := os.OpenFile(c.PeerList, os.O_RDONLY, os.ModePerm)
	if err != nil {
		log.Warn("no peers found", err)
	} else {
		// TODO: Proper peer handling
		var peerconfig []string
		pls := bufio.NewScanner(pl)
		pls.Split(bufio.ScanLines)
		for pls.Scan() {
			peerconfig = append(peerconfig, pls.Text())
		}
		connectedPeers := make(map[peer.ID]bool)
		for {
			ok, err := connectToPeers(h, ctx, peerconfig, connectedPeers)
			if err != nil {
				log.Fatal("problem connecting to peers:", err)
				os.Exit(1)
			}
			if !ok {
				log.Warn("Not all peers connected")
				time.Sleep(10 * time.Second)
				continue
			}
			log.Info("Connected to all peers")
			break
		}
	}
	select {
	case err := <-errCh:
		log.Fatal(err)
		os.Exit(1)
	}
}

func checkOrCreateRepo(repo string) (r string, err error) {
	log := common.Logger
	if repo == "" {
		hd, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("problem getting home dir: %s", err)
		}
		repo = hd + "/.fete-node"
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
				return "", err
			}
		}

	}
	return repo, nil
}

func checkOrCreatePrivateKey(peerKeyPath, repo string) (crypto.PrivKey, error) {
	log := common.Logger
	var privKey crypto.PrivKey
	var privKeyPath string
	if peerKeyPath == "" {
		privKeyPath = repo + "/private_key.pem"
		pkp, err := os.Open(privKeyPath)
		if err != nil {
			log.Info("Generating new peer keys at default location")
			privKey, err := genLibp2pKey()
			if err != nil {
				return nil, fmt.Errorf("Key generation issue: %s", err)
			}

			// Convert private key to bytes
			privKeyBytes, err := crypto.MarshalPrivateKey(privKey)
			if err != nil {
				return nil, fmt.Errorf("Error marshaling private key:i %s", err)
			}

			// Save private key to a file
			privKeyFile, err := os.Create(privKeyPath)
			if err != nil {
				return nil, fmt.Errorf("Error creating private key file: %s", err)
			}
			defer privKeyFile.Close()

			_, err = privKeyFile.Write(privKeyBytes)
			if err != nil {
				return nil, fmt.Errorf("Error writing private key to file: %s", err)
			}

			log.Info("Private key saved to", privKeyPath)

		} else {
			log.Info("Peer key exists, skipping creation")
		}
		defer pkp.Close()
	}
	if privKey == nil {
		if peerKeyPath != "" {
			privKeyPath = peerKeyPath
		}
		log.Info("loading peerkey", privKeyPath)
		var err error
		privKey, err = loadPrivateKey(privKeyPath)
		if err != nil {
			return nil, fmt.Errorf("Unmarshalling key error: %s", err)
		}
		peerKeyID, err := peer.IDFromPrivateKey(privKey)
		if err != nil {
			return nil, fmt.Errorf("Load peer id error: %s", err)
		}
		log.Info("Loaded peerkey:", peerKeyID)
	}
	return privKey, nil
}

func connectToPeers(h host.Host, ctx context.Context, pls []string, pm map[peer.ID]bool) (bool, error) {
	for _, pl := range pls {
		ma, err := multiaddr.NewMultiaddr(pl)
		if err != nil {
			return false, err
		}
		p, err := peer.AddrInfosFromP2pAddrs(ma)
		if err != nil {
			return false, err
		}
		peerid := p[0].ID
		if peerid != h.ID() {
			if err := h.Connect(ctx, p[0]); err != nil {
				return false, nil
			}
			pm[peerid] = true
		}
	}
	return true, nil
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
	ProtocolID     string
	ListenAdddress string
	ListenPort     int
	Repo           string
	PeerKeyPath    string
	PeerList       string
}
