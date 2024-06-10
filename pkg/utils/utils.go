package utils

import (
	"crypto/sha256"
	"encoding/hex"
)

const (
	prefix                = "elasti-"
	privateServicePostfix = "-pvt"
	endpointSlicePostfix  = "-endpointslice-to-resolver"
)

// GetPrivateSerivceName returns a private service name for a given public service name
// This generates a hash of the public service name and appends it to the private service name
// This way it decrease the chances of user having a same name, however, to be noted, the has will always be the same
// if the public service name is same
func GetPrivateSerivceName(publicSVCName string) string {
	hash := sha256.New()
	hash.Write([]byte(publicSVCName))
	hashed := hex.EncodeToString(hash.Sum(nil))
	pvtName := prefix + publicSVCName + privateServicePostfix + "-" + string(hashed)[:10]
	return pvtName
}

// GetEndpointSliceToResolverName returns an endpoint slice name for a given service name
func GetEndpointSliceToResolverName(serviceName string) string {
	hash := sha256.New()
	hash.Write([]byte(serviceName))
	hashed := hex.EncodeToString(hash.Sum(nil))
	return prefix + serviceName + endpointSlicePostfix + "-" + string(hashed)[:10]
}
