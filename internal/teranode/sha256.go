// internal/teranode/sha256.go
package teranode

import "crypto/sha256"

func shaSum(b []byte) [32]byte { return sha256.Sum256(b) }
