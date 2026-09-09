package usecase_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/require"

	"github.com/AgroBench/backend/internal/apperrors"
	cardomain "github.com/AgroBench/backend/internal/car/domain"
	"github.com/AgroBench/backend/internal/contribution/contract"
	cdomain "github.com/AgroBench/backend/internal/contribution/domain"
	"github.com/AgroBench/backend/internal/contribution/types/input"
	"github.com/AgroBench/backend/internal/contribution/usecase"
	coredomain "github.com/AgroBench/backend/internal/core/domain"
	cycledomain "github.com/AgroBench/backend/internal/cycle/domain"
	walletdomain "github.com/AgroBench/backend/internal/wallet/domain"
	"github.com/AgroBench/backend/pkg/port"
)

func init() {
	viper.Set("stake.amount_usdc", 10)
}

func nf() error { return apperrors.NotFound("mem", errors.New("not found")) }

type callLog struct{ steps []string }

func (l *callLog) add(s string) { l.steps = append(l.steps, s) }

type memRepo struct {
	log       *callLog
	contribs  map[uuid.UUID]cdomain.Contribution
	stakes    map[uuid.UUID]cdomain.Stake
	createErr error
	stakeErr  error
}

func newMemRepo(log *callLog) *memRepo {
	return &memRepo{
		log:      log,
		contribs: map[uuid.UUID]cdomain.Contribution{},
		stakes:   map[uuid.UUID]cdomain.Stake{},
	}
}

func (m *memRepo) WithTx(_ context.Context, fn func(contract.Repo) error) error {
	snapC := make(map[uuid.UUID]cdomain.Contribution, len(m.contribs))
	for k, v := range m.contribs {
		snapC[k] = v
	}
	snapS := make(map[uuid.UUID]cdomain.Stake, len(m.stakes))
	for k, v := range m.stakes {
		snapS[k] = v
	}
	if err := fn(m); err != nil {
		m.contribs = snapC
		m.stakes = snapS
		return err
	}
	return nil
}

func (m *memRepo) Create(_ context.Context, c cdomain.Contribution) error {
	m.log.add("create")
	if m.createErr != nil {
		return m.createErr
	}
	m.contribs[c.ID] = c
	return nil
}

func (m *memRepo) CreateStake(_ context.Context, s cdomain.Stake) error {
	m.log.add("create_stake")
	if m.stakeErr != nil {
		return m.stakeErr
	}
	m.stakes[s.ID] = s
	return nil
}

func (m *memRepo) ActiveInCycle(_ context.Context, cycleID, walletID uuid.UUID) (cdomain.Contribution, error) {
	for _, c := range m.contribs {
		if c.CycleID == cycleID && c.WalletID == walletID && c.Status != cdomain.StatusRejected {
			return c, nil
		}
	}
	return cdomain.Contribution{}, nf()
}

func (m *memRepo) HasRejected(_ context.Context, cycleID, walletID uuid.UUID) (bool, error) {
	for _, c := range m.contribs {
		if c.CycleID == cycleID && c.WalletID == walletID && c.Status == cdomain.StatusRejected {
			return true, nil
		}
	}
	return false, nil
}

func (m *memRepo) GetByID(_ context.Context, id uuid.UUID) (cdomain.Contribution, error) {
	c, ok := m.contribs[id]
	if !ok {
		return cdomain.Contribution{}, nf()
	}
	return c, nil
}
func (m *memRepo) ListByWallet(context.Context, uuid.UUID) ([]cdomain.Contribution, error) {
	return nil, nil
}
func (m *memRepo) UpdateReveal(context.Context, uuid.UUID, []byte) error { return nil }
func (m *memRepo) UpdateStatus(context.Context, uuid.UUID, cdomain.Status, string) error {
	return nil
}
func (m *memRepo) GetLockedStakeByWallet(context.Context, uuid.UUID) (cdomain.Stake, error) {
	return cdomain.Stake{}, nf()
}
func (m *memRepo) GetStakeByContribution(_ context.Context, contributionID uuid.UUID) (cdomain.Stake, error) {
	for _, s := range m.stakes {
		if s.ContributionID == contributionID {
			return s, nil
		}
	}
	return cdomain.Stake{}, nf()
}
func (m *memRepo) ReleaseStake(context.Context, uuid.UUID, string) error { return nil }
func (m *memRepo) SaveVerdict(context.Context, cdomain.Verdict, []cdomain.ValidatedMetric) error {
	return nil
}
func (m *memRepo) SaveAttestation(context.Context, cdomain.Attestation) error { return nil }
func (m *memRepo) ConsecutiveAccepted(context.Context, uuid.UUID, uuid.UUID, uuid.UUID) (int, error) {
	return 0, nil
}

type fakeChain struct {
	log       *callLog
	lockErr   error
	lockedIDs map[uuid.UUID]bool
	releases  int
}

func newFakeChain(log *callLog) *fakeChain {
	return &fakeChain{log: log, lockedIDs: map[uuid.UUID]bool{}}
}

