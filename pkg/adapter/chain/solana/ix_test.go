package solana

import (
	"crypto/sha256"
	"encoding/binary"
	"testing"

	ag "github.com/gagliardetto/solana-go"
	"github.com/stretchr/testify/require"

	"github.com/AgroBench/backend/internal/core/domain"
)

func testKeys(t *testing.T) (program, producer, authority, mint ag.PublicKey) {
	t.Helper()
	pk := func() ag.PublicKey {
		k, err := ag.NewRandomPrivateKey()
		require.NoError(t, err)
		return k.PublicKey()
	}
	return pk(), pk(), pk(), ag.MustPublicKeyFromBase58(UsdcMintDevnet)
}

func TestAnchorDiscriminatorMatchesINTERFACE(t *testing.T) {
	cases := map[string][]byte{
		"initialize":    {0xaf, 0xaf, 0x6d, 0x1f, 0x0d, 0x98, 0x9b, 0xed},
		"lock_stake":    {0x6f, 0xba, 0xaf, 0xe4, 0x31, 0xa5, 0x1b, 0xf8},
		"release_stake": {0x33, 0x05, 0x1c, 0xfa, 0xb9, 0xa8, 0x12, 0x35},
		"credit_pool":   {0x2c, 0x51, 0x1f, 0xf9, 0x7b, 0x2f, 0xb0, 0xfe},
		"distribute":    {0xbf, 0x2c, 0xdf, 0xcf, 0xa4, 0xec, 0x7e, 0x3d},
	}
	for name, want := range cases {
		sum := sha256.Sum256([]byte("global:" + name))
		require.Equal(t, want, sum[:8], name)
		require.Equal(t, want, anchorDiscriminator(name), name)
	}
}

func TestPoolAndStakePDAs(t *testing.T) {
	program, producer, _, _ := testKeys(t)
	pool, bump, err := poolPDA(program)
	require.NoError(t, err)
	require.False(t, pool.IsZero())
	require.LessOrEqual(t, bump, uint8(255))

	again, _, err := poolPDA(program)
	require.NoError(t, err)
	require.Equal(t, pool, again)

	stake, _, err := stakePDA(program, producer)
	require.NoError(t, err)
	require.NotEqual(t, pool, stake)
}

func TestLockStakeIxAccountsAndAmount(t *testing.T) {
	program, producer, authority, mint := testKeys(t)
	amount := uint64(domain.USDC(10))
	require.Equal(t, uint64(10_000_000), amount)

	ix, err := buildLockStakeIx(program, producer, authority, mint, amount)
	require.NoError(t, err)
	require.Equal(t, program, ix.Program)
	require.Len(t, ix.Accounts, 9)

	require.True(t, ix.Accounts[0].PublicKey.Equals(producer))
	require.True(t, ix.Accounts[0].IsSigner)
	require.False(t, ix.Accounts[0].IsWritable)

	require.True(t, ix.Accounts[1].PublicKey.Equals(authority))
	require.True(t, ix.Accounts[1].IsSigner)
	require.True(t, ix.Accounts[1].IsWritable)

	stake, _, err := stakePDA(program, producer)
	require.NoError(t, err)
	require.True(t, ix.Accounts[2].PublicKey.Equals(stake))
	require.True(t, ix.Accounts[2].IsWritable)
	require.False(t, ix.Accounts[2].IsSigner)

	prodATA, _, err := ataOf(producer, mint)
	require.NoError(t, err)
	require.True(t, ix.Accounts[3].PublicKey.Equals(prodATA))
	require.True(t, ix.Accounts[3].IsWritable)

	stakeATA, _, err := ataOf(stake, mint)
	require.NoError(t, err)
	require.True(t, ix.Accounts[4].PublicKey.Equals(stakeATA))
	require.True(t, ix.Accounts[4].IsWritable)

	require.True(t, ix.Accounts[5].PublicKey.Equals(mint))
	require.True(t, ix.Accounts[6].PublicKey.Equals(ag.TokenProgramID))
	require.True(t, ix.Accounts[7].PublicKey.Equals(ag.SPLAssociatedTokenAccountProgramID))
	require.True(t, ix.Accounts[8].PublicKey.Equals(ag.SystemProgramID))

	require.Equal(t, append(anchorDiscriminator("lock_stake"), encodeU64(amount)...), ix.Data)
	require.Equal(t, amount, binary.LittleEndian.Uint64(ix.Data[8:]))
}

