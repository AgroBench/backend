package usecase

import (
	"context"

	"github.com/google/uuid"

	"github.com/AgroBench/backend/internal/identity/contract"
	"github.com/AgroBench/backend/internal/identity/types/output"
)

type Me struct {
	users contract.UserReader
}

func NewMe(users contract.UserReader) *Me { return &Me{users: users} }

func (u *Me) Execute(ctx context.Context, userID uuid.UUID) (output.Me, error) {
	user, err := u.users.GetByID(ctx, userID)
	if err != nil {
		return output.Me{}, err
	}
	return output.NewMe(user), nil
}
