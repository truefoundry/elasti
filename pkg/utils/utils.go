package utils

import (
	"crypto/sha256"
	"encoding/hex"
)

const privateServicePostfix = "-pvt"

// GetPrivateSerivceName returns a private service name for a given public service name
func GetPrivateSerivceName(publicSVCName string) string {
	hash := sha256.New()
	hash.Write([]byte(publicSVCName))
	hashed := hex.EncodeToString(hash.Sum(nil))
	pvtName := publicSVCName + privateServicePostfix + "-" + string(hashed)[:10] + "-" + string(hashed)[11:16]
	return pvtName
}
