package input

type Register struct {
	Email    string `json:"email"    validate:"required,email,max=255"`
	Phone    string `json:"phone"    validate:"required,e164"`
	CPF      string `json:"cpf"     validate:"required,min=11,max=14"`
	Password string `json:"password" validate:"required,min=8,max=72"`
}

type Login struct {
	Email    string `json:"email"    validate:"required,email"`
	Password string `json:"password" validate:"required"`
}

type MFAVerify struct {
	MFAToken string `json:"mfa_token" validate:"required"`
	Code     string `json:"code"      validate:"required,len=6,numeric"`
}

type Refresh struct {
	RefreshToken string `json:"refresh_token" validate:"required"`
	Device       string `json:"device"         validate:"omitempty,max=120"`
}

type Logout struct {
	RefreshToken string `json:"refresh_token" validate:"required"`
}

type RecoveryStart struct {
	CPF   string `json:"cpf"   validate:"required,min=11,max=14"`
	Phone string `json:"phone" validate:"required,e164"`
}

type RecoveryConfirm struct {
	CPF         string `json:"cpf"          validate:"required,min=11,max=14"`
	Phone       string `json:"phone"        validate:"required,e164"`
	Code        string `json:"code"         validate:"required,len=6,numeric"`
	NewPassword string `json:"new_password" validate:"required,min=8,max=72"`
}
