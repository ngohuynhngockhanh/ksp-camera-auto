//go:build hiksdk

// C++ shim over HCNetSDK, exposing the plain-C interface in shim.h. Compiled by
// cgo as C++ (the HCNetSDK.h header uses `extern "C"` and requires a C++ TU).
#include "HCNetSDK.h"
#include "shim.h"
#include <string.h>

extern "C" int hik_init(const char *libdir) {
    if (!NET_DVR_Init()) return (int)NET_DVR_GetLastError();
    NET_DVR_SetConnectTime(5000, 1);
    NET_DVR_SetReconnect(10000, 1);
    if (libdir && libdir[0]) {
        NET_DVR_LOCAL_SDK_PATH p;
        memset(&p, 0, sizeof(p));
        strncpy(p.sPath, libdir, sizeof(p.sPath) - 1);
        NET_DVR_SetSDKInitCfg(NET_SDK_INIT_CFG_SDK_PATH, &p);
    }
    return 0;
}

extern "C" long hik_login(const char *ip, unsigned short port, const char *user, const char *pass) {
    NET_DVR_DEVICEINFO_V30 dev;
    memset(&dev, 0, sizeof(dev));
    LONG uid = NET_DVR_Login_V30((char *)ip, port, (char *)user, (char *)pass, &dev);
    if (uid < 0) return -(long)NET_DVR_GetLastError();
    return (long)uid;
}

extern "C" int hik_stdxml(long uid, const char *url, const char *body, unsigned int blen,
                          char *out, unsigned int outcap, unsigned int *outlen,
                          char *status, unsigned int statuscap) {
    NET_DVR_XML_CONFIG_INPUT in;
    memset(&in, 0, sizeof(in));
    in.dwSize = sizeof(in);
    in.lpRequestUrl = (void *)url;
    in.dwRequestUrlLen = (DWORD)strlen(url);
    in.dwRecvTimeOut = 5000;
    if (blen) {
        in.lpInBuffer = (void *)body;
        in.dwInBufferSize = blen;
    }

    NET_DVR_XML_CONFIG_OUTPUT o;
    memset(&o, 0, sizeof(o));
    o.dwSize = sizeof(o);
    o.lpOutBuffer = out;
    o.dwOutBufferSize = outcap;
    o.lpStatusBuffer = status;
    o.dwStatusSize = statuscap;
    if (statuscap) status[0] = 0;

    BOOL ok = NET_DVR_STDXMLConfig((LONG)uid, &in, &o);
    *outlen = o.dwReturnedXMLSize;
    if (!ok) return (int)NET_DVR_GetLastError();
    return 0;
}

extern "C" void hik_logout(long uid) { NET_DVR_Logout((LONG)uid); }
extern "C" void hik_cleanup(void) { NET_DVR_Cleanup(); }