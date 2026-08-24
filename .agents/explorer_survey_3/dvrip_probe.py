import socket, struct, hashlib, json, time, re

def gen1_hash(password):
    m = hashlib.md5(password.encode()).digest()
    out = []
    for j in range(8):
        v = (m[2*j] + m[2*j+1]) % 62
        if v < 10:
            out.append(chr(v + 48))
        elif v < 36:
            out.append(chr(v + 55))
        else:
            out.append(chr(v + 61))
    return ''.join(out)

def md5_upper(s):
    return hashlib.md5(s.encode()).hexdigest().upper()

def gen2_hash(user, password, realm, random):
    inner = md5_upper(user + ':' + realm + ':' + password)
    return md5_upper(user + ':' + random + ':' + inner)

def dvrip_login_hash(user, password, realm, random):
    return user + '&&' + gen2_hash(user, password, realm, random) + md5_upper(gen1_hash(password))

def read_exact(sock, n):
    buf = bytearray()
    while len(buf) < n:
        chunk = sock.recv(n - len(buf))
        if not chunk:
            raise EOFError('Socket closed')
        buf.extend(chunk)
    return bytes(buf)

def read_frame(sock):
    hdr = read_exact(sock, 32)
    chunk_len = struct.unpack('<I', hdr[4:8])[0]
    body = read_exact(sock, chunk_len) if chunk_len > 0 else b''
    return hdr, body

def probe_dahua(ip, port, user, password):
    sock = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
    sock.settimeout(3.0)
    try:
        sock.connect((ip, port))
    except Exception as e:
        return False, 'Connect failed: ' + str(e), None
    
    try:
        # Step 1: Realm
        req1 = bytearray(32)
        req1[0:4] = struct.pack('>I', 0xa0010000)
        req1[24:32] = struct.pack('>Q', 0x050201010000a1aa)
        sock.sendall(req1)
        _, body1 = read_frame(sock)
        body1_str = body1.decode('latin1', errors='ignore')
        
        realm_m = re.search(r'Realm:(Login to [^\r\n]+)', body1_str)
        rand_m = re.search(r'Random:([^\r\n]+)', body1_str)
        if not realm_m or not rand_m:
            sock.close()
            return False, 'Realm parse failed: ' + repr(body1_str), None
        realm = realm_m.group(1).strip()
        random = rand_m.group(1).strip()
        
        # Step 2: Login
        lhash = dvrip_login_hash(user, password, realm, random)
        req2 = bytearray(32)
        req2[0:4] = struct.pack('>I', 0xa0050000)
        req2[4:8] = struct.pack('<I', len(lhash))
        req2[24:32] = struct.pack('>Q', 0x050200080000a1aa)
        sock.sendall(req2 + lhash.encode())
        hdr2, _ = read_frame(sock)
        
        err_code = hdr2[8:12]
        if not (err_code[0] == 0x00 and err_code[1] == 0x08):
            sock.close()
            return False, 'Auth failed: ' + err_code.hex(), None
        
        session_id = struct.unpack('<I', hdr2[16:20])[0]
        
        # Step 3: RPC Call helper
        msg_id = 100
        def call_rpc(method, params=None):
            nonlocal msg_id
            msg_id += 1
            payload = {'method': method, 'id': msg_id, 'session': session_id}
            if params:
                payload['params'] = params
            data = json.dumps(payload).encode()
            
            hdr = bytearray(32)
            hdr[0:4] = struct.pack('>I', 0xf6000000)
            hdr[4:8] = struct.pack('<I', len(data))
            hdr[8:12] = struct.pack('<I', msg_id)
            hdr[16:20] = struct.pack('<I', len(data))
            hdr[24:28] = struct.pack('<I', session_id)
            sock.sendall(hdr + data)
            
            h, b = read_frame(sock)
            total = struct.unpack('<I', h[16:20])[0]
            full_body = bytearray(b)
            while len(full_body) < total:
                h_next, b_next = read_frame(sock)
                full_body.extend(b_next)
            return json.loads(full_body.decode('utf-8', errors='ignore'))
        
        info = {}
        try:
            info['sys'] = call_rpc('magicBox.getSystemInfo')
        except Exception as e:
            info['sys_err'] = str(e)
            
        try:
            info['sn'] = call_rpc('magicBox.getSerialNo')
        except Exception as e:
            info['sn_err'] = str(e)
            
        try:
            info['encode'] = call_rpc('configManager.getConfig', {'name': 'Encode'})
        except Exception as e:
            info['enc_err'] = str(e)
            
        sock.close()
        return True, 'OK', info
    except Exception as e:
        sock.close()
        return False, 'Error: ' + str(e), None

candidates = [
    'Admin123456', 'admin', 'smarthome12345', 'admin123', 'admin12345', '123456',
    'L2D6643F', 'L2A9991E', 'L251423F', 'L2AAA219', 'L28BF007', 'L250833C', 'L22F39D1', 'L21F07E0'
]

ips_to_scan = [
    '192.168.1.111', '192.168.1.112', '192.168.1.113', '192.168.1.114',
    '192.168.1.115', '192.168.1.116', '192.168.1.117', '192.168.1.118',
    '192.168.1.150',
    '192.168.1.190', '192.168.1.191', '192.168.1.192', '192.168.1.193',
    '192.168.1.194', '192.168.1.195', '192.168.1.196', '192.168.1.197'
]

results = {}
for ip in ips_to_scan:
    print('=== Probing ' + ip + ' ===')
    matched = False
    for pwd in candidates:
        ok, msg, data = probe_dahua(ip, 37777, 'admin', pwd)
        if ok:
            print('  [SUCCESS] ' + ip + ' password=' + pwd)
            results[ip] = {'password': pwd, 'data': data}
            matched = True
            break
        elif 'Connect failed' in msg:
            print('  [UNREACHABLE] ' + ip)
            break
    if not matched and ip not in results:
        print('  [FAILED AUTH] ' + ip)

with open('/tmp/dahua_scan_results.json', 'w') as out_f:
    json.dump(results, out_f, indent=2)
print('Done scanning. Saved to /tmp/dahua_scan_results.json')
