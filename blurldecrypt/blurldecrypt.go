package blurldecrypt

import (
	"crypto/aes"
	"crypto/md5"
	"errors"
	"fmt"
	"io"
	"os"
)

type Envelope struct {
	FirstByte byte
	Nonce     string
	Key       [16]byte
}

func AesDecrypt(key []byte, bytes []byte) ([]byte, error) {
	block, err := aes.NewCipher([]byte(key))
	if err != nil {
		return nil, err
	}
	if len(bytes)%block.BlockSize() != 0 {
		return nil, errors.New("ciphertext is not a multiple of the block size")
	}

	decrypted := make([]byte, len(bytes))
	for i := 0; i < len(bytes); i += block.BlockSize() {
		block.Decrypt(decrypted[i:i+block.BlockSize()], bytes[i:i+block.BlockSize()])
	}

	return decrypted, nil
}

func ParseEV(b []byte) (Envelope, error) {
	var data Envelope

	data.FirstByte = b[0]

	if data.FirstByte != 1 {
		return data, fmt.Errorf("Invalid EV")
	}

	stringLength := b[2]
	if len(b) < 5+int(stringLength)+16 {
		return data, fmt.Errorf("invalid key length")
	}

	data.Nonce = string(b[5 : 5+stringLength])

	// copy the key to the struct
	copy(data.Key[:], b[5+stringLength:5+stringLength+16])

	return data, nil
}

func GetEncryptionKey(filePath, nonce string, encryptedkey []byte) []byte {
	file, err := os.Open(filePath)
	if err != nil {
		fmt.Println("Error opening file:", err)
		return nil
	}
	defer file.Close()

	offset := int64(0)
	for {

		file.Seek(offset, 0)

		var first5Bytes [5]byte

		_, err = file.Read(first5Bytes[:])

		if err == io.EOF {
			break
		}
		if err != nil {
			fmt.Println("Error reading file:", err)
			break
		}

		hash := md5.New()
		hash.Write(first5Bytes[:4])
		hash.Write([]byte(nonce))
		result := hash.Sum(nil)

		if result[0] == first5Bytes[4] {
			// Getting the encryption key for the encryption key after we find the right key
			file.Seek(15, io.SeekCurrent)

			var EncryptionKey [32]byte
			_, err = file.Read(EncryptionKey[:])
			if err != nil && err != io.EOF {
				fmt.Println("Error Getting Encryption Key:", err)
				break
			}

			encryptionkey, err := AesDecrypt(EncryptionKey[:], encryptedkey)

			if err != nil {
				fmt.Println("failed to decrypt encryption key:", err)
			}

			return encryptionkey
		}

		offset += 0x34
	}
	return nil
}
