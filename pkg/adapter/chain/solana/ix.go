package solana

import (
	"crypto/sha256"
	"encoding/binary"

	ag "github.com/gagliardetto/solana-go"
)

// Layout 1:1 com programs/agrobench (INTERFACE.md + src/lib.rs).
//
// initialize:
//   0 authority (signer, writable)  — treasury
//   1 pool     (writable)           — PDA ["pool"]
//   2 usdc_mint
//   3 pool_ata (writable)           — ATA(pool, mint)
//   4 system_program
//   5 token_program
//   6 associated_token_program
//   7 rent
//
// lock_stake:
//   0 producer      (signer)
//   1 authority     (signer, writable)  — fee payer
//   2 stake         (writable)          — PDA ["stake", producer]
//   3 producer_ata  (writable)
//   4 stake_ata     (writable)          — ATA(stake, mint)
//   5 usdc_mint
//   6 token_program
//   7 associated_token_program
//   8 system_program
//
// release_stake:
//   0 producer      (signer)
//   1 authority     (signer)
//   2 stake         (writable)
//   3 producer_ata  (writable)
//   4 stake_ata     (writable)
//   5 token_program
//
// credit_pool:
//   0 authority (signer)
//   1 pool
//   2 from_ata  (writable)  — ATA da treasury
//   3 pool_ata  (writable)
//   4 token_program
//
// distribute:
//   0 authority (signer)
//   1 pool
//   2 pool_ata  (writable)
//   3 token_program
//   remaining: dest ATAs (writable), 1:1 com amounts

const (
	ProgramIDDevnet = "EytN8UaXrfTQc6Pq4AdQbQyJwUX37ddXsV7URayBBLrN"
	UsdcMintDevnet  = "4zMMC9srt5Ri5X14GAgXhaHii3GnPAEERYPJgZJDncDU"

	ixInitialize   = "initialize"
	ixLockStake    = "lock_stake"
	ixReleaseStake = "release_stake"
	ixCreditPool   = "credit_pool"
	ixDistribute   = "distribute"

	seedPool  = "pool"
	seedStake = "stake"
)

func anchorDiscriminator(name string) []byte {
	sum := sha256.Sum256([]byte("global:" + name))
	return sum[:8]
}

func poolPDA(program ag.PublicKey) (ag.PublicKey, uint8, error) {
	return ag.FindProgramAddress([][]byte{[]byte(seedPool)}, program)
}

func stakePDA(program, producer ag.PublicKey) (ag.PublicKey, uint8, error) {
	return ag.FindProgramAddress([][]byte{[]byte(seedStake), producer.Bytes()}, program)
}

func ataOf(owner, mint ag.PublicKey) (ag.PublicKey, uint8, error) {
	return ag.FindAssociatedTokenAddress(owner, mint)
}

func encodeU64(v uint64) []byte {
	b := make([]byte, 8)
	binary.LittleEndian.PutUint64(b, v)
	return b
}

func encodeVecU64(amounts []uint64) []byte {
	b := make([]byte, 4+8*len(amounts))
	binary.LittleEndian.PutUint32(b[:4], uint32(len(amounts)))
	for i, a := range amounts {
		binary.LittleEndian.PutUint64(b[4+i*8:], a)
	}
	return b
}

func ixData(name string, payload []byte) []byte {
	d := anchorDiscriminator(name)
	if len(payload) == 0 {
		return d
	}
	return append(append([]byte{}, d...), payload...)
}

type builtIx struct {
	Program  ag.PublicKey
	Accounts []*ag.AccountMeta
	Data     []byte
}

func (b builtIx) Instruction() ag.Instruction {
	return ag.NewInstruction(b.Program, b.Accounts, b.Data)
}

