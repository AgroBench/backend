package input

type CreateWallet struct {
	Pubkey        string `json:"pubkey"         validate:"required,min=32,max=64"`
	EncryptedBlob string `json:"encrypted_blob" validate:"required"` // base64
	BlobVersion   int    `json:"blob_version"   validate:"omitempty,gte=1"`
}

type ExportWallet struct {
	Code string `json:"code" validate:"required,len=6,numeric"`
}
