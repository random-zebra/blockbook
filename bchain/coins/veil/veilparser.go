package veil

import (
   "blockbook/bchain/coins/btc"
   "blockbook/bchain"
   "encoding/hex"
   "encoding/json"
   "math/big"

   "github.com/martinboehm/btcd/blockchain"
   "github.com/martinboehm/btcd/wire"
   "github.com/martinboehm/btcutil/chaincfg"
)

const (
   // Net Magics
   MainnetMagic wire.BitcoinNet = 0xa3d0cfb6
   TestnetMagic wire.BitcoinNet = 0xc4a7d1a8

   // Dummy TxId for zerocoin
   ZERO_INPUT = "0000000000000000000000000000000000000000000000000000000000000000"

   // Zerocoin op codes
   OP_ZEROCOINMINT  = 0xc1
   OP_ZEROCOINSPEND  = 0xc2

   // Labels
   ZCMINT_LABEL = "Zerocoin Mint"
   ZCSPEND_LABEL = "Zerocoin Spend"
   STAKE_LABEL = "Stake TX"
   //DATA_LABEL = "DATA"
   RINGCT_LABEL = "RingCT"
   CTDATA_LABEL = "Rangeproof"
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
   baseparser                           *bchain.BaseParser
   BitcoinOutputScriptToAddressesFunc   btc.OutputScriptToAddressesFunc
}


// NewVeilParser returns new VeilParser instance
func NewVeilParser(params *chaincfg.Params, c *btc.Configuration) *VeilParser {
   p := &VeilParser{
       BitcoinParser:   btc.NewBitcoinParser(params, c),
       baseparser:      &bchain.BaseParser{},
   }
   p.BitcoinOutputScriptToAddressesFunc = p.OutputScriptToAddressesFunc
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

// PackTx packs transaction to byte array using protobuf
func (p *VeilParser) PackTx(tx *bchain.Tx, height uint32, blockTime int64) ([]byte, error) {
	return p.baseparser.PackTx(tx, height, blockTime)
}


// UnpackTx unpacks transaction from protobuf byte array
func (p *VeilParser) UnpackTx(buf []byte) (*bchain.Tx, uint32, error) {
	return p.baseparser.UnpackTx(buf)
}


// Parses tx and adds handling for OP_ZEROCOINSPEND inputs
func (p *VeilParser) TxFromMsgTx(t *wire.MsgTx, parseAddresses bool) bchain.Tx {
	vin := make([]bchain.Vin, len(t.TxIn))
	for i, in := range t.TxIn {
		// extra check to not confuse Tx with single OP_ZEROCOINSPEND input as a coinbase Tx
		if !isZeroCoinSpendScript(in.SignatureScript) && blockchain.IsCoinBaseTx(t) {
			vin[i] = bchain.Vin{
				Coinbase: hex.EncodeToString(in.SignatureScript),
				Sequence: in.Sequence,
			}
			break
		}

		s := bchain.ScriptSig{
			Hex: hex.EncodeToString(in.SignatureScript),
			// missing: Asm,
		}

		txid := in.PreviousOutPoint.Hash.String()

		vin[i] = bchain.Vin{
			Txid:      txid,
			Vout:      in.PreviousOutPoint.Index,
			Sequence:  in.Sequence,
			ScriptSig: s,
		}
	}
	vout := make([]bchain.Vout, len(t.TxOut))
	for i, out := range t.TxOut {
		addrs := []string{}
		if parseAddresses {
			addrs, _, _ = p.OutputScriptToAddressesFunc(out.PkScript)
		}
		s := bchain.ScriptPubKey{
			Hex:       hex.EncodeToString(out.PkScript),
			Addresses: addrs,
			// missing: Asm,
			// missing: Type,
		}
		var vs big.Int
		vs.SetInt64(out.Value)
		vout[i] = bchain.Vout{
			ValueSat:     vs,
			N:            uint32(i),
			ScriptPubKey: s,
		}
	}
	tx := bchain.Tx{
		Txid:     t.TxHash().String(),
		Version:  t.Version,
		LockTime: t.LockTime,
		Vin:      vin,
		Vout:     vout,
		// skip: BlockHash,
		// skip: Confirmations,
		// skip: Time,
		// skip: Blocktime,
	}
	return tx
}

// ParseTxFromJson parses JSON message containing transaction and returns Tx struct
func (p *VeilParser) ParseTxFromJson(msg json.RawMessage) (*bchain.Tx, error) {
	var tx bchain.Tx
	err := json.Unmarshal(msg, &tx)
	if err != nil {
		return nil, err
	}

	for i := range tx.Vout {
		vout := &tx.Vout[i]
		// convert vout.JsonValue to big.Int and clear it, it is only temporary value used for unmarshal
		vout.ValueSat, err = p.AmountToBigInt(vout.JsonValue)
		if err != nil {
			return nil, err
		}
		vout.JsonValue = ""

		if vout.ScriptPubKey.Addresses == nil {
			vout.ScriptPubKey.Addresses = []string{}
		}
	}

	return &tx, nil
}

// outputScriptToAddresses converts ScriptPubKey to bitcoin addresses
func (p *VeilParser) outputScriptToAddresses(script []byte) ([]string, bool, error) {
	if isZeroCoinSpendScript(script) {
		return []string{ZCSPEND_LABEL}, false, nil
	}
	if isZeroCoinMintScript(script) {
		return []string{ZCMINT_LABEL}, false, nil
	}

	rv, s, _ := p.BitcoinOutputScriptToAddressesFunc(script)
	return rv, s, nil
}

func (p *VeilParser) GetAddrDescForUnknownInput(tx *bchain.Tx, input int) bchain.AddressDescriptor {
	if len(tx.Vin) > input {
		scriptHex := tx.Vin[input].ScriptSig.Hex

		if scriptHex != "" {
			script, _ := hex.DecodeString(scriptHex)
			return script
		}
	}

	s := make([]byte, 10)
	return s
}

// Checks if script is OP_ZEROCOINMINT
func isZeroCoinMintScript(signatureScript []byte) bool {
	return len(signatureScript) > 1 && signatureScript[0] == OP_ZEROCOINMINT
}

// Checks if script is OP_ZEROCOINSPEND
func isZeroCoinSpendScript(signatureScript []byte) bool {
	return len(signatureScript) >= 100 && signatureScript[0] == OP_ZEROCOINSPEND
}
