package solana

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"fmt"

	ag "github.com/gagliardetto/solana-go"
	ata "github.com/gagliardetto/solana-go/programs/associated-token-account"
	"github.com/gagliardetto/solana-go/programs/memo"
	"github.com/gagliardetto/solana-go/rpc"

	"github.com/AgroBench/backend/internal/core/domain"
	"github.com/AgroBench/backend/pkg/port"
)

func (c *Chain) hasProgram() bool { return !c.program.IsZero() }

func (c *Chain) ProgramID() string {
	if !c.hasProgram() {
		return ""
	}
	return c.program.String()
}

func (c *Chain) BuildLockStakeTx(ctx context.Context, req port.StakeRequest) (string, error) {
	producer, err := ag.PublicKeyFromBase58(string(req.Wallet))
	if err != nil {
		return "", fmt.Errorf("chain solana: wallet inválida: %w", err)
	}
	var instr ag.Instruction
	if c.hasProgram() {
		ix, err := buildLockStakeIx(c.program, producer, c.treasury.PublicKey(), c.mint, uint64(req.Amount))
		if err != nil {
			return "", err
		}
		instr = ix.Instruction()
	} else {
		instr = memo.NewMemoInstruction([]byte(stakeMemo("lock", req)), producer).Build()
	}
	return c.partialTxBase64(ctx, []ag.Instruction{instr})
}

func (c *Chain) BuildReleaseStakeTx(ctx context.Context, req port.StakeRequest) (string, error) {
	producer, err := ag.PublicKeyFromBase58(string(req.Wallet))
	if err != nil {
		return "", fmt.Errorf("chain solana: wallet inválida: %w", err)
	}
	var instr ag.Instruction
	if c.hasProgram() {
		ix, err := buildReleaseStakeIx(c.program, producer, c.treasury.PublicKey(), c.mint)
		if err != nil {
			return "", err
		}
		instr = ix.Instruction()
	} else {
		instr = memo.NewMemoInstruction([]byte(stakeMemo("release", req)), producer).Build()
	}
	return c.partialTxBase64(ctx, []ag.Instruction{instr})
}

func (c *Chain) SubmitSignedTx(ctx context.Context, txBase64 string) (port.TxRef, error) {
	raw, err := base64.StdEncoding.DecodeString(txBase64)
	if err != nil {
		return "", fmt.Errorf("chain solana: tx não é base64: %w", err)
	}
	tx, err := ag.TransactionFromBytes(raw)
	if err != nil {
		return "", fmt.Errorf("chain solana: tx inválida: %w", err)
	}
	if len(tx.Message.AccountKeys) == 0 || !tx.Message.AccountKeys[0].Equals(c.treasury.PublicKey()) {
		return "", errors.New("chain solana: fee payer deve ser a treasury")
	}
	if err := c.ensureProducerSigned(tx); err != nil {
		return "", err
	}
	if _, err := tx.PartialSign(c.treasurySigner); err != nil {
		return "", fmt.Errorf("chain solana: co-assinando: %w", err)
	}
	if err := requireAllSignatures(tx); err != nil {
		return "", err
	}
	sig, err := c.rpc.SendTransactionWithOpts(ctx, tx, rpc.TransactionOpts{PreflightCommitment: rpc.CommitmentConfirmed})
	if err != nil {
		return "", fmt.Errorf("chain solana: enviando tx: %w", err)
	}
	if err := c.waitConfirmed(ctx, sig); err != nil {
		return "", err
	}
	return port.TxRef(sig.String()), nil
}

func (c *Chain) CreditPool(ctx context.Context, amount domain.MicroUSDC, memoText string) (port.TxRef, error) {
	if amount <= 0 {
		return "", errors.New("chain solana: valor deve ser positivo")
	}
	if !c.hasProgram() {
		return c.TransferUSDC(ctx, port.TransferRequest{
			From: c.Treasury(), To: c.Pool(), Amount: amount, Memo: memoText,
		})
	}
	ix, err := buildCreditPoolIx(c.program, c.treasury.PublicKey(), c.mint, uint64(amount))
	if err != nil {
		return "", err
	}
	return c.sendAndConfirm(ctx, []ag.Instruction{ix.Instruction()})
}

func (c *Chain) DistributePool(ctx context.Context, payouts []port.PoolPayout) (port.TxRef, error) {
	if len(payouts) == 0 {
		return "", errors.New("chain solana: lista de payouts vazia")
	}
	if !c.hasProgram() {
		var last port.TxRef
		for _, p := range payouts {
			ref, err := c.TransferUSDC(ctx, port.TransferRequest{
				From: c.Pool(), To: p.Wallet, Amount: p.Amount, Memo: "payout",
			})
			if err != nil {
				return last, err
			}
			last = ref
		}
		return last, nil
	}
	const chunk = 12
	var last port.TxRef
	for i := 0; i < len(payouts); i += chunk {
		end := i + chunk
		if end > len(payouts) {
			end = len(payouts)
		}
		ref, err := c.distributeChunk(ctx, payouts[i:end])
		if err != nil {
			return last, err
		}
		last = ref
	}
	return last, nil
}

