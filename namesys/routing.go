package namesys

import (
	"fmt"

	"github.com/ipfs/go-ipfs/Godeps/_workspace/src/code.google.com/p/goprotobuf/proto"
	mh "github.com/ipfs/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-multihash"
	"github.com/ipfs/go-ipfs/Godeps/_workspace/src/golang.org/x/net/context"
	pb "github.com/ipfs/go-ipfs/namesys/internal/pb"
	path "github.com/ipfs/go-ipfs/path"
	routing "github.com/ipfs/go-ipfs/routing"
	u "github.com/ipfs/go-ipfs/util"
)

var log = u.Logger("namesys")

// routingResolver implements NSResolver for the main IPFS SFS-like naming
type routingResolver struct {
	routing routing.IpfsRouting
}

// NewRoutingResolver constructs a name resolver using the IPFS Routing system
// to implement SFS-like naming on top.
func NewRoutingResolver(route routing.IpfsRouting) Resolver {
	if route == nil {
		panic("attempt to create resolver with nil routing system")
	}

	return &routingResolver{routing: route}
}

// CanResolve implements Resolver. Checks whether name is a b58 encoded string.
func (r *routingResolver) CanResolve(name string) bool {
	_, err := mh.FromB58String(name)
	return err == nil
}

// Resolve implements Resolver. Uses the IPFS routing system to resolve SFS-like
// names.
func (r *routingResolver) Resolve(ctx context.Context, name string) (path.Path, error) {
	log.Debugf("RoutingResolve: '%s'", name)
	hash, err := mh.FromB58String(name)
	if err != nil {
		log.Warning("RoutingResolve: bad input hash: [%s]\n", name)
		return "", err
	}
	// name should be a multihash. if it isn't, error out here.

	// use the routing system to get the name.
	// /ipns/<name>
	h := []byte("/ipns/" + string(hash))

	ipnsKey := u.Key(h)
	val, err := r.routing.GetValue(ctx, ipnsKey)
	if err != nil {
		log.Warning("RoutingResolve get failed.")
		return "", err
	}

	entry := new(pb.IpnsEntry)
	err = proto.Unmarshal(val, entry)
	if err != nil {
		return "", err
	}

	// name should be a public key retrievable from ipfs
	pubkey, err := routing.GetPublicKey(r.routing, ctx, hash)
	if err != nil {
		return "", err
	}

	hsh, _ := pubkey.Hash()
	log.Debugf("pk hash = %s", u.Key(hsh))

	// check sig with pk
	if ok, err := pubkey.Verify(ipnsEntryDataForSig(entry), entry.GetSignature()); err != nil || !ok {
		return "", fmt.Errorf("Invalid value. Not signed by PrivateKey corresponding to %v", pubkey)
	}

	// ok sig checks out. this is a valid name.

	// check for old style record:
	valh, err := mh.Cast(entry.GetValue())
	if err != nil {
		// Not a multihash, probably a new record
		return path.ParsePath(string(entry.GetValue()))
	} else {
		// Its an old style multihash record
		log.Warning("Detected old style multihash record")
		return path.FromKey(u.Key(valh)), nil
	}
}