func (f *fakeChain) Commit(_ context.Context, _ port.CommitRequest) (port.TxRef, error) {
	f.log.add("commit")
	return "commit-tx", nil
}
func (f *fakeChain) LockStake(_ context.Context, req port.StakeRequest) (port.TxRef, error) {
	f.log.add("lock")
	if f.lockErr != nil {
		return "", f.lockErr
	}
	f.lockedIDs[req.ContributionID] = true
	return "lock-tx", nil
}
func (f *fakeChain) ReleaseStake(_ context.Context, req port.StakeRequest) (port.TxRef, error) {
	f.log.add("release")
	f.releases++
	delete(f.lockedIDs, req.ContributionID)
	return "release-tx", nil
}
func (f *fakeChain) Attest(context.Context, port.AttestRequest) (port.TxRef, error) { return "", nil }
func (f *fakeChain) RecordCARVerification(context.Context, port.Account, bool) (port.TxRef, error) {
	return "", nil
}
func (f *fakeChain) TransferUSDC(context.Context, port.TransferRequest) (port.TxRef, error) {
	return "", nil
}
func (f *fakeChain) Balance(context.Context, port.Account) (coredomain.MicroUSDC, error) {
	return 0, nil
}
func (f *fakeChain) Treasury() port.Account { return "treasury" }
func (f *fakeChain) Pool() port.Account     { return "pool" }
func (f *fakeChain) ProgramID() string      { return "" }
func (f *fakeChain) BuildLockStakeTx(context.Context, port.StakeRequest) (string, error) {
	return "mock-lock-tx", nil
}
func (f *fakeChain) BuildReleaseStakeTx(context.Context, port.StakeRequest) (string, error) {
	return "mock-release-tx", nil
}
func (f *fakeChain) SubmitSignedTx(_ context.Context, _ string) (port.TxRef, error) {
	f.log.add("lock")
	return "lock-tx", nil
}
func (f *fakeChain) CreditPool(context.Context, coredomain.MicroUSDC, string) (port.TxRef, error) {
	return "", nil
}
func (f *fakeChain) DistributePool(context.Context, []port.PoolPayout) (port.TxRef, error) {
	return "", nil
}
func (f *fakeChain) InitializeProgram(context.Context) (port.TxRef, error) { return "", nil }

type stubCycles struct{ c cycledomain.Cycle }

func (s *stubCycles) GetByID(context.Context, uuid.UUID) (cycledomain.Cycle, error) { return s.c, nil }
func (s *stubCycles) Create(context.Context, cycledomain.Cycle) error               { return nil }
func (s *stubCycles) List(context.Context, string, string, string) ([]cycledomain.Cycle, error) {
	return nil, nil
}
func (s *stubCycles) Close(context.Context, uuid.UUID) error          { return nil }
func (s *stubCycles) MarkAggregated(context.Context, uuid.UUID) error { return nil }

type stubProps struct{ p cardomain.Property }

func (s *stubProps) Create(context.Context, cardomain.Property) error { return nil }
func (s *stubProps) GetByUserID(context.Context, uuid.UUID) (cardomain.Property, error) {
	return s.p, nil
}
func (s *stubProps) GetByID(context.Context, uuid.UUID) (cardomain.Property, error) { return s.p, nil }

type stubWallets struct{ w walletdomain.Wallet }

func (s *stubWallets) Create(context.Context, walletdomain.Wallet) error { return nil }
func (s *stubWallets) GetByUserID(context.Context, uuid.UUID) (walletdomain.Wallet, error) {
	return s.w, nil
}
func (s *stubWallets) GetByID(context.Context, uuid.UUID) (walletdomain.Wallet, error) {
	return s.w, nil
}
func (s *stubWallets) ClaimPlaceholder(_ context.Context, _ uuid.UUID, pubkey string, blob []byte, version int) (walletdomain.Wallet, error) {
	s.w.Pubkey = pubkey
	s.w.EncryptedBlob = blob
	s.w.BlobVersion = version
	return s.w, nil
}
func (s *stubWallets) MarkExported(context.Context, uuid.UUID) error { return nil }
func (s *stubWallets) SumRewards(context.Context, uuid.UUID) (coredomain.MicroUSDC, error) {
	return 0, nil
}

type fixture struct {
	userID   uuid.UUID
	walletID uuid.UUID
	in       input.Commit
	repo     *memRepo
	chain    *fakeChain
	log      *callLog
	uc       *usecase.Commit
}

func newFixture() *fixture {
	now := time.Now()
	userID := coredomain.NewID()
	walletID := coredomain.NewID()
	f := &fixture{
		userID:   userID,
		walletID: walletID,
		log:      &callLog{},
		in: input.Commit{
			CycleID:    coredomain.NewID(),
			Level:      "basic",
			Hash:       strings.Repeat("ab", 32),
			PropertyID: coredomain.NewID(),
		},
	}
	f.repo = newMemRepo(f.log)
	f.chain = newFakeChain(f.log)
	f.uc = usecase.NewCommit(
		f.repo,
		&stubCycles{c: cycledomain.Cycle{
			ID: f.in.CycleID, Status: cycledomain.CycleOpen,
			OpensAt: now.Add(-time.Hour), ClosesAt: now.Add(time.Hour),
		}},
		&stubProps{p: cardomain.Property{
			ID: f.in.PropertyID, UserID: userID, CARStatus: cardomain.CARApproved,
		}},
		&stubWallets{w: walletdomain.Wallet{ID: walletID, UserID: userID, Pubkey: "MockWallet111"}},
		f.chain,
	)
	return f
}

