package dahua

import (
	"crypto/md5"
	"fmt"
	"strings"
)

// md5Upper returns the uppercase hex MD5 of s.
func md5Upper(s string) string {
	sum := md5.Sum([]byte(s))
	return strings.ToUpper(fmt.Sprintf("%x", sum))
}

// gen1Hash is Dahua's "sofia" 8-character password hash (gen1), used inside the
// DVRIP login hash. It compresses the 16-byte MD5 of the password into 8 chars
// from the set [0-9A-Za-z]. Ported from mcw0's DahuaHashCreator/_compressor.
func gen1Hash(password string) string {
	sum := md5.Sum([]byte(password)) // 16 bytes
	out := make([]byte, 8)
	for j := 0; j < 8; j++ {
		v := (int(sum[2*j]) + int(sum[2*j+1])) % 62
		switch {
		case v < 10:
			v += 48 // '0'-'9'
		case v < 36:
			v += 55 // 'A'-'Z'
		default:
			v += 61 // 'a'-'z'
		}
		out[j] = byte(v)
	}
	return string(out)
}

// gen2Hash is the Dahua gen2 realm+random MD5 challenge:
//
//	UPPER(MD5(user:random:UPPER(MD5(user:realm:pass))))
//
// realm is the full realm string (e.g. "Login to 18038F6DBFE666A3").
func gen2Hash(user, pass, realm, random string) string {
	inner := md5Upper(user + ":" + realm + ":" + pass)
	return md5Upper(user + ":" + random + ":" + inner)
}

// dvripLoginHash assembles the DVRIP login payload:
//
//	username + "&&" + gen2Hash + UPPER(MD5(gen1Hash(password)))
//
// Matches mcw0's dahua_logon(logon='dvrip').
func dvripLoginHash(user, pass, realm, random string) string {
	return user + "&&" + gen2Hash(user, pass, realm, random) + md5Upper(gen1Hash(pass))
}
