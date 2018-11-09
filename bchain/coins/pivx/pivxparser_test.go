// +build unittest

package pivx

import (
   "blockbook/bchain"
   "blockbook/bchain/coins/btc"
   "bytes"
   "encoding/hex"
   "math/big"
   "os"
   "reflect"
   "testing"

   "github.com/jakm/btcutil/chaincfg"
)

func TestMain(m *testing.M) {
   c := m.Run()
   chaincfg.ResetParams()
   os.Exit(c)
}

func Test_GetAddrDescFromAddress(t *testing.T) {
   type args struct {
      address string
   }
   tests := []struct {
      name    string
      args    args
      want    string
      wantErr bool
   }{
      {
         name:    "P2PKH",
         args:    args{address: "D6JGN6nUgE9UcvFwPv9oUw5stGeNj2NTJc"},
         want:    "76a9140cb412217817cb84feebe8ef3c5089182aa593ec88ac",
         wantErr: false,
      },
      {
         name:    "P2PKH from P2PK",
         args:    args{address: "DE6T9kVSJsUzT3q7MMighWHzVSztDuGaSX"},
         want:    "76a91462391713c14e102f941238ade41d2abb27df3eee88ac",
         wantErr: false,
      },
   }
   parser := NewPivxParser(GetChainParams("main"), &btc.Configuration{})

   for _, tt := range tests {
      t.Run(tt.name, func(t *testing.T) {
         got, err := parser.GetAddrDescFromAddress(tt.args.address)
         if (err != nil) != tt.wantErr {
            t.Errorf("GetAddrDescFromAddress() error = %v, wantErr %v", err, tt.wantErr)
            return
         }
         h := hex.EncodeToString(got)
         if !reflect.DeepEqual(h, tt.want) {
            t.Errorf("GetAddrDescFromAddress() = %v, want %v", h, tt.want)
         }
      })
   }
}

func Test_GetAddrDescFromVout(t *testing.T) {
   type args struct {
      vout bchain.Vout
   }
   tests := []struct {
      name    string
      args    args
      want    string
      wantErr bool
   }{
      {
         name:    "P2PKH",
         args:    args{vout: bchain.Vout{ScriptPubKey: bchain.ScriptPubKey{Hex: "76a9140cb412217817cb84feebe8ef3c5089182aa593ec88ac"}}},
         want:    "76a9140cb412217817cb84feebe8ef3c5089182aa593ec88ac",
         wantErr: false,
      },
      {
         name:    "ZEROCOIN MINT",
         args:    args{vout: bchain.Vout{ScriptPubKey: bchain.ScriptPubKey{Hex: "c10280004c809f63d910f2175f5a29d2ec6c158b4d2b9724fc4e6bf55b53dee9929364478c1e64c0fb5d90bbacbab02dc1b9639908565cac2b6d68c8394997d3e61f4c027d0ed583d4644d4a9a1a79f710d2143d9095c50e6ea2604b841801c6acb76bf9b3ffc71576002e4cdd41f9950ec379dea069170cd048d519a0ae113672427bb8293f"}}},
         want:    hex.EncodeToString(bchain.AddressDescriptor{OP_ZEROCOINMINT}),
         wantErr: false,
      },
   }
   parser := NewPivxParser(GetChainParams("main"), &btc.Configuration{})

   for _, tt := range tests {
      t.Run(tt.name, func(t *testing.T) {
         got, err := parser.GetAddrDescFromVout(&tt.args.vout)
         if (err != nil) != tt.wantErr {
            t.Errorf("GetAddrDescFromVout() error = %v, wantErr %v", err, tt.wantErr)
            return
         }
         h := hex.EncodeToString(got)
         if !reflect.DeepEqual(h, tt.want) {
            t.Errorf("GetAddrDescFromVout() = %v, want %v", h, tt.want)
         }
      })
   }
}

