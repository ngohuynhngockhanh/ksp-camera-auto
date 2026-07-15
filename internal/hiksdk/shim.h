//go:build hiksdk

/* Plain-C interface over the C++ HCNetSDK header, so cgo (which compiles its
 * preamble as C) can call into the SDK. Implemented in shim.cpp. */
#ifndef KSPCAM_HIK_SHIM_H
#define KSPCAM_HIK_SHIM_H

#ifdef __cplusplus
extern "C" {
#endif

/* Initialise the SDK and point it at libdir (the dir holding libhcnetsdk.so +
 * HCNetSDKCom/). Returns 0 on success, else the SDK error code. */
int hik_init(const char *libdir);

/* Log in on the device's private port. Returns a non-negative user id, or the
 * negated SDK error code on failure. */
long hik_login(const char *ip, unsigned short port, const char *user, const char *pass);

/* Run one NET_DVR_STDXMLConfig request. url is an ISAPI request line like
 * "GET /ISAPI/Streaming/channels/101". Writes up to outcap bytes of response
 * XML into out (actual length in *outlen) and the status XML into status.
 * Returns 0 on success, else the SDK error code. */
int hik_stdxml(long uid, const char *url, const char *body, unsigned int blen,
               char *out, unsigned int outcap, unsigned int *outlen,
               char *status, unsigned int statuscap);

void hik_logout(long uid);
void hik_cleanup(void);

#ifdef __cplusplus
}
#endif

#endif