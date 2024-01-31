package signer

import (
	"math/big"

	"bitbucket.org/pcastools/hash"
	"github.com/bnb-chain/tss-lib/tss"
	petname "github.com/dustinkirkland/golang-petname"
	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/peer"
)

/* A TSS Signer */
type Signer struct {
	PartyID *tss.PartyID
	ID      string
	PeerID  string
}

/* Options for Signer
 * PrivateKey is assiged here, but we should probably make this more secure
 */
type SignerOpts struct {
	Libp2pPrivKey crypto.PrivKey
}

/* Create a new Signer
 * ready to join a Party
 */
func NewSigner(o *SignerOpts) (*Signer, error) {
	// Generate random id
	id := petname.Generate(2, "-")
	// Get Peer ID
	peerID, err := peer.IDFromPrivateKey(o.Libp2pPrivKey)
	if err != nil {
		return nil, err
	}
	// Generate PartyID
	h := big.NewInt(int64(hash.String(string(peerID))))
	partyID := tss.NewPartyID(id, "", h)
	return &Signer{
		PartyID: partyID,
		ID:      id,
		PeerID:  string(peerID),
	}, nil

}
