package veil

import (
	"blockbook/bchain"
	"blockbook/bchain/coins/btc"
	"encoding/json"

	"github.com/golang/glog"
)

// VeilRPC is an interface to JSON-RPC bitcoind service.
type VeilRPC struct {
	*btc.BitcoinRPC
    BitcoinGetBlockInfo func(hash string) (*bchain.BlockInfo, error)
}

// NewVeilRPC returns new VeilRPC instance.
func NewVeilRPC(config json.RawMessage, pushHandler func(bchain.NotificationType)) (bchain.BlockChain, error) {
	b, err := btc.NewBitcoinRPC(config, pushHandler)
	if err != nil {
		return nil, err
	}

	s := &VeilRPC{
		b.(*btc.BitcoinRPC),
        b.GetBlockInfo,
	}
	s.RPCMarshaler = btc.JSONMarshalerV1{}
	s.ChainConfig.SupportsEstimateFee = true
	s.ChainConfig.SupportsEstimateSmartFee = false

	return s, nil
}

// Initialize initializes VeilRPC instance.
func (b *VeilRPC) Initialize() error {
	chainName, err := b.GetChainInfoAndInitializeMempool(b)
	if err != nil {
		return err
	}

	glog.Info("Chain name ", chainName)
	params := GetChainParams(chainName)

	// always create parser
	b.Parser = NewVeilParser(params, b.ChainConfig)

	// parameters for getInfo request
	if params.Net == MainnetMagic {
		b.Testnet = false
		b.Network = "livenet"
	} else {
		b.Testnet = true
		b.Network = "testnet"
	}

	glog.Info("rpc: block chain ", params.Name)

	return nil
}

// getBlockInfo extends GetBlockInfo for VeilRPC
func (g *VeilRPC) GetBlockInfo(hash string) (*bchain.BlockInfo, error) {
    bi, err := g.BitcoinGetBlockInfo(hash)
    if err != nil {
       return nil, err
    }

    // block is PoS (type=2) when nonce is zero
    var blocktype uint8
    blocktype = 1
    nonce, _ := bi.Nonce.Int64()
    if nonce == 0 {
       blocktype = 2
    }
    bi.BlockHeader.Type = blocktype

    return bi, nil
}

// GetBlock returns block with given hash.
func (g *VeilRPC) GetBlock(hash string, height uint32) (*bchain.Block, error) {
   var err error
   if hash == "" && height > 0 {
      hash, err = g.GetBlockHash(height)
      if err != nil {
         return nil, err
      }
   }

   glog.V(1).Info("rpc: getblock (verbosity=1) ", hash)

   bi, err := g.GetBlockInfo(hash)
   if err != nil {
      return nil, err
   }

   txs := make([]bchain.Tx, 0, len(bi.Txids))
   for _, txid := range bi.Txids {
      tx, err := g.GetTransaction(txid)
      if err != nil {
         if isMissingTx(err) {
            glog.Errorf("rpc: getblock: skipping missing tx %s in block %s", txid, hash)
            continue
         }
         return nil, err
      }
      txs = append(txs, *tx)
   }

   block := &bchain.Block{
      BlockHeader: bi.BlockHeader,
      Txs:         txs,
   }
   return block, nil
}


func isMissingTx(err error) bool {
   return err == bchain.ErrTxNotFound
}
