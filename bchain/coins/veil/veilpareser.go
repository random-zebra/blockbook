package veil

import (
   "blockbook/bchain/coins/btc"
   "blockbook/bchain"
   "bytes"
   "encoding/binary"
   "encoding/hex"
   "math/big"

   vlq "github.com/bsm/go-vlq"
   "github.com/btcsuite/btcd/blockchain"
   "github.com/btcsuite/btcd/chaincfg/chainhash"
   "github.com/btcsuite/btcd/wire"
   "github.com/jakm/btcutil"
   "github.com/jakm/btcutil/chaincfg"
   "github.com/jakm/btcutil/txscript"
)

const (
   // Net Magics
   MainnetMagic wire.BitcoinNet = 0xa3d0cfb6
   TestnetMagic wire.BitcoinNet = 0xc4a7d1a8

   // Zerocoin op codes
   OP_ZEROCOINMINT  = 0xc1
   OP_ZEROCOINSPEND  = 0xc2

   // Dummy Internal Address for Stakes outputs
   STAKE_ADDR_INT = 0xf7

   // Labels
   ZEROCOIN_LABEL = "Zerocoin Accumulator"
   STAKE_LABEL = "Proof of Stake TX"
)

var (
   MainNetParams chaincfg.Params
   TestNetParams chaincfg.Params
)

func init() {
   // Veil mainnet Address encoding magics
   MainNetParams = chaincfg.MainNetParams
   MainNetParams.Net = MainnetMagic
   MainNetParams.PubKeyHashAddrID = []byte{70}
   MainNetParams.ScriptHashAddrID = []byte{5}
   MainNetParams.PrivateKeyID = []byte{128}
   MainNetParams.Bech32HRPSegwit = "bv"

   // Veil testnet Address encoding magics
   TestNetParams = chaincfg.TestNet3Params
   TestNetParams.Net = TestnetMagic
   TestNetParams.PubKeyHashAddrID = []byte{111}
   TestNetParams.ScriptHashAddrID = []byte{196}
   TestNetParams.PrivateKeyID = []byte{239}
   TestNetParams.Bech32HRPSegwit = "tv"
}

// VeilParser handle
type VeilParser struct {
   *btc.BitcoinParser
}

// NewVeilParser returns new VeilParser instance
func NewVeilParser(params *chaincfg.Params, c *btc.Configuration) *VeilParser {
   p := &VeilParser{BitcoinParser: btc.NewBitcoinParser(params, c)}
   p.OutputScriptToAddressesFunc = p.outputScriptToAddresses
      return p
}

// GetChainParams contains network parameters for the main and test Veil network
func GetChainParams(chain string) *chaincfg.Params {
   if !chaincfg.IsRegistered(&MainNetParams) {
      err := chaincfg.Register(&MainNetParams)
      if err == nil {
         err = chaincfg.Register(&TestNetParams)
      }
      if err != nil {
         panic(err)
      }
   }
   switch chain {
   case "test":
      return &TestNetParams
   default:
      return &MainNetParams
   }
}

// GetAddrDescFromVout returns internal address representation (descriptor) of given transaction output
func (p *VeilParser) GetAddrDescFromVout(output *bchain.Vout) (bchain.AddressDescriptor, error) {
   // Stake first output
   if output.ScriptPubKey.Hex == "" {
      return bchain.AddressDescriptor{STAKE_ADDR_INT}, nil
  	}
   // zerocoin mint output
   if len(output.ScriptPubKey.Hex) > 1 && output.ScriptPubKey.Hex[:2] == hex.EncodeToString([]byte{OP_ZEROCOINMINT}) {
      return bchain.AddressDescriptor{OP_ZEROCOINMINT}, nil
	}
   // P2PK/P2PKH outputs
   ad, err := hex.DecodeString(output.ScriptPubKey.Hex)
   if err != nil {
      return ad, err
   }
   // convert possible P2PK script to P2PKH
   // so that all transactions by given public key are indexed together
   return txscript.ConvertP2PKtoP2PKH(ad)
}

// GetAddrDescFromAddress returns internal address representation (descriptor) of given address
func (p *VeilParser) GetAddrDescFromAddress(address string) (bchain.AddressDescriptor, error) {
   return p.addressToOutputScript(address)
}

// GetAddressesFromAddrDesc returns addresses for given address descriptor with flag if the addresses are searchable
func (p *VeilParser) GetAddressesFromAddrDesc(addrDesc bchain.AddressDescriptor) ([]string, bool, error) {
   return p.OutputScriptToAddressesFunc(addrDesc)
}

// addressToOutputScript converts Veil address to ScriptPubKey
func (p *VeilParser) addressToOutputScript(address string) ([]byte, error) {
   // dummy address for stake output
   if address == STAKE_LABEL {
      return bchain.AddressDescriptor{STAKE_ADDR_INT}, nil
	}
   // dummy address for zerocoin mint output
   if address == ZEROCOIN_LABEL {
      return bchain.AddressDescriptor{OP_ZEROCOINMINT}, nil
   }
   // regular address
   da, err := btcutil.DecodeAddress(address, p.Params)
   if err != nil {
      return nil, err
   }
   script, err := txscript.PayToAddrScript(da)
   if err != nil {
      return nil, err
   }
   return script, nil
}

