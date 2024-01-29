package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	p2pgrpc "github.com/birros/go-libp2p-grpc"
	"google.golang.org/grpc"

	libp2p "github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p-core/protocol"
	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/multiformats/go-multiaddr"

	"github.com/stongo/fetenode/common"
)

const pid protocol.ID = "/grpc/1.0.0"

func main() {
	c := &config{}
	flag.StringVar(&c.RendezvousString, "rendezvous", "meetme", "Unique string to identify group of nodes. Share this with your friends to let them connect with you")
	flag.StringVar(&c.ListenHost, "host", "0.0.0.0", "The bootstrap node host listen address\n")
	flag.StringVar(&c.ProtocolID, "pid", "/chat/1.1.0", "Sets a protocol id for stream headers")
	flag.StringVar(&c.PeerKeyPath, "key-path", "", "private key path")
	flag.StringVar(&c.Repo, "repo", "", "Sets a protocol id for stream headers")
	flag.IntVar(&c.ListenPort, "port", 4001, "node listen port")
	flag.Parse()

	repo := c.Repo
	if repo == "" {
		hd, err := os.UserHomeDir()
		if err != nil {
			common.Logger.Fatalf("problem getting home dir: %s", err)
			os.Exit(1)
		}
		repo = hd + "/.fetenode"
		common.Logger.Info("creating repo", repo)
		if err = os.Mkdir(repo, 0700); err != nil && os.IsNotExist(err) {
			common.Logger.Warn("default repo exists; skipping directory creation")
		}
	} else {
		if _, err := os.Stat(repo); err != nil && os.IsNotExist(err) {
			common.Logger.Fatal("Repo does not exist:", err)
			os.Exit(1)
		}

	}
	// Generate libp2p keypair if it doesn't exist
	var privKey crypto.PrivKey
	if c.PeerKeyPath == "" {
		privKeyPath := repo + "/private_key.pem"
		pubKeyPath := repo + "/public_key.pem"
		if _, err := os.Open(privKeyPath); err != nil {
			common.Logger.Info("Generating new peer keys at default location")
			privKey, pubKey, err := crypto.GenerateKeyPair(
				crypto.Ed25519,
				-1,
			)
			if err != nil {
				common.Logger.Fatalf("Key generation issue: %s", err)
				os.Exit(1)
				return
			}

			// Convert private key to bytes
			privKeyBytes, err := crypto.MarshalPrivateKey(privKey)
			if err != nil {
				fmt.Println("Error marshaling private key:", err)
				return
			}

			// Convert public key to bytes
			pubKeyBytes, err := crypto.MarshalPublicKey(pubKey)
			if err != nil {
				fmt.Println("Error marshaling public key:", err)
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

			common.Logger.Info("Private key saved to", privKeyPath)

			// Save public key to a file
			pubKeyFile, err := os.Create(pubKeyPath)
			if err != nil {
				fmt.Println("Error creating public key file:", err)
				return
			}
			defer pubKeyFile.Close()

			_, err = pubKeyFile.Write(pubKeyBytes)
			if err != nil {
				fmt.Println("Error writing public key to file:", err)
				return
			}

			common.Logger.Info("Public key saved to", pubKeyPath)
		} else {
			common.Logger.Info("Peer key exists, skipping creation")
		}
	} else {
		privKeyFile, err := os.Open(c.PeerKeyPath)
		if err != nil {
			common.Logger.Fatalf("Key does not exist: %s", err)
			os.Exit(1)
		}
		privKeyBytes := make([]byte, 100)
		privKeyFile.Read(privKeyBytes)
		privKey, err = crypto.UnmarshalPrivateKey(privKeyBytes)
		if err != nil {
			common.Logger.Fatalf("Unmarshalling key error: %s", err)
			os.Exit(1)
		}
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
		libp2p.ListenAddrs(sourceMultiAddr),
		libp2p.Identity(privKey),
	)
	if err != nil {
		common.Logger.Fatal("Error create new libp2p host:", err)
		os.Exit(1)
	}
	common.Logger.Info("Successfully created node")

	// grpc server
	s := grpc.NewServer(p2pgrpc.WithP2PCredentials())
	//proto.RegisterGreeterServer(s, &greeter.Server{})

	// serve grpc server over libp2p host
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	l := p2pgrpc.NewListener(ctx, h, pid)
	go s.Serve(l)
	common.Logger.Info("Started GRPC Server")
	select {}
}

type config struct {
	RendezvousString string
	ProtocolID       string
	ListenHost       string
	ListenPort       int
	Repo             string
	PeerKeyPath      string
}