func buildInitializeIx(program, authority, mint ag.PublicKey) (builtIx, error) {
	pool, _, err := poolPDA(program)
	if err != nil {
		return builtIx{}, err
	}
	poolATA, _, err := ataOf(pool, mint)
	if err != nil {
		return builtIx{}, err
	}
	return builtIx{
		Program: program,
		Accounts: []*ag.AccountMeta{
			ag.Meta(authority).WRITE().SIGNER(),
			ag.Meta(pool).WRITE(),
			ag.Meta(mint),
			ag.Meta(poolATA).WRITE(),
			ag.Meta(ag.SystemProgramID),
			ag.Meta(ag.TokenProgramID),
			ag.Meta(ag.SPLAssociatedTokenAccountProgramID),
			ag.Meta(ag.SysVarRentPubkey),
		},
		Data: ixData(ixInitialize, nil),
	}, nil
}

func buildLockStakeIx(program, producer, authority, mint ag.PublicKey, amount uint64) (builtIx, error) {
	stake, _, err := stakePDA(program, producer)
	if err != nil {
		return builtIx{}, err
	}
	producerATA, _, err := ataOf(producer, mint)
	if err != nil {
		return builtIx{}, err
	}
	stakeATA, _, err := ataOf(stake, mint)
	if err != nil {
		return builtIx{}, err
	}
	return builtIx{
		Program: program,
		Accounts: []*ag.AccountMeta{
			ag.Meta(producer).SIGNER(),
			ag.Meta(authority).WRITE().SIGNER(),
			ag.Meta(stake).WRITE(),
			ag.Meta(producerATA).WRITE(),
			ag.Meta(stakeATA).WRITE(),
			ag.Meta(mint),
			ag.Meta(ag.TokenProgramID),
			ag.Meta(ag.SPLAssociatedTokenAccountProgramID),
			ag.Meta(ag.SystemProgramID),
		},
		Data: ixData(ixLockStake, encodeU64(amount)),
	}, nil
}

func buildReleaseStakeIx(program, producer, authority, mint ag.PublicKey) (builtIx, error) {
	stake, _, err := stakePDA(program, producer)
	if err != nil {
		return builtIx{}, err
	}
	producerATA, _, err := ataOf(producer, mint)
	if err != nil {
		return builtIx{}, err
	}
	stakeATA, _, err := ataOf(stake, mint)
	if err != nil {
		return builtIx{}, err
	}
	return builtIx{
		Program: program,
		Accounts: []*ag.AccountMeta{
			ag.Meta(producer).SIGNER(),
			ag.Meta(authority).SIGNER(),
			ag.Meta(stake).WRITE(),
			ag.Meta(producerATA).WRITE(),
			ag.Meta(stakeATA).WRITE(),
			ag.Meta(ag.TokenProgramID),
		},
		Data: ixData(ixReleaseStake, nil),
	}, nil
}

func buildCreditPoolIx(program, authority, mint ag.PublicKey, amount uint64) (builtIx, error) {
	pool, _, err := poolPDA(program)
	if err != nil {
		return builtIx{}, err
	}
	fromATA, _, err := ataOf(authority, mint)
	if err != nil {
		return builtIx{}, err
	}
	poolATA, _, err := ataOf(pool, mint)
	if err != nil {
		return builtIx{}, err
	}
	return builtIx{
		Program: program,
		Accounts: []*ag.AccountMeta{
			ag.Meta(authority).SIGNER(),
			ag.Meta(pool),
			ag.Meta(fromATA).WRITE(),
			ag.Meta(poolATA).WRITE(),
			ag.Meta(ag.TokenProgramID),
		},
		Data: ixData(ixCreditPool, encodeU64(amount)),
	}, nil
}

func buildDistributeIx(program, authority, mint ag.PublicKey, destATAs []ag.PublicKey, amounts []uint64) (builtIx, error) {
	pool, _, err := poolPDA(program)
	if err != nil {
		return builtIx{}, err
	}
	poolATA, _, err := ataOf(pool, mint)
	if err != nil {
		return builtIx{}, err
	}
	accounts := []*ag.AccountMeta{
		ag.Meta(authority).SIGNER(),
		ag.Meta(pool),
		ag.Meta(poolATA).WRITE(),
		ag.Meta(ag.TokenProgramID),
	}
	for _, dest := range destATAs {
		accounts = append(accounts, ag.Meta(dest).WRITE())
	}
	return builtIx{
		Program:  program,
		Accounts: accounts,
		Data:     ixData(ixDistribute, encodeVecU64(amounts)),
	}, nil
}
