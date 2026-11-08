package dahua

import (
	"encoding/json"
	"testing"
)

func TestByteCountAcceptsFirmwareFloat(t *testing.T) {
	var got byteCount
	if err := json.Unmarshal([]byte(`120573657088.0`), &got); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if got != 120573657088 {
		t.Fatalf("got %d", got)
	}
}

func TestByteCountRejectsFraction(t *testing.T) {
	var got byteCount
	if err := json.Unmarshal([]byte(`12.5`), &got); err == nil {
		t.Fatal("expected fractional byte count to fail")
	}
}

func TestDecodeStorageDeviceAllInfoImouEmptySlot(t *testing.T) {
	// Confirmed live on Imou IPC-A32EP-L-3 & IPC-A32E-L over DVRIP 37777 when no SD card is inserted.
	var emptyImou rpcResp
	if err := json.Unmarshal([]byte(`{"id":2,"session":1064,"result":false,"params":{"info":[]},"error":null}`), &emptyImou); err != nil {
		t.Fatalf("unmarshal fixture: %v", err)
	}
	devs, err := decodeStorageDeviceAllInfo(emptyImou)
	if err != nil {
		t.Fatalf("expected empty slice without error on Imou empty SD slot, got err=%v", err)
	}
	if len(devs) != 0 {
		t.Fatalf("expected 0 devices, got %d", len(devs))
	}

	// Genuine RPC errors must still surface as errors.
	var realErr rpcResp
	_ = json.Unmarshal([]byte(`{"id":4,"session":1064,"result":false,"params":null,"error":{"code":268632064,"message":"InterfaceNotFound"}}`), &realErr)
	if _, err := decodeStorageDeviceAllInfo(realErr); err == nil {
		t.Fatal("expected InterfaceNotFound error to surface")
	}
}
