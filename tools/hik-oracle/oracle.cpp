// hik-oracle: a DEV-ONLY reference tool. It links the proprietary Hikvision
// HCNetSDK to log into an NVR/camera on the private port 8000 and run one
// NET_DVR_STDXMLConfig request (ISAPI XML tunnelled over 8000). Used to (a)
// prove we can read/write config on the device and (b) capture the 8000 wire
// protocol for the pure-Go clone effort. NOT shipped; the SDK is not committed.
//
// Usage:
//   ./oracle <ip> <port> <user> <pass> <sdk_lib_dir> "<METHOD> <isapi-path>" [inbody-file]
// e.g.
//   ./oracle 1.2.3.4 65527 admin 'secret' /path/to/lib "GET /ISAPI/System/deviceInfo"
#include "HCNetSDK.h"
#include <stdio.h>
#include <stdlib.h>
#include <string.h>

static char *slurp(const char *path, DWORD *n) {
    FILE *f = fopen(path, "rb");
    if (!f) return NULL;
    fseek(f, 0, SEEK_END);
    long sz = ftell(f);
    fseek(f, 0, SEEK_SET);
    char *buf = (char *)malloc(sz + 1);
    fread(buf, 1, sz, f);
    buf[sz] = 0;
    fclose(f);
    *n = (DWORD)sz;
    return buf;
}

int main(int argc, char **argv) {
    if (argc < 7) {
        fprintf(stderr, "usage: %s <ip> <port> <user> <pass> <sdk_lib_dir> \"<METHOD> <path>\" [inbody-file]\n", argv[0]);
        return 2;
    }
    const char *ip = argv[1];
    WORD port = (WORD)atoi(argv[2]);
    const char *user = argv[3];
    const char *pass = argv[4];
    const char *libdir = argv[5];
    const char *url = argv[6];

    NET_DVR_Init();
    NET_DVR_SetConnectTime(5000, 1);
    NET_DVR_SetReconnect(10000, 1);
    NET_DVR_SetLogToFile(3, "/tmp/hiksdk_log", TRUE); // verbose SDK log for RE

    // Point the SDK at its component libs (HCNetSDKCom lives under libdir).
    NET_DVR_LOCAL_SDK_PATH comPath;
    memset(&comPath, 0, sizeof(comPath));
    strncpy(comPath.sPath, libdir, sizeof(comPath.sPath) - 1);
    NET_DVR_SetSDKInitCfg(NET_SDK_INIT_CFG_SDK_PATH, &comPath);

    NET_DVR_DEVICEINFO_V30 dev;
    memset(&dev, 0, sizeof(dev));
    LONG uid = NET_DVR_Login_V30((char *)ip, port, (char *)user, (char *)pass, &dev);
    if (uid < 0) {
        printf("LOGIN_FAIL err=%u\n", NET_DVR_GetLastError());
        NET_DVR_Cleanup();
        return 1;
    }
    printf("LOGIN_OK uid=%ld startChan=%d chanNum=%d startDChan=%d\n",
           uid, dev.byStartChan, dev.byChanNum, dev.byStartDChan);

    NET_DVR_XML_CONFIG_INPUT in;
    memset(&in, 0, sizeof(in));
    in.dwSize = sizeof(in);
    in.lpRequestUrl = (void *)url;
    in.dwRequestUrlLen = (DWORD)strlen(url);
    in.dwRecvTimeOut = 5000;

    DWORD inLen = 0;
    char *inBody = NULL;
    if (argc >= 8) {
        inBody = slurp(argv[7], &inLen);
        in.lpInBuffer = inBody;
        in.dwInBufferSize = inLen;
    }

    DWORD outCap = 1 << 20;
    char *out = (char *)malloc(outCap);
    char status[8192];
    memset(status, 0, sizeof(status));
    NET_DVR_XML_CONFIG_OUTPUT o;
    memset(&o, 0, sizeof(o));
    o.dwSize = sizeof(o);
    o.lpOutBuffer = out;
    o.dwOutBufferSize = outCap;
    o.lpStatusBuffer = status;
    o.dwStatusSize = sizeof(status);

    BOOL ok = NET_DVR_STDXMLConfig(uid, &in, &o);
    if (!ok) {
        printf("STDXML_FAIL err=%u status=%.*s\n", NET_DVR_GetLastError(),
               (int)o.dwStatusSize, status);
    } else {
        printf("STDXML_OK returned=%u status=%.*s\n---XML-START---\n%.*s\n---XML-END---\n",
               o.dwReturnedXMLSize, (int)o.dwStatusSize, status,
               (int)o.dwReturnedXMLSize, out);
    }

    NET_DVR_Logout(uid);
    NET_DVR_Cleanup();
    free(out);
    if (inBody) free(inBody);
    return ok ? 0 : 1;
}
