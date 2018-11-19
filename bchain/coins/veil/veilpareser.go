package veil

import (
   "blockbook/bchain/coins/btc"
   "blockbook/bchain"
   "bytes"
   "encoding/binary"
   "encoding/hex"
   "io"
   "math/big"

   vlq "github.com/bsm/go-vlq"
   "github.com/btcsuite/btcd/blockchain"
   "github.com/btcsuite/btcd/chaincfg/chainhash"
   "github.com/btcsuite/btcd/wire"
   "github.com/jakm/btcutil"
   "github.com/jakm/btcutil/chaincfg"
   "github.com/jakm/btcutil/txscript"

   "fmt"
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

   // Veil testnet Address encoding magics
   TestNetParams = chaincfg.TestNet3Params
   TestNetParams.Net = TestnetMagic
   TestNetParams.PubKeyHashAddrID = []byte{111}
   TestNetParams.ScriptHashAddrID = []byte{196}
   TestNetParams.PrivateKeyID = []byte{239}
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
func (p *VeilParser) TxFromMsgTx(t *wire.MsgTx, parseAddresses bool) bchain.Tx {
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

   tx := bchain.Tx{
      // skip: Txid,
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

// ParseTx parses byte array containing transaction and returns Tx struct
func (p *VeilParser) ParseTx(b []byte) (*bchain.Tx, error) {
   t := wire.MsgTx{}
   r := bytes.NewReader(b)
   if err := UnserializeTransaction(&t, r); err != nil {
      return nil, err
   }
   tx := p.TxFromMsgTx(&t, true)
   tx.Txid = chainhash.DoubleHashH(b).String()
   tx.Hex = hex.EncodeToString(b)
   return &tx, nil
}

// ParseBlock parses raw block to our Block struct
func (p *VeilParser) ParseBlock(b []byte) (*bchain.Block, error) {
   w := wire.MsgBlock{}
   r := bytes.NewReader(b)

   if err := w.Deserialize(r); err != nil {
      return nil, err
   }

   txs := make([]bchain.Tx, len(w.Transactions))
   for ti, t := range w.Transactions {
      txs[ti] = p.TxFromMsgTx(t, false)
   }

   return &bchain.Block{
      BlockHeader: bchain.BlockHeader{
         Size: len(b),
         Time: w.Header.Timestamp.Unix(),
      },
      Txs: txs,
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

// -------- TX UNSERIALIZATION (replaces BtcDecode) --------

const (
   minTxInPayload = 41
   MinTxOutPayload = 9
   maxTxInPerMessage = (wire.MaxMessagePayload / minTxInPayload) + 1
   maxTxOutPerMessage = (wire.MaxMessagePayload / MinTxOutPayload) + 1
   maxWitnessItemsPerInput = 500000
)

func UnserializeTransaction(msg *wire.MsgTx, r io.Reader) error {
   *msg = wire.MsgTx{}

   // Tx Version + Tx Type
   version, err := Uint16(r, binary.LittleEndian)
   if err != nil {
      return err
   }
   msg.Version = int32(version)

   // Read the fUseSegwit flag value
   fUseSegwit := false
   err = readElement(r, &fUseSegwit)
	if err != nil {
		return err
	}

   // Read nLockTime
   nLockTime, err := Uint32(r, binary.LittleEndian)
   if err != nil {
      return err
   }
   msg.LockTime = nLockTime

   // Read the txin count
   count, err := ReadVarInt(r)
	if err != nil {
		return err
	}

   // A count of zero (meaning no TxIn's to the uninitiated) indicates
	// this is a transaction with witness data.
   var flag [1]byte
   if count == 0 && fUseSegwit {
      // We need to read the flag, which is a single byte.
   	if _, err = io.ReadFull(r, flag[:]); err != nil {
   		return err
   	}
      // At the moment, the flag MUST be 0x01. In the future other
   	// flag types may be supported.
   	if flag[0] != 0x01 {
   		str := fmt.Sprintf("witness tx but flag byte is %x", flag)
   		return messageError("UnserializeTransaction", str)
   	}
   	// With the Segregated Witness specific fields decoded, we can
   	// now read in the actual txin count.
   	count, err = ReadVarInt(r)
   	if err != nil {
   		return err
   	}
   }

   // Prevent more input transactions than could possibly fit into a
	// message.  It would be possible to cause memory exhaustion and panics
	// without a sane upper bound on this count.
	if count > uint64(maxTxInPerMessage) {
		str := fmt.Sprintf("too many input transactions to fit into "+
			"max message size [count %d, max %d]", count,
			maxTxInPerMessage)
		return messageError("UnserializeTransaction", str)
	}

   // Deserialize the inputs.
   var totalScriptSize uint64
   txIns := make([]wire.TxIn, count)
   msg.TxIn = make([]*wire.TxIn, count)
   for i := uint64(0); i < count; i++ {
		ti := &txIns[i]
		msg.TxIn[i] = ti
		err = readTxIn(r, ti)
		if err != nil {
			return err
		}
		totalScriptSize += uint64(len(ti.SignatureScript))
	}

   // Read the txout count
   count, err = ReadVarInt(r)
	if err != nil {
		return err
	}

   // Prevent more output transactions than could possibly fit into a
	// message.  It would be possible to cause memory exhaustion and panics
	// without a sane upper bound on this count.
	if count > uint64(maxTxOutPerMessage) {
		str := fmt.Sprintf("too many output transactions to fit into "+
			"max message size [count %d, max %d]", count,
			maxTxOutPerMessage)
		return messageError("MsgTx.BtcDecode", str)
	}

   // Deserialize the outputs.
	txOuts := make([]wire.TxOut, count)
	msg.TxOut = make([]*wire.TxOut, count)
	for i := uint64(0); i < count; i++ {
		to := &txOuts[i]
		msg.TxOut[i] = to
      // just nVersion = 1 for now
		_, err := readTxOut(r, to)
		if err != nil {
			return err
		}
		totalScriptSize += uint64(len(to.PkScript))
	}

   // Read witness data if present
   if fUseSegwit  {
      for _, txin := range msg.TxIn {
         // For each input, the witness is encoded as a stack
         // with one or more items. Therefore, we first read a
         // varint which encodes the number of stack items.
         witCount, err := ReadVarInt(r)
         if err != nil {
				return err
			}

         // Prevent a possible memory exhaustion attack by
			// limiting the witCount value to a sane upper bound.
			if witCount > maxWitnessItemsPerInput {
				str := fmt.Sprintf("too many witness items to fit "+
					"into max message size [count %d, max %d]",
					witCount, maxWitnessItemsPerInput)
				return messageError("MsgTx.BtcDecode", str)
			}

         // Then for witCount number of stack items, each item
			// has a varint length prefix, followed by the witness
			// item itself.
			txin.Witness = make([][]byte, witCount)
			for j := uint64(0); j < witCount; j++ {
				txin.Witness[j], err = readScript(r)
				if err != nil {
					return err
				}
				totalScriptSize += uint64(len(txin.Witness[j]))
			}
      }
   }

   return nil
}

// readScript reads a variable length byte array that represents a transaction
// script.  It is encoded as a varInt containing the length of the array
// followed by the bytes themselves.
func readScript(r io.Reader) ([]byte, error) {
	count, err := ReadVarInt(r)
	if err != nil {
		return nil, err
	}

	b := make([]byte, count)
	_, err = io.ReadFull(r, b)
	if err != nil {
		return nil, err
	}

	return b, nil
}

// readOutPoint reads the next sequence of bytes from r as an OutPoint.
func readOutPoint(r io.Reader, op *wire.OutPoint) error {
	_, err := io.ReadFull(r, op.Hash[:])
	if err != nil {
		return err
	}

	op.Index, err = Uint32(r, binary.LittleEndian)
	return err
}

// readTxIn reads the next sequence of bytes from r as a transaction input
// (TxIn).
func readTxIn(r io.Reader, ti *wire.TxIn) error {
	err := readOutPoint(r, &ti.PreviousOutPoint)
	if err != nil {
		return err
	}

	ti.SignatureScript, err = readScript(r)
	if err != nil {
		return err
	}

	return readElement(r, &ti.Sequence)
}

// readTxOut reads the next sequence of bytes from r as a transaction output
// (TxOut).
func readTxOut(r io.Reader, to *wire.TxOut) (uint8, error) {
   // read Tx type
   nVersion, err := Uint8(r)
   if err != nil {
      return 0, err
   }

	err = readElement(r, &to.Value)
	if err != nil {
		return 0, err
	}

	to.PkScript, err = readScript(r)
	return nVersion, err
}

// Uint8 reads one byte from the provided reader and
// returns the resulting uint8.
func Uint8(r io.Reader) (uint8, error) {
   buf := make([]byte, 1)

   if _, err := io.ReadFull(r, buf); err != nil {
      return 0, err
   }

   rv := buf[0]
   return rv, nil
}

// Uint16 reads two bytes from the provided reader, converts it
// to a number using the provided byte order, and returns the resulting uint16.
func Uint16(r io.Reader, byteOrder binary.ByteOrder) (uint16, error) {
   buf := make([]byte, 2)

   if _, err := io.ReadFull(r, buf); err != nil {
      return 0, err
   }

   rv := byteOrder.Uint16(buf)
   return rv, nil
}

// Uint32 reads four bytes from the provided reader, converts it
// to a number using the provided byte order, and returns the resulting uint32.
func Uint32(r io.Reader, byteOrder binary.ByteOrder) (uint32, error) {
   buf := make([]byte, 4)

   if _, err := io.ReadFull(r, buf); err != nil {
      return 0, err
   }

   rv := byteOrder.Uint32(buf)
   return rv, nil
}

// Uint64 reads eight bytes from the provided reader, converts it
// to a number using the provided byte order, and returns the resulting uint64.
func Uint64(r io.Reader, byteOrder binary.ByteOrder) (uint64, error) {
   buf := make([]byte, 8)

   if _, err := io.ReadFull(r, buf); err != nil {
      return 0, err
   }

   rv := byteOrder.Uint64(buf)
   return rv, nil
}

// ReadVarInt reads a variable length integer from r and returns it as a uint64.
func ReadVarInt(r io.Reader) (uint64, error) {
   discriminant, err := Uint8(r)
	if err != nil {
		return 0, err
	}

   var rv uint64
	switch discriminant {
	case 0xff:
		sv, err := Uint64(r, binary.LittleEndian)
		if err != nil {
			return 0, err
		}
		rv = sv
		// The encoding is not canonical if the value could have been
		// encoded using fewer bytes.
		min := uint64(0x100000000)
		if rv < min {
			return 0, messageError("ReadVarInt", "rv < min")
		}

	case 0xfe:
		sv, err := Uint32(r, binary.LittleEndian)
		if err != nil {
			return 0, err
		}
		rv = uint64(sv)
		// The encoding is not canonical if the value could have been
		// encoded using fewer bytes.
		min := uint64(0x10000)
		if rv < min {
         return 0, messageError("ReadVarInt", "rv < min")
		}

	case 0xfd:
		sv, err := Uint16(r, binary.LittleEndian)
		if err != nil {
			return 0, err
		}
		rv = uint64(sv)
		// The encoding is not canonical if the value could have been
		// encoded using fewer bytes.
		min := uint64(0xfd)
		if rv < min {
			return 0, messageError("ReadVarInt", "rv < min")
		}

	default:
		rv = uint64(discriminant)
	}

	return rv, nil
}

// readElement reads the next sequence of bytes from r using little endian
// depending on the concrete type of element pointed to.
func readElement(r io.Reader, element interface{}) error {
	// Attempt to read the element based on the concrete type via fast
	// type assertions first.
	switch e := element.(type) {
	case *int32:
		rv, err := Uint32(r, binary.LittleEndian)
		if err != nil {
			return err
		}
		*e = int32(rv)
		return nil

	case *uint32:
		rv, err := Uint32(r, binary.LittleEndian)
		if err != nil {
			return err
		}
		*e = rv
		return nil

	case *int64:
		rv, err := Uint64(r, binary.LittleEndian)
		if err != nil {
			return err
		}
		*e = int64(rv)
		return nil

	case *uint64:
		rv, err := Uint64(r, binary.LittleEndian)
		if err != nil {
			return err
		}
		*e = rv
		return nil

	case *bool:
		rv, err := Uint8(r)
		if err != nil {
			return err
		}
		if rv == 0x00 {
			*e = false
		} else {
			*e = true
		}
		return nil

   }

	// Fall back to the slower binary.Read if a fast path was not available
	// above.
	return binary.Read(r, binary.LittleEndian, element)
}

// readElements reads multiple items from r.  It is equivalent to multiple
// calls to readElement.
func readElements(r io.Reader, elements ...interface{}) error {
	for _, element := range elements {
		err := readElement(r, element)
		if err != nil {
			return err
		}
	}
	return nil
}


// --- ERRORS --- (from btcsuite/btcd/wire)
// MessageError describes an issue with a message.
// An example of some potential issues are messages from the wrong bitcoin
// network, invalid commands, mismatched checksums, and exceeding max payloads.
//
// This provides a mechanism for the caller to type assert the error to
// differentiate between general io errors such as io.EOF and issues that
// resulted from malformed messages.
type MessageError struct {
	Func        string // Function name
	Description string // Human readable description of the issue
}

// Error satisfies the error interface and prints human-readable errors.
func (e *MessageError) Error() string {
	if e.Func != "" {
		return fmt.Sprintf("%v: %v", e.Func, e.Description)
	}
	return e.Description
}

// messageError creates an error for the given function and description.
func messageError(f string, desc string) *MessageError {
	return &MessageError{Func: f, Description: desc}
}