func Test_GetAddressesFromAddrDesc(t *testing.T) {
   type args struct {
      script string
   }
   tests := []struct {
      name    string
      args    args
      want    []string
      want2   bool
      wantErr bool
   }{
      {
         name:    "P2PKH",
         args:    args{script: "76a9140cb412217817cb84feebe8ef3c5089182aa593ec88ac"},
         want:    []string{"D6JGN6nUgE9UcvFwPv9oUw5stGeNj2NTJc"},
         want2:   true,
         wantErr: false,
      },
      {
         name:    "OP_RETURN ascii",
         args:    args{script: "6a0461686f6a"},
         want:    []string{"OP_RETURN (ahoj)"},
         want2:   false,
         wantErr: false,
      },
      {
         name:    "OP_RETURN OP_PUSHDATA1 ascii",
         args:    args{script: "6a4c0b446c6f7568792074657874"},
         want:    []string{"OP_RETURN (Dlouhy text)"},
         want2:   false,
         wantErr: false,
      },
      {
         name:    "OP_RETURN hex",
         args:    args{script: "6a072020f1686f6a20"},
         want:    []string{"OP_RETURN 2020f1686f6a20"},
         want2:   false,
         wantErr: false,
      },
      {
         name:    "OP_ZEROCOINMINT",
         args:    args{script: "c10280004c809f63d910f2175f5a29d2ec6c158b4d2b9724fc4e6bf55b53dee9929364478c1e64c0fb5d90bbacbab02dc1b9639908565cac2b6d68c8394997d3e61f4c027d0ed583d4644d4a9a1a79f710d2143d9095c50e6ea2604b841801c6acb76bf9b3ffc71576002e4cdd41f9950ec379dea069170cd048d519a0ae113672427bb8293f"},
         want:    []string{ZPIV_LABEL},
         want2:   false,
         wantErr: false,
      },
   }

   parser := NewPivxParser(GetChainParams("main"), &btc.Configuration{})

   for _, tt := range tests {
      t.Run(tt.name, func(t *testing.T) {
         b, _ := hex.DecodeString(tt.args.script)
         got, got2, err := parser.GetAddressesFromAddrDesc(b)
         if (err != nil) != tt.wantErr {
            t.Errorf("GetAddressesFromAddrDesc() error = %v, wantErr %v", err, tt.wantErr)
            return
         }
         if !reflect.DeepEqual(got, tt.want) {
            t.Errorf("GetAddressesFromAddrDesc() = %v, want %v", got, tt.want)
         }
         if !reflect.DeepEqual(got2, tt.want2) {
            t.Errorf("GetAddressesFromAddrDesc() = %v, want %v", got2, tt.want2)
         }
      })
   }
}

var (
   basecoinTx bchain.Tx
   zerocoinMintTx bchain.Tx

   basecoinTxPacked = "001607ac8bbdfd9a46010000000105a2b972575b810e15b9ea073afc7ada87444c53096bb6af42bd539e0c352f80000000006b48304502210098552d04e56f9bb17a4ce1504d9c41207966f9d00d551e789a1ff3c16dc21a7202206b2db55365b2c0325226b8c33d389c0537e07c71e636cecab45772a19d1c68100121036e38b81971b79e160f2a405f9673bee9b54a8901827163fcbb56593b6c35ebb6ffffffff012ad8f505000000001976a9141463d364a1afbdb5da5de66efe68e711c29c300d88ac00000000"
   zerocoinMintTxPacked = "0016078c8bbdfd833601000000011f3d790b96a9a2b995c5082beb703e5ebbd7e2981e60a285bed04e3a8da8027b000000006a47304402204e285305d4b93a4ba50b1f4b8efd7c04d71dcc2c6d021a83ea461664dd7505bb0220164deb20633ab26bd27c7979bbebfec9b9bf2b1e6c6a2b57d033a4fcd2cd448f0121037ad97e1e8eac52718b96152fdc62a0a1ce8a03ffb5e6f6e31267beff04331b84ffffffff0200e1f5050000000086c10280004c809f63d910f2175f5a29d2ec6c158b4d2b9724fc4e6bf55b53dee9929364478c1e64c0fb5d90bbacbab02dc1b9639908565cac2b6d68c8394997d3e61f4c027d0ed583d4644d4a9a1a79f710d2143d9095c50e6ea2604b841801c6acb76bf9b3ffc71576002e4cdd41f9950ec379dea069170cd048d519a0ae113672427bb8293fc0d45407000000001976a9140cb412217817cb84feebe8ef3c5089182aa593ec88ac00000000"
)

