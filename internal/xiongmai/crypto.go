// Package xiongmai implements an object-oriented, pure-Go (CGO_ENABLED=0)
// dual-transport protocol suite for Hangzhou Xiongmai Technology (XM / XMEye /
// iCSee / Vitacam / JFTech / Sofia) IP cameras and DVR/HVR/NVR recorders.
//
// Supported transports (via the Transport strategy interface):
//   - SofiaTCPTransport: Binary 20-byte Little-Endian framing over TCP 34567
//     with MD5_8 authentication, multi-packet reassembly, and OPPlayBack (1420/1424).
//   - CGITransport: HTTP /cgi-bin/*.cgi with RSA-1024 PKCS#1 v1.5 + AES-128-CBC
//     ZeroPadding (DTAK) session encryption and RFC 6455 WebSocket HttpPlayBack.
//   - Auto-negotiation in Client: transparently falls back from TCP 34567
//     Ret=205 (ERR_RSA_REQUIRED) to HTTP CGI :80, and from HTTP :80 timeout/404
//     on IPCs to Sofia TCP :34567.
package xiongmai

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/md5"
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/big"
	"net"
	"strconv"
	"strings"
)

// MD5_8 computes the 8-character Sofia/Xiongmai password hash:
//
//	digest = MD5(input)
//	rem[i] = (digest[2*i] + digest[2*i+1]) % 62
//	mapped onto '0'-'9' (0..9), 'A'-'Z' (10..35), 'a'-'z' (36..61).
func MD5_8(input string) string {
	sum := md5.Sum([]byte(input))
	var out [8]byte
	for i := 0; i < 8; i++ {
		rem := (int(sum[2*i]) + int(sum[2*i+1])) % 62
		switch {
		case rem < 10:
			out[i] = byte('0' + rem)
		case rem < 36:
			out[i] = byte('A' + rem - 10)
		default:
			out[i] = byte('a' + rem - 36)
		}
	}
	return string(out[:])
}

// HexMD5Lower returns the 32-character lowercase hex MD5 digest of input.
// Note: Xiongmai's RSA CgiLogin concatenates Salt + HexMD5Lower(password), NOT MD5_8.
func HexMD5Lower(input string) string {
	sum := md5.Sum([]byte(input))
	return hex.EncodeToString(sum[:])
}

// ParseRSAPublicKey parses Xiongmai's GetSalt PublicKey format: "<ModulusHex>,<ExpHex>"
// (typically 1024-bit modulus + "010001" = 65537).
func ParseRSAPublicKey(pubKeyStr string) (*rsa.PublicKey, error) {
	parts := strings.Split(strings.TrimSpace(pubKeyStr), ",")
	if len(parts) != 2 {
		return nil, fmt.Errorf("xiongmai: invalid RSA PublicKey format %q", pubKeyStr)
	}
	modHex := strings.TrimSpace(parts[0])
	expHex := strings.TrimSpace(parts[1])
	if modHex == "" || expHex == "" {
		return nil, fmt.Errorf("xiongmai: empty modulus or exponent in PublicKey")
	}
	n := new(big.Int)
	if _, ok := n.SetString(modHex, 16); !ok || n.Sign() <= 0 {
		return nil, fmt.Errorf("xiongmai: invalid RSA modulus hex")
	}
	eVal, err := strconv.ParseInt(expHex, 16, 32)
	if err != nil || eVal <= 1 {
		return nil, fmt.Errorf("xiongmai: invalid RSA exponent %q: %w", expHex, err)
	}
	return &rsa.PublicKey{N: n, E: int(eVal)}, nil
}

// EncryptRSAPKCS1Hex encrypts plaintext with RSA PKCS#1 v1.5 and returns
// an uppercase hexadecimal string (matching Xiongmai Web/plugin/js/rsa.js).
func EncryptRSAPKCS1Hex(pub *rsa.PublicKey, plaintext []byte) (string, error) {
	if pub == nil {
		return "", fmt.Errorf("xiongmai: nil RSA public key")
	}
	ct, err := rsa.EncryptPKCS1v15(rand.Reader, pub, plaintext)
	if err != nil {
		return "", fmt.Errorf("xiongmai: rsa encrypt: %w", err)
	}
	return strings.ToUpper(hex.EncodeToString(ct)), nil
}

// RandomAESKey16 generates a 16-byte ASCII session key in the range '0'..'Z'
// (ASCII 48..90), matching GetRandomFunc(16) in Web/plugin/function.js.
func RandomAESKey16() (string, error) {
	const span = 90 - 48 + 1 // 43 chars: 48..90
	var raw [16]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", err
	}
	var out [16]byte
	for i, b := range raw {
		out[i] = byte(48 + int(b)%span)
	}
	return string(out[:]), nil
}

// RandomHex96 generates 48 random bytes encoded as 96 lowercase hex chars (VERK).
func RandomHex96() (string, error) {
	var raw [48]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(raw[:]), nil
}

