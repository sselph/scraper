package ss

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
)

const key = "Ym+lFlDfNS/Ud2kRTitrzA54qKBlAru5vD3k6fUqrCc="
const data = "tyaeVitrQtwVtlZdKN1htjwLYrLgSezPt2edn8o1ayryAQmCHnIK3BKCbGZfMA+5FGbFMrgR1UD1p+vHkuUzgby/oR0kKzHcryJe/2OuzjMsy6K7CYaCCWzFWPny0i8XU3w="

func ObfuscateDevInfo(info DevInfo) (string, error) {
	k, err := base64.StdEncoding.DecodeString(key)
	if err != nil {
		return "", err
	}
	b, err := json.Marshal(info)
	if err != nil {
		return "", err
	}
	block, err := aes.NewCipher(k[:])
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, 12)
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		panic(err.Error())
	}
	ciphertext := gcm.Seal(nil, nonce, b, nil)
	return base64.StdEncoding.EncodeToString(append(nonce, ciphertext...)), nil
}

func DeobfuscateDevInfo() (info DevInfo, err error) {
	out := DevInfo{}
	k, err := base64.StdEncoding.DecodeString(key)
	if err != nil {
		return out, err
	}
	d, err := base64.StdEncoding.DecodeString(data)
	if err != nil {
		return out, err
	}
	block, err := aes.NewCipher(k[:])
	if err != nil {
		return out, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return out, err
	}
	if len(d) < gcm.NonceSize() {
		return out, errors.New("malformed ciphertext")
	}
	p, err := gcm.Open(nil, d[:gcm.NonceSize()], d[gcm.NonceSize():], nil)
	if err != nil {
		return out, err
	}
	err = json.Unmarshal(p, &out)
	return out, err
}
