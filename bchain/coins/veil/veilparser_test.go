// +build unittest

package veil

import (
	"blockbook/bchain"
   "blockbook/bchain/coins/btc"
   "bytes"
	"encoding/hex"
   "fmt"
   "io/ioutil"
	"math/big"
	"os"
   "path/filepath"
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
			args:    args{address: "mmiS4Yy8mJv7XLGUH3qCbwdMkaihxW5mqL"},
			want:    "76a91443fc82598c0fda340f40921f17a92e1b5ee98af688ac",
			wantErr: false,
		},
	}
	parser := NewVeilParser(GetChainParams("test"), &btc.Configuration{})

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
			args:    args{vout: bchain.Vout{ScriptPubKey: bchain.ScriptPubKey{Hex: "76a91443fc82598c0fda340f40921f17a92e1b5ee98af688ac"}}},
			want:    "76a91443fc82598c0fda340f40921f17a92e1b5ee98af688ac",
			wantErr: false,
		},
	}
	parser := NewVeilParser(GetChainParams("test"), &btc.Configuration{})

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
			args:    args{script: "76a91443fc82598c0fda340f40921f17a92e1b5ee98af688ac"},
			want:    []string{"mmiS4Yy8mJv7XLGUH3qCbwdMkaihxW5mqL"},
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
	}

	parser := NewVeilParser(GetChainParams("test"), &btc.Configuration{})

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
	testTx bchain.Tx

	testTxPacked = "0007c91a8bbf9e8e0802000000000000010000000000000000000000000000000000000000000000000000000000000000ffffffff0602c800022010ffffffff010100f2052a010000002321024870d405ffe8e8a290aa480499aef7e4d1e3481c6ddddf721230ddaa5c367097ac"
)

func init() {

	testTx = bchain.Tx{
		Hex:       "02000000000000010000000000000000000000000000000000000000000000000000000000000000ffffffff0602c800022010ffffffff010100f2052a010000002321024870d405ffe8e8a290aa480499aef7e4d1e3481c6ddddf721230ddaa5c367097ac",
		Blocktime: 1542701956,
		Txid:      "fb18c4c91d47e7dcda657e41f54ac349c7f33d246385d8591b2f4d3117a615d4",
		LockTime:  0,
		Version:   2,
		Vin: []bchain.Vin{
			{
            Coinbase: "02c800022010",
            Sequence: 4294967295,
			},
		},
		Vout: []bchain.Vout{
			{
				ValueSat: *big.NewInt(5000000000),
				N:        0,
				ScriptPubKey: bchain.ScriptPubKey{
					Hex: "21024870d405ffe8e8a290aa480499aef7e4d1e3481c6ddddf721230ddaa5c367097ac",
					Addresses: []string{
						"n3QLkstMZYjWxqQnhZT8NV26TuTjm4Hz2W",
					},
				},
			},
		},
	}
}

func Test_PackTx(t *testing.T) {
	type args struct {
		tx        bchain.Tx
		height    uint32
		blockTime int64
		parser    *VeilParser
	}
	tests := []struct {
		name    string
		args    args
		want    string
		wantErr bool
	}{
		{
			name: "testnet-1",
			args: args{
				tx:        testTx,
				height:    510234,
				blockTime: 1542701956,
				parser:    NewVeilParser(GetChainParams("test"), &btc.Configuration{}),
			},
			want:    testTxPacked,
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

func Test_UnpackTx(t *testing.T) {
	type args struct {
		packedTx string
		parser   *VeilParser
	}
	tests := []struct {
		name    string
		args    args
		want    *bchain.Tx
		want1   uint32
		wantErr bool
	}{
		{
			name: "testnet-1",
			args: args{
				packedTx: testTxPacked,
				parser:   NewVeilParser(GetChainParams("test"), &btc.Configuration{}),
			},
			want:    &testTx,
			want1:   510234,
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

type testBlock struct {
	size int
	time int64
	txs  []string
}

var testParseBlockTxs = map[int]testBlock{
	200: testBlock{
		size: 407,
		time: 1542701956,
		txs: []string{
			"fb18c4c91d47e7dcda657e41f54ac349c7f33d246385d8591b2f4d3117a615d4",
		},
	},
}

func helperLoadBlock(t *testing.T, height int) []byte {
	name := fmt.Sprintf("block_dump.%d", height)
	path := filepath.Join("testdata", name)

	d, err := ioutil.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	d = bytes.TrimSpace(d)

	b := make([]byte, hex.DecodedLen(len(d)))
	_, err = hex.Decode(b, d)
	if err != nil {
		t.Fatal(err)
	}

	return b
}

func TestParseBlock(t *testing.T) {
	p := NewVeilParser(GetChainParams("test"), &btc.Configuration{})

	for height, tb := range testParseBlockTxs {
		b := helperLoadBlock(t, height)

		blk, err := p.ParseBlock(b)
		if err != nil {
			t.Fatal(err)
		}

		if blk.Size != tb.size {
			t.Errorf("ParseBlock() block size: got %d, want %d", blk.Size, tb.size)
		}

		if blk.Time != tb.time {
			t.Errorf("ParseBlock() block time: got %d, want %d", blk.Time, tb.time)
		}

		if len(blk.Txs) != len(tb.txs) {
			t.Errorf("ParseBlock() number of transactions: got %d, want %d", len(blk.Txs), len(tb.txs))
		}

		for ti, tx := range tb.txs {
			if blk.Txs[ti].Txid != tx {
				t.Errorf("ParseBlock() transaction %d: got %s, want %s", ti, blk.Txs[ti].Txid, tx)
			}
		}
	}
}
