package main

import (
	"bufio"
	"context"
	"crypto/rand"
	"flag"
	"fmt"
	"net/http"
	"os"
	"time"

	libp2p "github.com/libp2p/go-libp2p"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/multiformats/go-multiaddr"

	"github.com/stongo/fete-node/common"
	"github.com/stongo/fete-node/partypubsub"
	"github.com/stongo/fete-node/rpc"
	"github.com/stongo/fete-node/signer"
)

func main() {
	c := &config{}
	log := common.Logger
	flag.StringVar(&c.ListenAdddress, "address", "0.0.0.0", "Node host listen address")
	flag.IntVar(&c.ListenPort, "port", 4001, "Node listen port")
	flag.IntVar(&c.HTTPListenPort, "http-port", 5000, "Node listen port")
	flag.StringVar(&c.PeerKeyPath, "key-path", "", "Private key path")
	flag.StringVar(&c.PeerList, "peer-list", "", "Path to file containing peer multiaddrs")
	flag.StringVar(&c.Repo, "repo", "", "Repository for application storage")
	flag.StringVar(&c.PubSubTopic, "topic", "fete", "PubSub Topic for signing parties")
	flag.Parse()

	// Repo management
	repo, err := checkOrCreateRepo(c.Repo)
	if err != nil {
		log.Fatal(err)
		os.Exit(1)
	}

	// Libp2p key management
	privKey, err := checkOrCreatePrivateKey(c.PeerKeyPath, repo)
	if err != nil {
		log.Fatal(err)
		os.Exit(1)
	}

	// Create a libp2p node
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
	defer h.Close()
	peerInfo := peer.AddrInfo{
		ID:    h.ID(),
		Addrs: h.Addrs(),
	}
	addrs, err := peer.AddrInfoToP2pAddrs(&peerInfo)
	if err != nil {
		log.Fatal("Error creating new libp2p host:", err)
		os.Exit(1)
	}
	log.Info("PeerID:", addrs[0])
	log.Info("Successfully created node")

	// Create a pubsub service for exchanging messages with other signers
	ctx, cancel := context.WithCancel(context.Background())
	ps, err := pubsub.NewGossipSub(ctx, h)
	if err != nil {
		log.Fatal("Error creating pubsub service:", err)
		os.Exit(1)
	}
	// join the TSS signing party
	// @TODO: should there be a topic for keygen, message signing and key regeneration?
	t := c.PubSubTopic
	pps, err := partypubsub.JoinSignersPartyPS(ctx, ps, h.ID(), t)
	if err != nil {
		log.Fatal("Error joining pubsub topic %s: %s", t, err)
		os.Exit(1)
	}
	log.Infof("Started PubSub TSS Signing topic: %s", t)

	// Jsonrpc 2.0 server
	// use this for message signing requests
	s, err := rpc.NewServer()
	if err != nil {
		log.Errorf("Problem creating JSONRPC server: %s", err)
	}
	http.HandleFunc("/rpc", s.ServeHTTP)
	errCh := make(chan error, 1)

	go func() { errCh <- http.ListenAndServe(fmt.Sprintf(":%d", c.HTTPListenPort), nil) }()
	log.Info("Started HTTP JSONRPC 2.0 Server")

	// Connect to known peers only
	// p2p network is for tss signing party on a private subnet
	defer cancel()
	pl, err := os.OpenFile(c.PeerList, os.O_RDONLY, os.ModePerm)
	if err != nil {
		log.Warn("no peers found:", err)
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
	defer pl.Close()
	p, err := signer.NewSigner(&signer.SignerOpts{Libp2pPrivKey: privKey})
	if err != nil {
		log.Fatalf("Error creating a new TSS signer: %s", err)
		os.Exit(1)
	}
	log.Infof("Success creating new signer: %s", p.PartyID)
	select {
	case err := <-errCh:
		log.Fatal(err)
		os.Exit(1)
	// @TODO handle TSS messages
	case <-pps.Messages:
		log.Infof("received message from %s pubsub topic", t)
	}
}

/* If a user supplies repo info, check if it's there already, or else create it
 * Otherwise create a repo at the default location if it doesn't exist
 */
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

/* Check if a key already exists, or else create one */
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

/* Connect to know peers, supplied in the "pls" argument */
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

/* Generate a private key*/
func genLibp2pKey() (crypto.PrivKey, error) {
	pk, _, err := crypto.GenerateEd25519Key(rand.Reader)
	if err != nil {
		return nil, err
	}
	return pk, nil
}

/* Load a private key from file */
func loadPrivateKey(path string) (crypto.PrivKey, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return crypto.UnmarshalPrivateKey(data)
}

/* Application config */
type config struct {
	ProtocolID     string
	ListenAdddress string
	ListenPort     int
	HTTPListenPort int
	Repo           string
	PeerKeyPath    string
	PeerList       string
	PubSubTopic    string
}
