package utils

import (
	"crypto/sha256"
	"encoding/hex"
)

func GetPrivateSerivceName(publicSVCName string) string {
	hash := sha256.New()
	hash.Write([]byte(publicSVCName))
	hashed := hex.EncodeToString(hash.Sum(nil))
	return publicSVCName + "-pvt" + "-" + string(hashed)[:10] + "-" + string(hashed)[11:16]
}
