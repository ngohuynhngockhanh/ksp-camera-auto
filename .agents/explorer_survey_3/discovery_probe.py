import socket, struct, json, time, uuid, xml.etree.ElementTree as ET

def test_dahua_discover():
    print("=== DHDiscover (UDP 37810) ===")
    s = socket.socket(socket.AF_INET, socket.SOCK_DGRAM)
    s.setsockopt(socket.SOL_SOCKET, socket.SO_BROADCAST, 1)
    s.settimeout(3.0)
    s.bind(('', 0))
    
    payload = json.dumps({"method": "DHDiscover.search", "params": {"mac": "", "uni": 1}}).encode()
    pkt = bytearray(32 + len(payload))
    pkt[0] = 0x20
    pkt[4:8] = b'DHIP'
    pkt[16:20] = struct.pack('<I', len(payload))
    pkt[24:28] = struct.pack('<I', len(payload))
    pkt[32:] = payload
    
    s.sendto(pkt, ('255.255.255.255', 37810))
    try:
        s.sendto(pkt, ('239.255.255.251', 37810))
    except Exception as e:
        pass
        
    start = time.time()
    discovered = []
    while time.time() - start < 3.0:
        try:
            data, addr = s.recvfrom(65535)
            if len(data) >= 32 and data[4:8] == b'DHIP':
                body = data[32:].rstrip(b'\x00').decode('utf-8', errors='ignore')
                print("Dahua response from:", addr, body[:200])
                try:
                    j = json.loads(body)
                    discovered.append((addr[0], j))
                except Exception as e:
                    discovered.append((addr[0], body))
        except socket.timeout:
            break
        except Exception as e:
            print("Error recv:", e)
            break
    s.close()
    return discovered

def test_onvif_discover():
    print("=== ONVIF WS-Discovery (UDP 3702) ===")
    msg_id = "urn:uuid:" + str(uuid.uuid4())
    soap = f"""<?xml version="1.0" encoding="UTF-8"?>
<e:Envelope xmlns:e="http://www.w3.org/2003/05/soap-envelope"
            xmlns:w="http://schemas.xmlsoap.org/ws/2004/08/addressing"
            xmlns:d="http://schemas.xmlsoap.org/ws/2005/04/discovery"
            xmlns:dn="http://www.onvif.org/ver10/network/wsdl">
  <e:Header>
    <w:MessageID>{msg_id}</w:MessageID>
    <w:To e:mustUnderstand="1">urn:schemas-xmlsoap-org:ws:2005:04:discovery</w:To>
    <w:Action>http://schemas.xmlsoap.org/ws/2005/04/discovery/Probe</w:Action>
  </e:Header>
  <e:Body>
    <d:Probe>
      <d:Types>dn:NetworkVideoTransmitter</d:Types>
    </d:Probe>
  </e:Body>
</e:Envelope>"""

    s = socket.socket(socket.AF_INET, socket.SOCK_DGRAM)
    s.setsockopt(socket.SOL_SOCKET, socket.SO_BROADCAST, 1)
    s.settimeout(3.0)
    s.bind(('', 0))
    s.sendto(soap.encode(), ('239.255.255.250', 3702))
    
    start = time.time()
    discovered = []
    while time.time() - start < 3.0:
        try:
            data, addr = s.recvfrom(65535)
            text = data.decode('utf-8', errors='ignore')
            print("ONVIF response from:", addr, text[:300])
            discovered.append((addr[0], text))
        except socket.timeout:
            break
        except Exception as e:
            print("Error onvif:", e)
            break
    s.close()
    return discovered

if __name__ == '__main__':
    dh = test_dahua_discover()
    onv = test_onvif_discover()
    with open('/tmp/discover_results.json', 'w') as f:
        json.dump({'dahua': dh, 'onvif': onv}, f, indent=2)
