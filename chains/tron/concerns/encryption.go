package concerns

import (
	"encoding/hex"

	"github.com/blocktree/go-owcdrivers/addressEncoder"
)

func DecodeAddress(address string) (string, error) {
	codeType := addressEncoder.TRON_mainnetAddress

	toAddressBytes, err := addressEncoder.AddressDecode(address, codeType)
	if err != nil {
		return "", err
	}
	toAddressBytes = append(codeType.Prefix, toAddressBytes...)

	return hex.EncodeToString(toAddressBytes), nil
}

func EncodeAddress(hexStr string) (string, error) {
	codeType := addressEncoder.TRON_mainnetAddress

	b, err := hex.DecodeString(hexStr)
	if err != nil {
		return "", err
	}
	if len(b) > 20 {
		b = b[1:]
	}

	return addressEncoder.AddressEncode(b, codeType), nil
}

func DecodeHex(hexStr string) (string, error) {
	d, err := hex.DecodeString(hexStr)
	if err != nil {
		return "", err
	}

	return string(d), nil
}