func init() {
   basecoinTx = bchain.Tx{
      Hex:       "010000000105a2b972575b810e15b9ea073afc7ada87444c53096bb6af42bd539e0c352f80000000006b48304502210098552d04e56f9bb17a4ce1504d9c41207966f9d00d551e789a1ff3c16dc21a7202206b2db55365b2c0325226b8c33d389c0537e07c71e636cecab45772a19d1c68100121036e38b81971b79e160f2a405f9673bee9b54a8901827163fcbb56593b6c35ebb6ffffffff012ad8f505000000001976a9141463d364a1afbdb5da5de66efe68e711c29c300d88ac00000000",
      Blocktime: 1541383843,
      Txid:      "a1eae41f54a9bda8bf2d12c0c37f8607e0a0d6075109ea2b58ba047486892ea2",
      LockTime:  0,
      Version: 1,
      Vin: []bchain.Vin{
         {
            ScriptSig: bchain.ScriptSig{
               Hex: "48304502210098552d04e56f9bb17a4ce1504d9c41207966f9d00d551e789a1ff3c16dc21a7202206b2db55365b2c0325226b8c33d389c0537e07c71e636cecab45772a19d1c68100121036e38b81971b79e160f2a405f9673bee9b54a8901827163fcbb56593b6c35ebb6",
            },
            Txid:     "802f350c9e53bd42afb66b09534c4487da7afc3a07eab9150e815b5772b9a205",
            Vout:     0,
            Sequence: 4294967295,
         },
      },
      Vout: []bchain.Vout{
         {
            ValueSat: *big.NewInt(99997738),
            N:        0,
            ScriptPubKey: bchain.ScriptPubKey{
               Hex: "76a9141463d364a1afbdb5da5de66efe68e711c29c300d88ac",
               Addresses: []string{"D6zue7LTtUpEiuowXwpMWKyyrWF6L2Cr6W"},
            },
         },
      },
   }

   zerocoinMintTx = bchain.Tx{
      Hex:       "01000000011f3d790b96a9a2b995c5082beb703e5ebbd7e2981e60a285bed04e3a8da8027b000000006a47304402204e285305d4b93a4ba50b1f4b8efd7c04d71dcc2c6d021a83ea461664dd7505bb0220164deb20633ab26bd27c7979bbebfec9b9bf2b1e6c6a2b57d033a4fcd2cd448f0121037ad97e1e8eac52718b96152fdc62a0a1ce8a03ffb5e6f6e31267beff04331b84ffffffff0200e1f5050000000086c10280004c809f63d910f2175f5a29d2ec6c158b4d2b9724fc4e6bf55b53dee9929364478c1e64c0fb5d90bbacbab02dc1b9639908565cac2b6d68c8394997d3e61f4c027d0ed583d4644d4a9a1a79f710d2143d9095c50e6ea2604b841801c6acb76bf9b3ffc71576002e4cdd41f9950ec379dea069170cd048d519a0ae113672427bb8293fc0d45407000000001976a9140cb412217817cb84feebe8ef3c5089182aa593ec88ac00000000",
      Blocktime: 1541382363,
      Txid:      "1d415a15ad97f0af38d8493b70e7906f348bb55e495d9f82262b607b045ffa4b",
      LockTime:  0,
      Version: 1,
      Vin: []bchain.Vin{
         {
            ScriptSig: bchain.ScriptSig{
               Hex: "47304402204e285305d4b93a4ba50b1f4b8efd7c04d71dcc2c6d021a83ea461664dd7505bb0220164deb20633ab26bd27c7979bbebfec9b9bf2b1e6c6a2b57d033a4fcd2cd448f0121037ad97e1e8eac52718b96152fdc62a0a1ce8a03ffb5e6f6e31267beff04331b84",
            },
            Txid:     "7b02a88d3a4ed0be85a2601e98e2d7bb5e3e70eb2b08c595b9a2a9960b793d1f",
            Vout:     0,
            Sequence: 4294967295,
         },
      },
      Vout: []bchain.Vout{
         {
            ValueSat: *big.NewInt(100000000),
            N:        0,
            ScriptPubKey: bchain.ScriptPubKey{
               Hex: "c10280004c809f63d910f2175f5a29d2ec6c158b4d2b9724fc4e6bf55b53dee9929364478c1e64c0fb5d90bbacbab02dc1b9639908565cac2b6d68c8394997d3e61f4c027d0ed583d4644d4a9a1a79f710d2143d9095c50e6ea2604b841801c6acb76bf9b3ffc71576002e4cdd41f9950ec379dea069170cd048d519a0ae113672427bb8293f",
               Addresses: []string{ZPIV_LABEL},
            },
         },
         {
            ValueSat: *big.NewInt(123000000),
            N:        1,
            ScriptPubKey: bchain.ScriptPubKey{
               Hex: "76a9140cb412217817cb84feebe8ef3c5089182aa593ec88ac",
               Addresses: []string{"D6JGN6nUgE9UcvFwPv9oUw5stGeNj2NTJc"},
            },
         },
      },
   }
}

