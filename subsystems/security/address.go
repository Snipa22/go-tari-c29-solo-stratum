package security

import (
	"errors"
	"fmt"
	"github.com/snipa22/go-tari-c29-solo-stratum/subsystems/config"
	"strings"
)

func ValidateAddress(address string) error {
	if len(address) < 90 || len(address) > 91 {
		return errors.New("address has an invalid length")
	}
	validPrefix := false
	for _, v := range config.AllowedAddressPrefixes {
		if strings.HasPrefix(address, v) {
			validPrefix = true
		}
	}
	if !validPrefix {
		return errors.New(fmt.Sprintf("address has an invalid prefix, currently supported is %v", config.AllowedAddressPrefixes))
	}
	return nil
}