func (c *Chain) InitializeProgram(ctx context.Context) (port.TxRef, error) {
	if !c.hasProgram() {
		return "", errors.New("chain solana: CHAIN_SOLANA_PROGRAM_ID vazio")
	}
	ix, err := buildInitializeIx(c.program, c.treasury.PublicKey(), c.mint)
	if err != nil {
		return "", err
	}
	return c.sendAndConfirm(ctx, []ag.Instruction{ix.Instruction()})
}

func (c *Chain) distributeChunk(ctx context.Context, payouts []port.PoolPayout) (port.TxRef, error) {
	var instrs []ag.Instruction
	dests := make([]ag.PublicKey, 0, len(payouts))
	amounts := make([]uint64, 0, len(payouts))
	payer := c.treasury.PublicKey()
	for _, p := range payouts {
		if p.Amount <= 0 {
			continue
		}
		owner, err := ag.PublicKeyFromBase58(string(p.Wallet))
		if err != nil {
			return "", fmt.Errorf("chain solana: wallet inválida: %w", err)
		}
		dst, _, err := ag.FindAssociatedTokenAddress(owner, c.mint)
		if err != nil {
			return "", err
		}
		exists, err := c.accountExists(ctx, dst)
		if err != nil {
			return "", err
		}
		if !exists {
			instrs = append(instrs, ata.NewCreateInstruction(payer, owner, c.mint).Build())
		}
		dests = append(dests, dst)
		amounts = append(amounts, uint64(p.Amount))
	}
	if len(dests) == 0 {
		return "", errors.New("chain solana: nenhum payout positivo")
	}
	ix, err := buildDistributeIx(c.program, payer, c.mint, dests, amounts)
	if err != nil {
		return "", err
	}
	instrs = append(instrs, ix.Instruction())
	return c.sendAndConfirm(ctx, instrs)
}

func (c *Chain) partialTxBase64(ctx context.Context, instrs []ag.Instruction) (string, error) {
	bh, err := c.rpc.GetLatestBlockhash(ctx, rpc.CommitmentFinalized)
	if err != nil {
		return "", fmt.Errorf("chain solana: blockhash: %w", err)
	}
	tx, err := ag.NewTransaction(instrs, bh.Value.Blockhash, ag.TransactionPayer(c.treasury.PublicKey()))
	if err != nil {
		return "", fmt.Errorf("chain solana: montando tx: %w", err)
	}
	if _, err := tx.PartialSign(c.treasurySigner); err != nil {
		return "", fmt.Errorf("chain solana: assinando treasury: %w", err)
	}
	raw, err := tx.MarshalBinary()
	if err != nil {
		return "", fmt.Errorf("chain solana: serializando tx: %w", err)
	}
	return base64.StdEncoding.EncodeToString(raw), nil
}

func (c *Chain) treasurySigner(key ag.PublicKey) *ag.PrivateKey {
	if key.Equals(c.treasury.PublicKey()) {
		return &c.treasury
	}
	return nil
}

func (c *Chain) ensureProducerSigned(tx *ag.Transaction) error {
	n := int(tx.Message.Header.NumRequiredSignatures)
	if n > len(tx.Message.AccountKeys) {
		n = len(tx.Message.AccountKeys)
	}
	signers := tx.Message.AccountKeys[:n]
	var zero ag.Signature
	foundProducer := false
	for i, key := range signers {
		if key.Equals(c.treasury.PublicKey()) {
			continue
		}
		foundProducer = true
		if i >= len(tx.Signatures) || bytes.Equal(tx.Signatures[i][:], zero[:]) {
			return fmt.Errorf("%w: faltando assinatura do produtor", port.ErrNeedsCoSign)
		}
	}
	if !foundProducer {
		return fmt.Errorf("%w: tx não inclui o produtor como signer", port.ErrNeedsCoSign)
	}
	return nil
}

func requireAllSignatures(tx *ag.Transaction) error {
	var zero ag.Signature
	n := int(tx.Message.Header.NumRequiredSignatures)
	if len(tx.Signatures) < n {
		return fmt.Errorf("%w: assinaturas incompletas", port.ErrNeedsCoSign)
	}
	for i := 0; i < n; i++ {
		if bytes.Equal(tx.Signatures[i][:], zero[:]) {
			return fmt.Errorf("%w: assinatura %d ausente", port.ErrNeedsCoSign, i)
		}
	}
	return nil
}