// AESEncryptZeroPadB64 encrypts UTF-8 plain bytes with AES-128-CBC using
// 16 zero IV bytes and CryptoJS-compatible ZeroPadding, returning Base64.
func AESEncryptZeroPadB64(plain []byte, key16 string) (string, error) {
	if len(key16) != 16 {
		return "", fmt.Errorf("xiongmai: AES key must be 16 bytes, got %d", len(key16))
	}
	block, err := aes.NewCipher([]byte(key16))
	if err != nil {
		return "", err
	}
	padded := zeroPad16(plain)
	iv := make([]byte, aes.BlockSize) // 16 zero bytes: CryptoJS.enc.Utf8.parse("")
	ct := make([]byte, len(padded))
	cipher.NewCBCEncrypter(block, iv).CryptBlocks(ct, padded)
	return base64.StdEncoding.EncodeToString(ct), nil
}

// AESDecryptZeroPadB64 decrypts a Xiongmai CGI response body.
// Per Web/plugin/class.js:223, very large JSON payloads (e.g. 64-ch Record
// tables) may be returned as unencrypted JSON even when DataEncryptionType=="AES".
// We detect plain JSON objects first before Base64+AES-128-CBC decryption.
func AESDecryptZeroPadB64(body []byte, key16 string) ([]byte, error) {
	trimmed := bytes.TrimSpace(body)
	if len(trimmed) == 0 {
		return nil, fmt.Errorf("xiongmai: empty response body")
	}
	if trimmed[0] == '{' {
		var probe map[string]any
		if json.Unmarshal(trimmed, &probe) == nil {
			if _, hasRet := probe["Ret"]; hasRet {
				return trimmed, nil
			}
			if _, hasName := probe["Name"]; hasName {
				return trimmed, nil
			}
		}
	}
	if key16 == "" {
		return bytes.TrimRight(trimmed, "\x00\r\n "), nil
	}
	if len(key16) != 16 {
		return nil, fmt.Errorf("xiongmai: AES key must be 16 bytes, got %d", len(key16))
	}
	raw, err := base64.StdEncoding.DecodeString(string(trimmed))
	if err != nil {
		// Retry stripping whitespace/newlines inside base64
		clean := strings.Map(func(r rune) rune {
			if r == '\r' || r == '\n' || r == ' ' || r == '\t' {
				return -1
			}
			return r
		}, string(trimmed))
		raw, err = base64.StdEncoding.DecodeString(clean)
		if err != nil {
			return nil, fmt.Errorf("xiongmai: base64 decode response: %w", err)
		}
	}
	if len(raw) == 0 || len(raw)%aes.BlockSize != 0 {
		return nil, fmt.Errorf("xiongmai: cipher length %d is not a multiple of 16", len(raw))
	}
	block, err := aes.NewCipher([]byte(key16))
	if err != nil {
		return nil, err
	}
	iv := make([]byte, aes.BlockSize)
	pt := make([]byte, len(raw))
	cipher.NewCBCDecrypter(block, iv).CryptBlocks(pt, raw)
	return bytes.TrimRight(pt, "\x00\r\n "), nil
}

func zeroPad16(in []byte) []byte {
	if len(in) == 0 {
		return make([]byte, aes.BlockSize)
	}
	rem := len(in) % aes.BlockSize
	if rem == 0 {
		out := make([]byte, len(in))
		copy(out, in)
		return out
	}
	out := make([]byte, len(in)+(aes.BlockSize-rem))
	copy(out, in)
	return out
}

// HexIPToDecIP converts Xiongmai's Little-Endian hex IPv4 ("0x8A4DA8C0") to
// dotted-quad ("192.168.77.138").
func HexIPToDecIP(hexStr string) (string, error) {
	s := strings.TrimSpace(hexStr)
	s = strings.TrimPrefix(strings.TrimPrefix(s, "0x"), "0X")
	if len(s) == 0 || len(s) > 8 {
		return "", fmt.Errorf("xiongmai: invalid hex IP %q", hexStr)
	}
	v, err := strconv.ParseUint(s, 16, 32)
	if err != nil {
		return "", fmt.Errorf("xiongmai: parse hex IP %q: %w", hexStr, err)
	}
	var b [4]byte
	binary.LittleEndian.PutUint32(b[:], uint32(v))
	return net.IPv4(b[0], b[1], b[2], b[3]).String(), nil
}

// DecIPToHexIP converts dotted-quad IPv4 ("192.168.77.138") into Xiongmai's
// Little-Endian hex notation ("0x8A4DA8C0").
func DecIPToHexIP(ipStr string) (string, error) {
	ip := net.ParseIP(strings.TrimSpace(ipStr)).To4()
	if ip == nil {
		return "", fmt.Errorf("xiongmai: invalid IPv4 address %q", ipStr)
	}
	v := binary.LittleEndian.Uint32(ip)
	return fmt.Sprintf("0x%08X", v), nil
}