func TestCommitAttempt1PersistsContributionBeforeStake(t *testing.T) {
	f := newFixture()
	out, err := f.uc.Execute(context.Background(), f.userID, f.in)
	require.NoError(t, err)
	require.Equal(t, 1, out.Attempt)
	require.Equal(t, string(cdomain.StatusCommitted), out.Status)
	require.Equal(t, []string{"commit", "create", "lock", "create_stake"}, f.log.steps)
	require.Len(t, f.repo.contribs, 1)
	require.Len(t, f.repo.stakes, 1)
	require.Len(t, f.chain.lockedIDs, 1)
}

func TestCommitAttempt1LockStakeFailureRollsBackCreate(t *testing.T) {
	f := newFixture()
	f.chain.lockErr = errors.New("saldo insuficiente")
	_, err := f.uc.Execute(context.Background(), f.userID, f.in)
	require.Error(t, err)
	require.True(t, apperrors.Is(err, apperrors.ErrExternalDependency))
	require.Equal(t, []string{"commit", "create", "lock"}, f.log.steps)
	require.Empty(t, f.repo.contribs)
	require.Empty(t, f.repo.stakes)
	require.Empty(t, f.chain.lockedIDs)
	require.Equal(t, 0, f.chain.releases)
}

func TestCommitAttempt1CreateStakeFailureReleasesLock(t *testing.T) {
	f := newFixture()
	f.repo.stakeErr = errors.New("fk")
	_, err := f.uc.Execute(context.Background(), f.userID, f.in)
	require.Error(t, err)
	require.Equal(t, []string{"commit", "create", "lock", "create_stake", "release"}, f.log.steps)
	require.Empty(t, f.repo.contribs)
	require.Empty(t, f.repo.stakes)
	require.Empty(t, f.chain.lockedIDs)
	require.Equal(t, 1, f.chain.releases)
}

func TestCommitAttempt2SkipsStake(t *testing.T) {
	f := newFixture()
	rejectedID := coredomain.NewID()
	f.repo.contribs[rejectedID] = cdomain.Contribution{
		ID: rejectedID, CycleID: f.in.CycleID, WalletID: f.walletID,
		Status: cdomain.StatusRejected, Attempt: 1,
	}
	out, err := f.uc.Execute(context.Background(), f.userID, f.in)
	require.NoError(t, err)
	require.Equal(t, 2, out.Attempt)
	require.Equal(t, string(cdomain.StatusCommitted), out.Status)
	require.Equal(t, []string{"commit", "create"}, f.log.steps)
	require.Empty(t, f.repo.stakes)
	require.Empty(t, f.chain.lockedIDs)
}

func TestCommitRejectsActiveDuplicate(t *testing.T) {
	f := newFixture()
	out, err := f.uc.Execute(context.Background(), f.userID, f.in)
	require.NoError(t, err)

	_, err = f.uc.Execute(context.Background(), f.userID, f.in)
	require.Error(t, err)
	require.True(t, apperrors.Is(err, apperrors.ErrConflict))
	require.Equal(t, 1, out.Attempt)
}

func TestCommitWithStakeTxSkipsLockStake(t *testing.T) {
	f := newFixture()
	f.in.StakeTx = "already-locked-on-chain"
	out, err := f.uc.Execute(context.Background(), f.userID, f.in)
	require.NoError(t, err)
	require.Equal(t, 1, out.Attempt)
	require.Equal(t, []string{"commit", "create", "create_stake"}, f.log.steps)
	require.Len(t, f.repo.stakes, 1)
	require.Empty(t, f.chain.lockedIDs)
}

func TestCommitWithSignedTxSubmitsThenSkipsLockStake(t *testing.T) {
	f := newFixture()
	f.in.SignedTx = "c2lnbmVkLXR4"
	out, err := f.uc.Execute(context.Background(), f.userID, f.in)
	require.NoError(t, err)
	require.Equal(t, 1, out.Attempt)
	require.Equal(t, []string{"lock", "commit", "create", "create_stake"}, f.log.steps)
	require.Len(t, f.repo.stakes, 1)
}

func TestCommitAttempt1NeedsCoSignWithoutStakeTx(t *testing.T) {
	f := newFixture()
	f.chain.lockErr = port.ErrNeedsCoSign
	_, err := f.uc.Execute(context.Background(), f.userID, f.in)
	require.Error(t, err)
	require.True(t, apperrors.Is(err, apperrors.ErrInvalidInput))
	require.Empty(t, f.repo.contribs)
	require.Empty(t, f.repo.stakes)
}
