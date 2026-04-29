// Command derive-address reads a WIF from stdin or argv and prints the
// corresponding regtest P2PKH address. Used by compose/bootstrap.sh to
// fund the test wallet without embedding key-derivation logic in shell.
//
//	echo cVjzvdHG... | ./bin/derive-address
//	./bin/derive-address cVjzvdHG...
package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/libsv/go-bk/wif"
	"github.com/libsv/go-bt/v2/bscript"
)

func main() {
	wifStr := strings.TrimSpace(readInput())
	if wifStr == "" {
		fmt.Fprintln(os.Stderr, "usage: derive-address <wif>  (or pipe WIF to stdin)")
		os.Exit(2)
	}
	w, err := wif.DecodeWIF(wifStr)
	if err != nil {
		fmt.Fprintf(os.Stderr, "decode WIF: %v\n", err)
		os.Exit(1)
	}
	// Detect testnet WIFs by prefix (regtest uses testnet network bytes).
	mainnet := true
	if c := wifStr[0]; c == 'c' || c == '9' {
		mainnet = false
	}
	addr, err := bscript.NewAddressFromPublicKey(w.PrivKey.PubKey(), mainnet)
	if err != nil {
		fmt.Fprintf(os.Stderr, "derive: %v\n", err)
		os.Exit(1)
	}
	fmt.Println(addr.AddressString)
}

func readInput() string {
	if len(os.Args) >= 2 {
		return os.Args[1]
	}
	r := bufio.NewReader(os.Stdin)
	line, _ := r.ReadString('\n')
	return line
}
