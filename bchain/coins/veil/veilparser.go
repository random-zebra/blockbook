package veil

import (
   "bytes"
   "blockbook/bchain/coins/btc"
   "blockbook/bchain"
   "encoding/binary"
   "encoding/hex"
   "encoding/json"
   "fmt"
   "io"
   "math/big"

   "github.com/golang/glog"
   "github.com/martinboehm/btcd/wire"
   "github.com/martinboehm/btcutil/chaincfg"
)

const (
   // Net Magics
   MainnetMagic wire.BitcoinNet = 0xa3d0cfb6
   TestnetMagic wire.BitcoinNet = 0xc4a7d1a8

   // Zerocoin op codes
   OP_ZEROCOINMINT  = 0xc1
   OP_ZEROCOINSPEND  = 0xc2

   // Labels
   ZCMINT_LABEL = "Zerocoin Mint"
   ZCSPEND_LABEL = "Zerocoin Spend"
   CBASE_LABEL = "CoinBase TX"
   STAKE_LABEL = "CoinStake TX"
   //DATA_LABEL = "DATA"
   RINGCT_LABEL = "RingCT"
   CTDATA_LABEL = "Rangeproof"

   // Dummy Internal Addresses
   STAKE_ADDR_INT = 0xf7
   RINGCT_ADDR_INT = 0xf8
   CTDATA_ADDR_INT = 0xf9
   CBASE_ADDR_INT = 0xfa

   // Number of blocks per budget cycle
   nBlocksPerPeriod = 43200
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
   BitcoinGetAddrDescFromAddress        func(address string) (bchain.AddressDescriptor, error)
}


// NewVeilParser returns new VeilParser instance
func NewVeilParser(params *chaincfg.Params, c *btc.Configuration) *VeilParser {
   bcp := btc.NewBitcoinParser(params, c)
   p := &VeilParser{
       BitcoinParser:   bcp,
       baseparser:      &bchain.BaseParser{},
       BitcoinGetAddrDescFromAddress: bcp.GetAddrDescFromAddress,
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

        if vout.ScriptPubKey.Hex == "" {
            if vout.Type == "ringct" {
                vout.ScriptPubKey.Hex = fmt.Sprintf("%02x", RINGCT_ADDR_INT)
            } else if vout.Type == "data" {
                vout.ScriptPubKey.Hex = fmt.Sprintf("%02x", CTDATA_ADDR_INT)
            } else if vout.Type == "coinbase" {
                vout.ScriptPubKey.Hex = fmt.Sprintf("%02x", CBASE_ADDR_INT)
            } else if vout.Type == "standard" {
                vout.ScriptPubKey.Hex = fmt.Sprintf("%02x", STAKE_ADDR_INT)
            }
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
    if isCoinBaseScript(script) {
        return []string{CBASE_LABEL}, false, nil
    }
    if isCoinStakeScript(script) {
        return []string{STAKE_LABEL}, false, nil
    }
    if isRangeProofScript(script) {
        return []string{CTDATA_LABEL}, false, nil
    }
    if isRingCTScript(script) {
        return []string{RINGCT_LABEL}, false, nil
    }

	rv, s, _ := p.BitcoinOutputScriptToAddressesFunc(script)
	return rv, s, nil
}

// GetAddrDescFromAddress returns internal address representation (descriptor) of given address
func (p *VeilParser) GetAddrDescFromAddress(address string) (bchain.AddressDescriptor, error) {
    // dummy address for cbase output
   if address == STAKE_LABEL {
      return bchain.AddressDescriptor{CBASE_ADDR_INT}, nil
    }
    // dummy address for stake output
   if address == STAKE_LABEL {
      return bchain.AddressDescriptor{STAKE_ADDR_INT}, nil
	}
   // dummy address for RingCT output
   if address == RINGCT_LABEL {
      return bchain.AddressDescriptor{RINGCT_ADDR_INT}, nil
   }
   // dummy address for Rangeproof output
   if address == CTDATA_LABEL {
      return bchain.AddressDescriptor{CTDATA_ADDR_INT}, nil
   }
   return p.BitcoinGetAddrDescFromAddress(address)
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

func (p *VeilParser) GetValueSatForUnknownInput(tx *bchain.Tx, input int) *big.Int {
	if len(tx.Vin) > input {
		scriptHex := tx.Vin[input].ScriptSig.Hex

		if scriptHex != "" {
			script, _ := hex.DecodeString(scriptHex)
			if isZeroCoinSpendScript(script) {
                valueSat,  err := p.GetValueSatFromZerocoinSpend(script)
                if err != nil {
                    glog.Warningf("tx %v: input %d unable to convert denom to big int", tx.Txid, input)
                    return big.NewInt(0)
                }
                return valueSat
            }
		}
	}
    return big.NewInt(0)
}

// Decodes the amount from the zerocoin spend script
func (p *VeilParser) GetValueSatFromZerocoinSpend(signatureScript []byte) (*big.Int, error) {
    r := bytes.NewReader(signatureScript)
    r.Seek(1, io.SeekCurrent)                       // skip opcode
    len, err := Uint8(r)                            // get serialized coinspend size
    if err != nil {
        return nil, err
    }
    r.Seek(int64(len), io.SeekCurrent)              // and skip its bytes
    r.Seek(2, io.SeekCurrent)                       // skip version and spendtype
    len,  err = Uint8(r)                            // get pubkey len
    if err != nil {
        return nil, err
    }
    r.Seek(int64(len), io.SeekCurrent)              // and skip its bytes
    len, err = Uint8(r)                             // get vchsig len
    if err != nil {
        return nil, err
    }
    r.Seek(int64(len), io.SeekCurrent)              // and skip its bytes
    // get denom
    denom, err := Uint32(r, binary.LittleEndian)    // get denomination
    if err != nil {
        return nil, err
    }

    return big.NewInt(int64(denom)*1e8), nil
}

// Checks if script is OP_ZEROCOINMINT
func isZeroCoinMintScript(signatureScript []byte) bool {
	return len(signatureScript) > 1 && signatureScript[0] == OP_ZEROCOINMINT
}

// Checks if script is OP_ZEROCOINSPEND
func isZeroCoinSpendScript(signatureScript []byte) bool {
	return len(signatureScript) >= 100 && signatureScript[0] == OP_ZEROCOINSPEND
}

// Checks if script is dummy internal address for Coinbase
func isCoinBaseScript(signatureScript []byte) bool {
	return len(signatureScript) == 1 && signatureScript[0] == CBASE_ADDR_INT
}

// Checks if script is dummy internal address for Stake
func isCoinStakeScript(signatureScript []byte) bool {
	return len(signatureScript) == 1 && signatureScript[0] == STAKE_ADDR_INT
}

// Checks if script is dummy internal address for RangeProof
func isRangeProofScript(signatureScript []byte) bool {
	return len(signatureScript) == 1 && signatureScript[0] == CTDATA_ADDR_INT
}

// Checks if script is dummy internal address for RingCT
func isRingCTScript(signatureScript []byte) bool {
	return len(signatureScript) == 1 && signatureScript[0] == RINGCT_ADDR_INT
}
