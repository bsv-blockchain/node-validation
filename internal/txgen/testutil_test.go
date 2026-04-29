// internal/txgen/testutil_test.go
package txgen

import (
	"encoding/json"

	"github.com/bsv-blockchain/node-validation/internal/txgen/testdata"
)

func jsonRoundTrip(in, out any) error {
	b, err := json.Marshal(in)
	if err != nil {
		return err
	}
	return json.Unmarshal(b, out)
}

// utxoFromFixture converts a testdata.UTXO to a txgen.UTXO.
func utxoFromFixture(f testdata.UTXO) UTXO {
	return UTXO{
		TxID:     f.TxID,
		Vout:     f.Vout,
		Satoshis: f.Satoshis,
		Script:   f.Script,
	}
}
