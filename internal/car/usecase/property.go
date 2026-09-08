package usecase

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/AgroBench/backend/internal/apperrors"
	"github.com/AgroBench/backend/internal/car/contract"
	"github.com/AgroBench/backend/internal/car/domain"
	"github.com/AgroBench/backend/internal/car/types/input"
	"github.com/AgroBench/backend/internal/car/types/output"
	"github.com/AgroBench/backend/internal/core/auth"
	coredomain "github.com/AgroBench/backend/internal/core/domain"
	walletcontract "github.com/AgroBench/backend/internal/wallet/contract"
	"github.com/AgroBench/backend/pkg/port"
)

type CreateProperty struct {
	props   contract.PropertyRepo
	catalog contract.CatalogRepo
	sicar   port.SicarClient
	chain   port.ChainClient
	wallets walletcontract.WalletRepo
}

func NewCreateProperty(props contract.PropertyRepo, catalog contract.CatalogRepo, sicar port.SicarClient, chain port.ChainClient, wallets walletcontract.WalletRepo) *CreateProperty {
	return &CreateProperty{props: props, catalog: catalog, sicar: sicar, chain: chain, wallets: wallets}
}

func (u *CreateProperty) Execute(ctx context.Context, userID uuid.UUID, in input.CreateProperty) (output.Property, error) {
	const op = "car.CreateProperty"
	if _, err := u.catalog.GetMicroRegion(ctx, in.MicroRegionID); err != nil {
		return output.Property{}, err
	}
	rec, err := u.sicar.Lookup(ctx, in.CAR)
	if err != nil {
		return output.Property{}, apperrors.External(op, 0, err).WithDetail("falha ao consultar SICAR")
	}
	now := time.Now()
	status := domain.CARRejected
	var verified *time.Time
	if rec.Exists && rec.Active {
		status = domain.CARApproved
		verified = &now
	}
	p := domain.Property{
		ID:            coredomain.NewID(),
		UserID:        userID,
		CARHMAC:       auth.HMACIdentifier(in.CAR),
		MicroRegionID: in.MicroRegionID,
		CARStatus:     status,
		VerifiedAt:    verified,
	}
	if err := u.props.Create(ctx, p); err != nil {
		if apperrors.Is(err, apperrors.ErrConflict) {
			return output.Property{}, apperrors.Conflict(op, err).WithDetail("CAR já vinculado")
		}
		return output.Property{}, err
	}
	if u.chain != nil && u.wallets != nil {
		if w, err := u.wallets.GetByUserID(ctx, userID); err == nil {
			_, _ = u.chain.RecordCARVerification(ctx, port.Account(w.Pubkey), status == domain.CARApproved)
		}
	}
	return output.NewProperty(p), nil
}

type GetProperty struct{ props contract.PropertyRepo }

func NewGetProperty(props contract.PropertyRepo) *GetProperty { return &GetProperty{props: props} }

func (u *GetProperty) Execute(ctx context.Context, userID uuid.UUID) (output.Property, error) {
	p, err := u.props.GetByUserID(ctx, userID)
	if err != nil {
		return output.Property{}, err
	}
	return output.NewProperty(p), nil
}

type ListRegions struct{ catalog contract.CatalogRepo }

func NewListRegions(c contract.CatalogRepo) *ListRegions { return &ListRegions{catalog: c} }

func (u *ListRegions) Execute(ctx context.Context) ([]domain.MicroRegion, error) {
	return u.catalog.ListMicroRegions(ctx)
}

type ListCultures struct{ catalog contract.CatalogRepo }

func NewListCultures(c contract.CatalogRepo) *ListCultures { return &ListCultures{catalog: c} }

func (u *ListCultures) Execute(ctx context.Context) ([]domain.Culture, error) {
	return u.catalog.ListCultures(ctx)
}
