package models

type SymmetricKeyData struct {
	Key      string
	KeyID    string
	KeyRef   string
	Provider string
}

type AsymmetricKeyData struct {
	PrivateKey string
	PublicKey  string
	KeyID      string
	KeyRef     string
	Provider   string
}
