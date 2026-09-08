package domain

type Role string

const (
	RoleProducer    Role = "producer"
	RoleInstitution Role = "institution"
	RoleAdmin       Role = "admin"
)
