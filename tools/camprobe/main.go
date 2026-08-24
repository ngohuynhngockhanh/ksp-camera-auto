package main

import (
	"crypto/md5"
	"encoding/binary"
	"fmt"
	"net"
	"strings"
	"time"
)

func gen1Hash(password string) string {
	sum := md5.Sum([]byte(password))
	out := make([]byte, 8)
	for j := 0; j < 8; j++ {
		v := (int(sum[2*j]) + int(sum[2*j+1])) % 62
		if v < 10 {
			out[j] = byte(v + 48)
		} else if v < 36 {
			out[j] = byte(v + 55)
		} else {
			out[j] = byte(v + 61)
		}
	}
	return string(out)
}

func gen2Hash(user, password, realm, random string) string {
	step1 := fmt.Sprintf("%x", md5.Sum([]byte(user+":"+realm+":"+password)))
	step1 = strings.ToUpper(step1)
	step2 := fmt.Sprintf("%x", md5.Sum([]byte(user+":"+random+":"+step1)))
	return strings.ToUpper(step2)
}

func dvripLoginHash(user, password, realm, random string) string {
	g1 := gen1Hash(password)
	g1md5 := strings.ToUpper(fmt.Sprintf("%x", md5.Sum([]byte(g1))))
	g2 := gen2Hash(user, password, realm, random)
	return user + "&&" + g2 + g1md5
}

func testLogin(addr, username, password string) {
	conn, err := net.DialTimeout("tcp", addr, 2*time.Second)
	if err != nil {
		fmt.Printf("[-] %s: dial failed: %v\n", addr, err)
		return
	}
	defer conn.Close()

	// Step 1: Realm
	realmReq := make([]byte, 32)
	binary.BigEndian.PutUint32(realmReq[0:4], 0xa0010000)
	binary.BigEndian.PutUint64(realmReq[24:32], 0x050201010000a1aa)
	conn.Write(realmReq)

	hdr := make([]byte, 32)
	conn.Read(hdr)
	chunkLen := binary.LittleEndian.Uint32(hdr[4:8])
	body := make([]byte, chunkLen)
	conn.Read(body)

	sBody := string(body)
	i := strings.Index(sBody, "Login to")
	if i < 0 {
		fmt.Printf("[-] %s: realm parse failed: %q\n", addr, sBody)
		return
	}
	j := strings.Index(sBody[i:], "\r\n")
	realm := sBody[i : i+j]
	r := strings.Index(sBody, "Random:")
	rest := sBody[r+len("Random:"):]
	if k := strings.Index(rest, "\r\n"); k >= 0 {
		rest = rest[:k]
	}
	random := strings.TrimSpace(rest)

	// Step 2: Login
	hash := dvripLoginHash(username, password, realm, random)
	loginReq := make([]byte, 32)
	binary.BigEndian.PutUint32(loginReq[0:4], 0xa0050000)
	binary.LittleEndian.PutUint32(loginReq[4:8], uint32(len(hash)))
	binary.BigEndian.PutUint64(loginReq[24:32], 0x050200080000a1aa)
	conn.Write(append(loginReq, []byte(hash)...))

	respHdr := make([]byte, 32)
	conn.Read(respHdr)

	errCodeUint := binary.LittleEndian.Uint32(respHdr[8:12])
	sessionID := binary.LittleEndian.Uint32(respHdr[16:20])

	fmt.Printf("[%s] user=%q pass=%q -> raw hdr[8:12]=%02x %02x %02x %02x (uint32=0x%04x), sessionID=%d\n",
		addr, username, password, respHdr[8], respHdr[9], respHdr[10], respHdr[11], errCodeUint, sessionID)
}

func main() {
	passwords := []string{
		"a12345678", "Admin123456", "smarthome12345", "admin12345", "admin123456",
		"kd123456", "admin", "123456", "888888", "Admin@123", "Admin12345",
	}
	ips := []string{"192.168.1.150", "192.168.1.3", "192.168.1.111"}

	for _, ip := range ips {
		addr := ip + ":37777"
		fmt.Printf("\n=== Testing %s ===\n", addr)
		for _, pwd := range passwords {
			testLogin(addr, "admin", pwd)
		}
	}
}