func TestGetAddrDesc(t *testing.T) {
   type args struct {
      tx     bchain.Tx
      parser *PivxParser
   }
   tests := []struct {
      name string
      args args
   }{
      {
         name: "pivx-1",
         args: args{
            tx:     basecoinTx,
            parser: NewPivxParser(GetChainParams("main"), &btc.Configuration{}),
         },
      },
      {
         name: "pivx-2",
         args: args{
            tx:     zerocoinMintTx,
            parser: NewPivxParser(GetChainParams("main"), &btc.Configuration{}),
         },
      },
   }
   for _, tt := range tests {
      t.Run(tt.name, func(t *testing.T) {
         for n, vout := range tt.args.tx.Vout {
            got1, err := tt.args.parser.GetAddrDescFromVout(&vout)
            if err != nil {
               t.Errorf("getAddrDescFromVout() error = %v, vout = %d", err, n)
               return
            }
            got2, err := tt.args.parser.GetAddrDescFromAddress("")
            if len(vout.ScriptPubKey.Addresses) > 0 {
               got2, err = tt.args.parser.GetAddrDescFromAddress(vout.ScriptPubKey.Addresses[0])
            }

            if err != nil {
               t.Errorf("getAddrDescFromAddress() error = %v, vout = %d", err, n)
               return
            }
            if !bytes.Equal(got1, got2) {
               t.Errorf("Address descriptors mismatch: got1 = %v, got2 = %v", got1, got2)
            }
         }
      })
   }
}

func TestPackTx(t *testing.T) {
   type args struct {
      tx        bchain.Tx
      height    uint32
      blockTime int64
      parser    *PivxParser
   }
   tests := []struct {
      name    string
      args    args
      want    string
      wantErr bool
   }{
      {
         name: "pivx-1",
         args: args{
            tx:        basecoinTx,
            height:    1443756,
            blockTime: 1541383843,
            parser:    NewPivxParser(GetChainParams("main"), &btc.Configuration{}),
         },
         want:    basecoinTxPacked,
         wantErr: false,
      },
      {
         name: "pivx-2",
         args: args{
            tx:        zerocoinMintTx,
            height:    1443724,
            blockTime: 1541382363,
            parser:    NewPivxParser(GetChainParams("main"), &btc.Configuration{}),
         },
         want:    zerocoinMintTxPacked,
         wantErr: false,
      },
   }
   for _, tt := range tests {
      t.Run(tt.name, func(t *testing.T) {
         got, err := tt.args.parser.PackTx(&tt.args.tx, tt.args.height, tt.args.blockTime)
         if (err != nil) != tt.wantErr {
            t.Errorf("packTx() error = %v, wantErr %v", err, tt.wantErr)
            return
         }
         h := hex.EncodeToString(got)
         if !reflect.DeepEqual(h, tt.want) {
            t.Errorf("packTx() = %v, want %v", h, tt.want)
         }
      })
   }
}

func TestUnpackTx(t *testing.T) {
   type args struct {
      packedTx string
      parser   *PivxParser
   }
   tests := []struct {
      name    string
      args    args
      want    *bchain.Tx
      want1   uint32
      wantErr bool
   }{
      {
         name: "pivx-1",
         args: args{
            packedTx: basecoinTxPacked,
            parser:   NewPivxParser(GetChainParams("main"), &btc.Configuration{}),
         },
         want:    &basecoinTx,
         want1:   1443756,
         wantErr: false,
      },
      {
         name: "pivx-2",
         args: args{
            packedTx: zerocoinMintTxPacked,
            parser:   NewPivxParser(GetChainParams("main"), &btc.Configuration{}),
         },
         want:    &zerocoinMintTx,
         want1:   1443724,
         wantErr: false,
      },
   }
   for _, tt := range tests {
      t.Run(tt.name, func(t *testing.T) {
         b, _ := hex.DecodeString(tt.args.packedTx)
         got, got1, err := tt.args.parser.UnpackTx(b)
         if (err != nil) != tt.wantErr {
            t.Errorf("unpackTx() error = %v, wantErr %v", err, tt.wantErr)
            return
         }
         if !reflect.DeepEqual(got, tt.want) {
            t.Errorf("unpackTx() got = %v, want %v", got, tt.want)
         }
         if got1 != tt.want1 {
            t.Errorf("unpackTx() got1 = %v, want %v", got1, tt.want1)
         }
      })
   }
}
