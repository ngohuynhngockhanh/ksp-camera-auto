import socket, struct, re, sys
sys.path.append("/tmp")
import dvrip_probe

for ip in ["192.168.1.111", "192.168.1.112", "192.168.1.113", "192.168.1.114", "192.168.1.115", "192.168.1.116", "192.168.1.117", "192.168.1.118", "192.168.1.150"]:
    try:
        sock = socket.socket()
        sock.settimeout(2.0)
        sock.connect((ip, 37777))
        req1 = bytearray(32)
        req1[0:4] = struct.pack(">I", 0xa0010000)
        req1[24:32] = struct.pack(">Q", 0x050201010000a1aa)
        sock.sendall(req1)
        hdr1, b1 = dvrip_probe.read_frame(sock)
        print("=== " + ip + " ===")
        print("HDR1: " + hdr1.hex())
        print("BODY1: " + repr(b1))
        
        body1_str = b1.decode("latin1", errors="ignore")
        realm = re.search(r"Realm:(Login to [^\r\n]+)", body1_str).group(1)
        random = re.search(r"Random:([^\r\n]+)", body1_str).group(1)
        
        # Test each password on a fresh connection
        sock.close()
        for pwd in ["Admin123456", "smarthome12345", "admin", "123456", "L2D6643F", "L2A9991E", "L251423F", "L2AAA219", "L28BF007", "L250833C", "L22F39D1", "L21F07E0"]:
            s = socket.socket()
            s.settimeout(2.0)
            s.connect((ip, 37777))
            s.sendall(req1)
            _, b_sub = dvrip_probe.read_frame(s)
            r_str = b_sub.decode("latin1", errors="ignore")
            rlm = re.search(r"Realm:(Login to [^\r\n]+)", r_str).group(1)
            rnd = re.search(r"Random:([^\r\n]+)", r_str).group(1)
            
            lhash = dvrip_probe.dvrip_login_hash("admin", pwd, rlm, rnd)
            req2 = bytearray(32)
            req2[0:4] = struct.pack(">I", 0xa0050000)
            req2[4:8] = struct.pack("<I", len(lhash))
            req2[24:32] = struct.pack(">Q", 0x050200080000a1aa)
            s.sendall(req2 + lhash.encode())
            hdr2, b2 = dvrip_probe.read_frame(s)
            err_code = hdr2[8:12]
            err_hex = err_code.hex()
            # 0x0008 success
            is_success = (err_code[0] == 0x00 and err_code[1] == 0x08)
            print("  pwd=" + pwd + " HDR2=" + hdr2.hex() + " errCode=" + err_hex + " success=" + str(is_success))
            s.close()
            if is_success:
                print("  ===> SUCCESS on " + ip + " with pwd=" + pwd)
                break
    except Exception as e:
        print("Error on " + ip + ": " + str(e))
