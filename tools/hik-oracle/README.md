# hik-oracle (dev-only)

Reference tool that links the proprietary Hikvision **HCNetSDK** to log into a
device/NVR on the private port **8000** and run one `NET_DVR_STDXMLConfig`
request (ISAPI XML tunnelled over 8000). Used to prove read/write config access
and to study the 8000 transport. **The SDK itself is not committed** (gitignored
under `docs-sdk/hikvision/hcnetsdk/`).

## Build & run

```bash
SDK=/path/to/EN-HCNetSDKV6.1.9.4.../lib   # dir with libhcnetsdk.so + HCNetSDKCom/
g++ -o oracle oracle.cpp -I<sdk>/incEn -L$SDK -lhcnetsdk -Wl,-rpath,$SDK
LD_LIBRARY_PATH=$SDK ./oracle <ip> <port> <user> <pass> $SDK/ "GET /ISAPI/System/deviceInfo"
# write: pass an ISAPI XML body file as the 7th arg with a "PUT ..." url
```

Proven against a DS-7108NI-Q1/M NVR: login on 8000 (through NAT), ISAPI GET and
PUT (statusCode 1). The production tool uses the same ISAPI XML via the optional
cgo `hiksdk` backend (build tag `hiksdk`).
