// Package observer provides shared block-tip polling and reorg-detection
// helpers used by PC-1 (parallel node comparison) and INTER-1 (mixed-network
// consensus). It abstracts over teranode and svnode RPC clients via a
// minimal TipReader interface.
package observer