func TestReleaseStakeIxRequiresProducerSigner(t *testing.T) {
	program, producer, authority, mint := testKeys(t)
	ix, err := buildReleaseStakeIx(program, producer, authority, mint)
	require.NoError(t, err)
	require.Len(t, ix.Accounts, 6)
	require.True(t, ix.Accounts[0].PublicKey.Equals(producer))
	require.True(t, ix.Accounts[0].IsSigner)
	require.False(t, ix.Accounts[0].IsWritable)
	require.True(t, ix.Accounts[1].PublicKey.Equals(authority))
	require.True(t, ix.Accounts[1].IsSigner)
	require.False(t, ix.Accounts[1].IsWritable)
	require.Equal(t, anchorDiscriminator("release_stake"), ix.Data)
}

func TestCreditPoolIxAmount(t *testing.T) {
	program, _, authority, mint := testKeys(t)
	ix, err := buildCreditPoolIx(program, authority, mint, 500_000_000)
	require.NoError(t, err)
	require.True(t, ix.Accounts[0].PublicKey.Equals(authority))
	require.True(t, ix.Accounts[0].IsSigner)
	require.False(t, ix.Accounts[0].IsWritable)
	pool, _, err := poolPDA(program)
	require.NoError(t, err)
	require.True(t, ix.Accounts[1].PublicKey.Equals(pool))
	require.False(t, ix.Accounts[1].IsWritable)
	require.Equal(t, uint64(500_000_000), binary.LittleEndian.Uint64(ix.Data[8:]))
}

func TestDistributeIxVecAmountsAndDestATAs(t *testing.T) {
	program, producer, authority, mint := testKeys(t)
	ata1, _, err := ataOf(producer, mint)
	require.NoError(t, err)
	ata2, _, err := ataOf(authority, mint)
	require.NoError(t, err)
	amounts := []uint64{1_000_000, 2_000_000}

	ix, err := buildDistributeIx(program, authority, mint, []ag.PublicKey{ata1, ata2}, amounts)
	require.NoError(t, err)
	require.Len(t, ix.Accounts, 6) // 4 fixas + 2 dest
	require.False(t, ix.Accounts[1].IsWritable)
	require.True(t, ix.Accounts[4].PublicKey.Equals(ata1))
	require.True(t, ix.Accounts[4].IsWritable)
	require.True(t, ix.Accounts[5].PublicKey.Equals(ata2))

	require.Equal(t, encodeVecU64(amounts), ix.Data[8:])
	require.Equal(t, uint32(2), binary.LittleEndian.Uint32(ix.Data[8:12]))
	require.Equal(t, uint64(1_000_000), binary.LittleEndian.Uint64(ix.Data[12:20]))
	require.Equal(t, uint64(2_000_000), binary.LittleEndian.Uint64(ix.Data[20:28]))
}

func TestInitializeIxPoolIsPDA(t *testing.T) {
	program, _, authority, mint := testKeys(t)
	ix, err := buildInitializeIx(program, authority, mint)
	require.NoError(t, err)
	require.Len(t, ix.Accounts, 8)
	pool, _, err := poolPDA(program)
	require.NoError(t, err)
	require.True(t, ix.Accounts[1].PublicKey.Equals(pool))
	require.True(t, ix.Accounts[1].IsWritable)
	poolATA, _, err := ataOf(pool, mint)
	require.NoError(t, err)
	require.True(t, ix.Accounts[3].PublicKey.Equals(poolATA))
	require.True(t, ix.Accounts[7].PublicKey.Equals(ag.SysVarRentPubkey))
	require.Equal(t, anchorDiscriminator("initialize"), ix.Data)
}