// outputScriptToAddresses converts ScriptPubKey to addresses with a flag that the addresses are searchable
func (p *VeilParser) outputScriptToAddresses(script []byte) ([]string, bool, error) {
   // empty script --> newly generated coins
   if len(script) == 0 {
      return nil, false, nil
   }

   // coinstake tx output
   if len(script) > 0 && script[0] == STAKE_ADDR_INT {
      return []string{STAKE_LABEL}, false, nil
   }

   // zerocoin mint output
   ozm := TryParseOPZerocoinMint(script)
   if ozm != "" {
      return []string{ozm}, false, nil
   }

   // basecoin tx output
   sc, addresses, _, err := txscript.ExtractPkScriptAddrs(script, p.Params)

   if err != nil {
      return nil, false, err
   }
   rv := make([]string, len(addresses))

   for i, a := range addresses {
      rv[i] = a.EncodeAddress()
   }
   var s bool

   if sc == txscript.PubKeyHashTy || sc == txscript.WitnessV0PubKeyHashTy ||
   sc == txscript.ScriptHashTy || sc == txscript.WitnessV0ScriptHashTy {
      s = true
   } else if len(addresses) == 0 {
      or := btc.TryParseOPReturn(script)
      if or != "" {
         rv = []string{or}
      }
   }
   return rv, s, nil
}

// TxFromMsgTx returns the transaction from wire msg
func (p *VeilParser) TxFromMsgTx(t *wire.MsgTx, tx *Tx, parseAddresses bool) bchain.Tx {
   // Tx Inputs
   vin := make([]bchain.Vin, len(t.TxIn))
   for i, in := range t.TxIn {
      if blockchain.IsCoinBaseTx(t) {
         vin[i] = bchain.Vin{
            Coinbase: hex.EncodeToString(in.SignatureScript),
            Sequence: in.Sequence,
         }
         break
      }

      s := bchain.ScriptSig{
         Hex: hex.EncodeToString(in.SignatureScript),
      }

      vin[i] = bchain.Vin{
         Sequence:  in.Sequence,
         ScriptSig: s,
      }

      // zerocoin spends have no PreviousOutPoint
      if in.SignatureScript[0] != OP_ZEROCOINSPEND {
         vin[i].Txid = in.PreviousOutPoint.Hash.String()
         vin[i].Vout = in.PreviousOutPoint.Index
      }
   }
   // Tx Outputs
   vout := make([]bchain.Vout, len(t.TxOut))
   for i, out := range t.TxOut {
      addrs := []string{}
      if parseAddresses {
         if len(out.PkScript) > 0 {
            addrs, _, _ = p.OutputScriptToAddressesFunc(out.PkScript)
         } else {
         // stake tx script
         addrs = []string{STAKE_LABEL}
         }
      }

      s := bchain.ScriptPubKey{
         Hex:       hex.EncodeToString(out.PkScript),
         Addresses: addrs,
      }

      var vs big.Int
      vs.SetInt64(out.Value)
      vout[i] = bchain.Vout{
         N:            uint32(i),
         ScriptPubKey: s,
         ValueSat:	  vs,
      }
   }

   bchaintx := bchain.Tx{
      Txid:     TxHash(t, tx).String(),
      Version:  t.Version,
      LockTime: t.LockTime,
      Vin:      vin,
      Vout:     vout,
      // skip: BlockHash,
      // skip: Confirmations,
      // skip: Time,
      // skip: Blocktime,
   }

   return bchaintx
}

// ParseTx parses byte array containing transaction and returns Tx struct
func (p *VeilParser) ParseTx(b []byte) (*bchain.Tx, error) {
   tm := wire.MsgTx{}
   tx := Tx{}
   r := bytes.NewReader(b)
   if err := UnserializeTx(&tm, &tx, r); err != nil {
      return nil, err
   }
   bchaintx := p.TxFromMsgTx(&tm, &tx, true)
   bchaintx.Hex = hex.EncodeToString(b)
   return &bchaintx, nil
}

// ParseBlock parses raw block to our Block struct
func (p *VeilParser) ParseBlock(b []byte) (*bchain.Block, error) {
   w := wire.MsgBlock{}
   r := bytes.NewReader(b)
   blk := TxBlock{}
   hashWitnessMerkleRoot := chainhash.Hash{}
   hashAccumulators := chainhash.Hash{}

   if err := UnserializeBlock(&w, &blk, &hashWitnessMerkleRoot, &hashAccumulators,
         r); err != nil {
      return nil, err
   }

   bchaintxs := make([]bchain.Tx, len(w.Transactions))

   for ti, t := range w.Transactions {
      bchaintxs[ti] = p.TxFromMsgTx(t, blk.txs[ti], false)
   }

   return &bchain.Block{
      BlockHeader: bchain.BlockHeader{
         Size: len(b),
         Time: w.Header.Timestamp.Unix(),
      },
      Txs: bchaintxs,
   }, nil
}

// PackTx packs transaction to byte array
func (p *VeilParser) PackTx(tx *bchain.Tx, height uint32, blockTime int64) ([]byte, error) {
   buf := make([]byte, 4+vlq.MaxLen64+len(tx.Hex)/2)
   binary.BigEndian.PutUint32(buf[0:4], height)
   vl := vlq.PutInt(buf[4:4+vlq.MaxLen64], blockTime)
   hl, err := hex.Decode(buf[4+vl:], []byte(tx.Hex))
   return buf[0 : 4+vl+hl], err
}

// UnpackTx unpacks transaction from byte array
func (p *VeilParser) UnpackTx(buf []byte) (*bchain.Tx, uint32, error) {
   height := binary.BigEndian.Uint32(buf)
   bt, l := vlq.Int(buf[4:])
   tx, err := p.ParseTx(buf[4+l:])
   if err != nil {
      return nil, 0, err
   }
   tx.Blocktime = bt

   return tx, height, nil
}

// TryParseOPZerocoinMint tries to process
// OP_ZEROCOINMINT script and returns its string representation
func TryParseOPZerocoinMint(script []byte) string {
   if len(script) > 0 && script[0] == OP_ZEROCOINMINT {
      return ZEROCOIN_LABEL
   }
   return ""
}
