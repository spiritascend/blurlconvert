package festdecrypt

import (
	"crypto/aes"
	"crypto/cipher"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strings"
)

func parseJsonFromDecryptedBlob(blob string) string {
	startidx := 0
	endidx := 0

	for cidx, c := range blob {
		if startidx == 0 {
			if string(c) == "{" {
				startidx = cidx
			}
		}
		if string(c) == "}" {
			endidx = cidx + 1
		}
	}

	if endidx == 0 || startidx == 0 {
		return blob
	}

	return blob[startidx:endidx]
}

func decryptEnvelope(EVString string, Bearer string) (string, error) {
	Envelope, err := base64.StdEncoding.DecodeString(EVString)
	if err != nil {
		log.Fatal("failed to decode envelope:", err)
	}

	if Envelope[0] != 1 {
		return "", errors.New("envelope header is invalid")
	}

	fourthbyteofEnv := int(Envelope[3])
	fifthbyteofEnv := int(Envelope[4])

	Envelope = Envelope[5:]

	if len(Bearer) >= fourthbyteofEnv {
		subkey := []byte(Bearer[len(Bearer)-fourthbyteofEnv:])

		if len(subkey) == fourthbyteofEnv {
			TrailBytes := []byte(Envelope[len(Envelope)-(0x10-fourthbyteofEnv):])

			finalkey := append(TrailBytes, subkey...)

			startIndex := fourthbyteofEnv + len(Envelope) - 0x10
			endIndex := startIndex + (0x10 - fourthbyteofEnv)

			Envelope = append(Envelope[:startIndex], Envelope[endIndex:]...)

			if (len(Envelope)-fifthbyteofEnv)%16 == 0 {
				encryptedIntArray := Envelope[fifthbyteofEnv:]

				IV := make([]byte, aes.BlockSize)

				block, err := aes.NewCipher(finalkey)
				if err != nil {
					return "", err
				}

				if len(encryptedIntArray)%aes.BlockSize != 0 {
					return "", err
				}

				mode := cipher.NewCBCDecrypter(block, IV)

				mode.CryptBlocks(encryptedIntArray, encryptedIntArray)

				return parseJsonFromDecryptedBlob(string(encryptedIntArray)), nil
			}

		} else {
			return "", errors.New("invalid bearer subkey length")
		}
	}

	return "", nil
}

type CDMJson struct {
	Keys []struct {
		K   string `json:"k"`
		Kid string `json:"kid"`
		Kty string `json:"kty"`
	} `json:"keys"`
}

func addBase64Padding(b64blob []byte) string {
	paddingNeeded := (4 - len(b64blob)%4) % 4

	for i := 0; i < paddingNeeded; i++ {
		b64blob = append(b64blob, '=')
	}

	return string(b64blob)
}

func GetFestEncryptionKey(EVString string, Bearer string) (string, error) {
	var cdmobj CDMJson

	strevobj, err := decryptEnvelope(EVString, Bearer)

	if err != nil {
		return "", err
	}

	err = json.Unmarshal([]byte(strevobj), &cdmobj)

	if err != nil {
		return "", err
	}

	if len(cdmobj.Keys) == 0 {
		return "", errors.New("no keys found in ev blob")
	}

	key := cdmobj.Keys[0].K

	key = strings.ReplaceAll(key, "-", "+")
	key = strings.ReplaceAll(key, "_", "/")

	decodedKey, err := base64.StdEncoding.DecodeString(addBase64Padding([]byte(key)))
	if err != nil {
		return "", fmt.Errorf("failed to decode key %v", err)
	}

	key = hex.EncodeToString(decodedKey)

	return key, nil
}
