package utils

import (
	"cloudque/pkg/config"
	bizerrors "cloudque/pkg/errors"
	"crypto/aes"
	"crypto/cipher"
	"crypto/md5"
	"encoding/base64"
	"os"
	"strings"
)

func DecryptCryptoJSPassphrase(cipherBase64, passphrase string) (string, error) {
	data, err := base64.StdEncoding.DecodeString(cipherBase64)
	if err != nil {
		return "", bizerrors.NewWithErr(bizerrors.CodeParamFormatError, "Base64解析失败", err)
	}
	if len(data) < 16 || string(data[:8]) != "Salted__" {
		return "", bizerrors.New(bizerrors.CodeParamFormatError, "密文格式无效(缺少Salted__头)")
	}
	salt := data[8:16]
	ciphertext := data[16:]

	key, iv := evpBytesToKey([]byte(passphrase), salt, 32, 16)
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", bizerrors.NewWithErr(bizerrors.CodeInternalError, "创建AES块失败", err)
	}
	if len(ciphertext)%aes.BlockSize != 0 {
		return "", bizerrors.New(bizerrors.CodeParamFormatError, "密文长度非法(非块对齐)")
	}
	plain := make([]byte, len(ciphertext))
	cipher.NewCBCDecrypter(block, iv).CryptBlocks(plain, ciphertext)

	pad := int(plain[len(plain)-1])
	if pad <= 0 || pad > aes.BlockSize || pad > len(plain) {
		return "", bizerrors.New(bizerrors.CodeParamFormatError, "PKCS7填充非法")
	}
	return string(plain[:len(plain)-pad]), nil
}

func evpBytesToKey(passphrase, salt []byte, keyLen, ivLen int) (key, iv []byte) {
	total := keyLen + ivLen
	var d, di []byte
	for len(d) < total {
		h := md5.New()
		h.Write(di)
		h.Write(passphrase)
		h.Write(salt)
		di = h.Sum(nil)
		d = append(d, di...)
	}
	key = d[:keyLen]
	iv = d[keyLen:total]
	return
}

func DecryptIfCryptoJS(s string) string {
	if s == "" {
		return s
	}
	if !strings.HasPrefix(s, "U2FsdGVkX1") {
		return s
	}
	var passphrase string
	if cfg := config.Get(); cfg != nil {
		passphrase = cfg.App.AESSecret
	}
	if passphrase == "" {
		passphrase = os.Getenv("AES_SECRET")
	}
	if passphrase == "" {
		passphrase = os.Getenv("VITE_AES_KEY")
	}
	if passphrase == "" {
		return s
	}
	p, err := DecryptCryptoJSPassphrase(s, passphrase)
	if err != nil {
		return s
	}
	return p
}
