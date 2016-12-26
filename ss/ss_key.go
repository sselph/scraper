package ss

import (
	"crypto/aes"
	"crypto/cipher"
	"encoding/base64"
	"encoding/json"
	"errors"
)

const key = "Ym+lFlDfNS/Ud2kRTitrzA54qKBlAru5vD3k6fUqrCc="
const data = "RumO8MHyiHBKM8DVIdehXUJfZUiLNOBJp2S/3ZBynxqW5Xo4MM2Eowjz3kf7OtMkjXwG8myC+0Fk2hxUZjvaSjqi7gT4FYZe92HqrfV8dkGlLuutWpMBlKCuJhAdLeQMou0="

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
