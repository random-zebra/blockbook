package pivx

import (
   "blockbook/bchain"
   "blockbook/bchain/coins/btc"
   "encoding/json"

   "github.com/golang/glog"
   "github.com/juju/errors"
)

// PivxRPC is an interface to JSON-RPC pivxd service.
type PivxRPC struct {
   *btc.BitcoinRPC
}

func NewPivxRPC(config json.RawMessage, pushHandler func(bchain.NotificationType)) (bchain.BlockChain, error) {
   b, err := btc.NewBitcoinRPC(config, pushHandler)
   if err != nil {
      return nil, err
   }

   g := &PivxRPC{
      BitcoinRPC: b.(*btc.BitcoinRPC),
   }

   g.RPCMarshaler = btc.JSONMarshalerV1{}
   g.ChainConfig.SupportsEstimateSmartFee = false

   return g, nil
}

// Initialize initializes PivxRPC instance.
func (b *PivxRPC) Initialize() error {
   chainName, err := b.GetChainInfoAndInitializeMempool(b)
   if err != nil {
      return err
   }

   params := GetChainParams(chainName)

   // always create parser
   b.Parser = NewPivxParser(params, b.ChainConfig)

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


// GetBlock returns block with given hash.
func (g *PivxRPC) GetBlock(hash string, height uint32) (*bchain.Block, error) {
   var err error
   if hash == "" && height > 0 {
      hash, err = g.GetBlockHash(height)
      if err != nil {
         return nil, err
      }
   }

   glog.V(1).Info("rpc: getblock (verbosity=1) ", hash)

   res := btc.ResGetBlockThin{}
   req := btc.CmdGetBlock{Method: "getblock"}
   req.Params.BlockHash = hash
   req.Params.Verbosity = 1
   err = g.Call(&req, &res)

   if err != nil {
      return nil, errors.Annotatef(err, "hash %v", hash)
   }
   if res.Error != nil {
      return nil, errors.Annotatef(res.Error, "hash %v", hash)
   }

   txs := make([]bchain.Tx, 0, len(res.Result.Txids))
   for _, txid := range res.Result.Txids {
      tx, err := g.GetTransaction(txid)
      if err != nil {
         if isInvalidTx(err) {
            glog.Errorf("rpc: getblock: skipping transanction in block %s due error: %s", hash, err)
            continue
         }
         return nil, err
      }
      txs = append(txs, *tx)
   }
   block := &bchain.Block{
      BlockHeader: res.Result.BlockHeader,
      Txs:         txs,
   }
   return block, nil
}

func isInvalidTx(err error) bool {
   switch e1 := err.(type) {
   case *errors.Err:
      switch e2 := e1.Cause().(type) {
      case *bchain.RPCError:
         if e2.Code == -5 { // "No information available about transaction"
            return true
         }
      }
   }

   return false
}

// GetTransactionForMempool returns a transaction by the transaction ID.
// It could be optimized for mempool, i.e. without block time and confirmations
func (p *PivxRPC) GetTransactionForMempool(txid string) (*bchain.Tx, error) {
   return p.GetTransaction(txid)
}

// GetMempoolEntry returns mempool data for given transaction
func (p *PivxRPC) GetMempoolEntry(txid string) (*bchain.MempoolEntry, error) {
   return nil, errors.New("GetMempoolEntry: not implemented")
}

func isErrBlockNotFound(err *bchain.RPCError) bool {
   return err.Message == "Block not found" || err.Message == "Block height out of range"
}
