package txgen

import (
	"errors"
	"sync"

	"github.com/libsv/go-bk/bec"
)

// UTXO represents one unspent output controlled by the funder.
type UTXO struct {
	TxID        [32]byte
	Vout        uint32
	Satoshis    uint64
	Script      []byte
	BlockHeight int64 // 0 = unconfirmed
}

// Output is one output spec for a BuildRequest.
type Output struct {
	Script      []byte
	Satoshis    uint64
	Description string
}

// BuildRequest specifies what to build.
type BuildRequest struct {
	Outputs   []Output
	FeeRate   uint64 // sat/kB
	SpendUTXO *UTXO  // optional explicit input
}

// BuildResult is the outcome of a Build* call.
type BuildResult struct {
	TxID   [32]byte
	HexTx  string
	Inputs []UTXO
	Change *UTXO // nil if dust-absorbed
}

// ErrInsufficientFunds is returned when the funder does not have enough
// satoshis to satisfy a BuildRequest at the given fee rate.
var ErrInsufficientFunds = errors.New("txgen: insufficient funds")

// ErrNoWallet is returned when SV Node lacks wallet support and
// Bootstrap cannot proceed. Tests should skip cleanly when they see
// this error.
var ErrNoWallet = errors.New("txgen: SV Node wallet not available")

// dustThresholdSats is the BSV dust threshold; outputs below this are
// folded into the fee rather than emitted.
const dustThresholdSats uint64 = 546

// keyAndUTXOs is what Funder protects with its mutex.
type keyAndUTXOs struct {
	key   *bec.PrivateKey
	utxos []UTXO
	mu    sync.Mutex
}
