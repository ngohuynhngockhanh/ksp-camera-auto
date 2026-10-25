package bidahub

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"
)

type seedBoxSpec struct {
	ID              string
	ClubID          string
	ClubName        string
	BoxName         string
	Branding        string
	VPNIP           string
	LANIP           string
	VirtualIP       string
	SoCModel        string
	Arch            string
	RAMTotalMB      int
	RAMAvailableMB  int
	ZRAMTotalMB     int
	ZRAMAlgo        string
	SDTotalGB       int
	SDUsedPct       int
	SDFSType        string
	SDMountOpts     string
	CamerasCount    int
	Vendor          string
	Resolution      string
	Width           int
	Height          int
	FPS             int
	BitrateKbps     int
	Codec           string
	NVRHost         string
	DoubleStreamIDs []int
	ShinobiGroupKey string
	StoreDomain     string
	CStoreDomain    string
	PM2CameraMB     int
	HardeningScore  int
	Notes           string
}

var builtInFleetSpecs = []seedBoxSpec{
	{
		ID: "inut_204_141", ClubID: "club_789_mia", ClubName: "789 Billiards (MIAPool)", BoxName: "789 Billiards — MIAPool Box 1",
		Branding: "billiards", VPNIP: "77.88.204.141", LANIP: "192.168.1.10", VirtualIP: "192.168.1.250",
		SoCModel: "Allwinner H313", Arch: "aarch64", RAMTotalMB: 1945, RAMAvailableMB: 1120, ZRAMTotalMB: 1945, ZRAMAlgo: "lz4",
		SDTotalGB: 58, SDUsedPct: 48, SDFSType: "f2fs", SDMountOpts: "rw,noatime,errors=continue",
		CamerasCount: 7, Vendor: "Dahua", Resolution: "1920x1080", Width: 1920, Height: 1080, FPS: 30, BitrateKbps: 1536, Codec: "H.265",
		ShinobiGroupKey: "mia", StoreDomain: "store-789billiards.video.io.vn", CStoreDomain: "c-store-789billiards.video.io.vn",
		PM2CameraMB: 300, HardeningScore: 8, Notes: "Virtual IP 192.168.1.250/24 end0:1, doSnapshot=false, iso5 fragmented MP4",
	},
	{
		ID: "inut_204_150", ClubID: "club_99_billiards_cs11", ClubName: "99 Billiards - CS11", BoxName: "99 Billiards - CS11 - Box 1",
		Branding: "billiards", VPNIP: "77.88.204.150", LANIP: "172.10.17.252", VirtualIP: "172.10.17.252",
		SoCModel: "Rockchip RK3528", Arch: "aarch64", RAMTotalMB: 1843, RAMAvailableMB: 780, ZRAMTotalMB: 2662, ZRAMAlgo: "lz4",
		SDTotalGB: 118, SDUsedPct: 62, SDFSType: "f2fs", SDMountOpts: "rw,noatime,errors=continue,users",
		CamerasCount: 12, Vendor: "Dahua", Resolution: "1920x1080", Width: 1920, Height: 1080, FPS: 30, BitrateKbps: 1536, Codec: "H.265",
		ShinobiGroupKey: "99cs111", StoreDomain: "store-99cs11-1.video.io.vn", CStoreDomain: "c-store-99cs11-1.video.io.vn",
		PM2CameraMB: 220, HardeningScore: 8, Notes: "Multi-box CS11 Node 1, 24/24 FFmpeg active",
	},
	{
		ID: "inut_204_152", ClubID: "club_99_billiards_cs11", ClubName: "99 Billiards - CS11", BoxName: "99 Billiards - CS11 - Box 2",
		Branding: "billiards", VPNIP: "77.88.204.152", LANIP: "172.10.16.109", VirtualIP: "172.10.16.109",
		SoCModel: "Rockchip RK3528", Arch: "aarch64", RAMTotalMB: 1843, RAMAvailableMB: 1180, ZRAMTotalMB: 1843, ZRAMAlgo: "lz4",
		SDTotalGB: 116, SDUsedPct: 58, SDFSType: "f2fs", SDMountOpts: "rw,noatime,errors=continue,users",
		CamerasCount: 12, Vendor: "Dahua", Resolution: "1920x1080", Width: 1920, Height: 1080, FPS: 30, BitrateKbps: 1536, Codec: "H.265",
		ShinobiGroupKey: "99cs112", StoreDomain: "store-99cs11-2.video.io.vn", CStoreDomain: "c-store-99cs11-2.video.io.vn",
		PM2CameraMB: 220, HardeningScore: 8, Notes: "Multi-box CS11 Node 2, 116GB F2FS preserved",
	},
	{
		ID: "inut_206_60", ClubID: "club_99_billiards_cs11", ClubName: "99 Billiards - CS11", BoxName: "99 Billiards - CS11 - Box 3 (VIP)",
		Branding: "billiards", VPNIP: "77.88.206.60", LANIP: "172.10.16.122", VirtualIP: "172.10.16.122",
		SoCModel: "Rockchip RK3568", Arch: "aarch64", RAMTotalMB: 3891, RAMAvailableMB: 2150, ZRAMTotalMB: 3891, ZRAMAlgo: "lz4",
		SDTotalGB: 233, SDUsedPct: 54, SDFSType: "ext4", SDMountOpts: "rw,noatime,errors=continue,users",
		CamerasCount: 19, Vendor: "Dahua", Resolution: "2560x1440", Width: 2560, Height: 1440, FPS: 30, BitrateKbps: 1536, Codec: "H.265",
		ShinobiGroupKey: "vip", StoreDomain: "store-99cs11-3.video.io.vn", CStoreDomain: "c-store-99cs11-3.video.io.vn",
		PM2CameraMB: 350, HardeningScore: 8, Notes: "Multi-box CS11 Node 3 VIP, 19 cameras on 233GB ext4",
	},
	{
		ID: "inut_204_157", ClubID: "club_kely_pickleball", ClubName: "Kely Pickleball", BoxName: "Kely Pickleball",
		Branding: "pickleball", VPNIP: "77.88.204.157", LANIP: "192.168.1.250", VirtualIP: "192.168.1.250",
		SoCModel: "ARM64 Octopus Planet", Arch: "aarch64", RAMTotalMB: 1843, RAMAvailableMB: 890, ZRAMTotalMB: 3584, ZRAMAlgo: "lz4",
		SDTotalGB: 118, SDUsedPct: 51, SDFSType: "f2fs", SDMountOpts: "rw,noatime,errors=continue",
		CamerasCount: 12, Vendor: "Dahua", Resolution: "1920x1080", Width: 1920, Height: 1080, FPS: 30, BitrateKbps: 1536, Codec: "H.265",
		ShinobiGroupKey: "kely", StoreDomain: "store-kely.video.io.vn", CStoreDomain: "c-store-kely.video.io.vn",
		PM2CameraMB: 220, HardeningScore: 8, Notes: "12 sân Pickleball, 24/24 FFmpeg",
	},
	{
		ID: "inut_205_72", ClubID: "club_master_billiards", ClubName: "Master Billiards Club", BoxName: "Master Billiards Club",
		Branding: "billiards", VPNIP: "77.88.205.72", LANIP: "192.168.1.200", VirtualIP: "192.168.1.254",
		SoCModel: "Allwinner H313", Arch: "aarch64", RAMTotalMB: 1945, RAMAvailableMB: 1024, ZRAMTotalMB: 1945, ZRAMAlgo: "lz4",
		SDTotalGB: 118, SDUsedPct: 49, SDFSType: "f2fs", SDMountOpts: "rw,noatime,errors=continue",
		CamerasCount: 10, Vendor: "Dahua", Resolution: "2880x1620", Width: 2880, Height: 1620, FPS: 30, BitrateKbps: 1536, Codec: "H.265",
		ShinobiGroupKey: "master2", StoreDomain: "store-master2.video.io.vn", CStoreDomain: "c-store-master2.video.io.vn",
		PM2CameraMB: 220, HardeningScore: 8, Notes: "10 camera Dahua 3K 2880x1620 @ 30fps H.265 AAC",
	},
	{
		ID: "inut_205_87", ClubID: "club_heypick_pickleball", ClubName: "Heypick Pickleball 292 Kim Giang", BoxName: "Heypick Pickleball 292 Kim Giang",
		Branding: "pickleball", VPNIP: "77.88.205.87", LANIP: "192.168.1.210", VirtualIP: "192.168.1.254",
		SoCModel: "Allwinner H313", Arch: "aarch64", RAMTotalMB: 1945, RAMAvailableMB: 880, ZRAMTotalMB: 1945, ZRAMAlgo: "lz4",
		SDTotalGB: 58, SDUsedPct: 55, SDFSType: "f2fs", SDMountOpts: "rw,noatime,errors=continue",
		CamerasCount: 7, Vendor: "Dahua", Resolution: "1920x1080", Width: 1920, Height: 1080, FPS: 30, BitrateKbps: 1536, Codec: "H.265",
		ShinobiGroupKey: "heyPick", StoreDomain: "store-heypick.video.io.vn", CStoreDomain: "c-store-heypick.video.io.vn",
		PM2CameraMB: 180, HardeningScore: 8, Notes: "7 sân Pickleball, 14/14 FFmpeg",
	},
	{
		ID: "inut_204_159", ClubID: "club_axis_billiards", ClubName: "Axis Billiards", BoxName: "Axis Billiards",
		Branding: "billiards", VPNIP: "77.88.204.159", LANIP: "192.168.1.159", VirtualIP: "192.168.1.254",
		SoCModel: "Amlogic S912", Arch: "aarch64", RAMTotalMB: 1843, RAMAvailableMB: 860, ZRAMTotalMB: 1843, ZRAMAlgo: "lz4",
		SDTotalGB: 118, SDUsedPct: 52, SDFSType: "f2fs", SDMountOpts: "rw,noatime,errors=continue",
		CamerasCount: 9, Vendor: "Dahua", Resolution: "2880x1620", Width: 2880, Height: 1620, FPS: 30, BitrateKbps: 1536, Codec: "H.265",
		ShinobiGroupKey: "axis", StoreDomain: "store-axis.video.io.vn", CStoreDomain: "c-store-axis.video.io.vn",
		PM2CameraMB: 200, HardeningScore: 8, Notes: "Amlogic S912 CPU=8 sysrq trap removed, 18/18 FFmpeg",
	},
	{
		ID: "inut_205_67", ClubID: "club_bink_billiards", ClubName: "BinK Billiards Club", BoxName: "BinK Billiards Club",
		Branding: "billiards", VPNIP: "77.88.205.67", LANIP: "192.168.1.167", VirtualIP: "192.168.1.254",
		SoCModel: "Allwinner H313", Arch: "aarch64", RAMTotalMB: 1945, RAMAvailableMB: 887, ZRAMTotalMB: 1945, ZRAMAlgo: "lz4",
		SDTotalGB: 118, SDUsedPct: 60, SDFSType: "f2fs", SDMountOpts: "rw,noatime,errors=continue",
		CamerasCount: 11, Vendor: "Dahua", Resolution: "2560x1440", Width: 2560, Height: 1440, FPS: 30, BitrateKbps: 1536, Codec: "H.265",
		ShinobiGroupKey: "billiardclub", StoreDomain: "store-bink.video.io.vn", CStoreDomain: "c-store-bink.video.io.vn",
		PM2CameraMB: 220, HardeningScore: 8, Notes: "11 camera (10 cam 4MP H.265 + 1 cam 2MP), 22/22 FFmpeg",
	},
	{
		ID: "inut_205_69", ClubID: "club_phe_vietnam", ClubName: "PHE Vietnam", BoxName: "PHE Vietnam",
		Branding: "billiards", VPNIP: "77.88.205.69", LANIP: "192.168.1.169", VirtualIP: "192.168.1.254",
		SoCModel: "Allwinner H313", Arch: "aarch64", RAMTotalMB: 1945, RAMAvailableMB: 1070, ZRAMTotalMB: 1945, ZRAMAlgo: "lz4",
		SDTotalGB: 58, SDUsedPct: 45, SDFSType: "f2fs", SDMountOpts: "rw,noatime,errors=continue",
		CamerasCount: 6, Vendor: "Dahua", Resolution: "1920x1080", Width: 1920, Height: 1080, FPS: 30, BitrateKbps: 1536, Codec: "H.265",
		ShinobiGroupKey: "phe", StoreDomain: "store-phe.video.io.vn", CStoreDomain: "c-store-phe.video.io.vn",
		PM2CameraMB: 170, HardeningScore: 8, Notes: "eMMC swapfile removed, 12/12 FFmpeg",
	},
	{
		ID: "inut_205_85", ClubID: "club_lionking_nd", ClubName: "Lion King Nam Định CS2", BoxName: "Lion King Nam Định CS2",
		Branding: "billiards", VPNIP: "77.88.205.85", LANIP: "192.168.1.185", VirtualIP: "192.168.1.254",
		SoCModel: "Allwinner H313", Arch: "aarch64", RAMTotalMB: 1945, RAMAvailableMB: 1100, ZRAMTotalMB: 1945, ZRAMAlgo: "lz4",
		SDTotalGB: 58, SDUsedPct: 52, SDFSType: "f2fs", SDMountOpts: "rw,noatime,errors=continue",
		CamerasCount: 6, Vendor: "Dahua", Resolution: "2304x1296", Width: 2304, Height: 1296, FPS: 30, BitrateKbps: 1536, Codec: "H.265",
		NVRHost: "192.168.1.250", ShinobiGroupKey: "lionkingnd", StoreDomain: "store-lionkingnd.video.io.vn", CStoreDomain: "c-store-lionkingnd.video.io.vn",
		PM2CameraMB: 170, HardeningScore: 8, Notes: "6 camera Dahua NVR 2304x1296 H.265 AAC",
	},
	{
		ID: "inut_205_56", ClubID: "club_kingkong_tn", ClubName: "King Kong Thái Nguyên", BoxName: "King Kong Thái Nguyên",
		Branding: "billiards", VPNIP: "77.88.205.56", LANIP: "192.168.1.156", VirtualIP: "192.168.1.254",
		SoCModel: "Allwinner H313", Arch: "aarch64", RAMTotalMB: 1945, RAMAvailableMB: 1050, ZRAMTotalMB: 1945, ZRAMAlgo: "lz4",
		SDTotalGB: 118, SDUsedPct: 56, SDFSType: "f2fs", SDMountOpts: "rw,noatime,errors=continue",
		CamerasCount: 9, Vendor: "Dahua", Resolution: "1920x1080", Width: 1920, Height: 1080, FPS: 30, BitrateKbps: 1536, Codec: "H.265",
		ShinobiGroupKey: "mp4", StoreDomain: "store-kingkongtn.video.io.vn", CStoreDomain: "c-store-kingkongtn.video.io.vn",
		PM2CameraMB: 200, HardeningScore: 8, Notes: "9 camera Dahua 1536k GOP 50 H.265 AAC",
	},
	{
		ID: "inut_205_53", ClubID: "club_the_best_billiards", ClubName: "The Best Billiards Club", BoxName: "The Best Billiards Club",
		Branding: "billiards", VPNIP: "77.88.205.53", LANIP: "192.168.1.153", VirtualIP: "192.168.1.254",
		SoCModel: "Allwinner H313", Arch: "aarch64", RAMTotalMB: 1945, RAMAvailableMB: 981, ZRAMTotalMB: 1945, ZRAMAlgo: "lz4",
		SDTotalGB: 118, SDUsedPct: 61, SDFSType: "f2fs", SDMountOpts: "rw,noatime,errors=continue",
		CamerasCount: 12, Vendor: "Dahua", Resolution: "1920x1080", Width: 1920, Height: 1080, FPS: 30, BitrateKbps: 1536, Codec: "H.265",
		ShinobiGroupKey: "ultrapool1", StoreDomain: "store-thebest.video.io.vn", CStoreDomain: "c-store-thebest.video.io.vn",
		PM2CameraMB: 220, HardeningScore: 8, Notes: "12 cameras, audio transcode disabled",
	},
	{
		ID: "inut_205_62", ClubID: "club_billiards_2000", ClubName: "Billiards 2000 Club", BoxName: "Billiards 2000 Club",
		Branding: "billiards", VPNIP: "77.88.205.62", LANIP: "192.168.1.162", VirtualIP: "192.168.1.254",
		SoCModel: "Allwinner H313", Arch: "aarch64", RAMTotalMB: 1945, RAMAvailableMB: 1060, ZRAMTotalMB: 1945, ZRAMAlgo: "lz4",
		SDTotalGB: 58, SDUsedPct: 50, SDFSType: "f2fs", SDMountOpts: "rw,noatime,errors=continue",
		CamerasCount: 8, Vendor: "Dahua", Resolution: "1920x1080", Width: 1920, Height: 1080, FPS: 30, BitrateKbps: 1536, Codec: "H.265",
		ShinobiGroupKey: "2000", StoreDomain: "store-2000club.video.io.vn", CStoreDomain: "c-store-2000club.video.io.vn",
		PM2CameraMB: 185, HardeningScore: 8, Notes: "8 camera Dahua, 16/16 FFmpeg",
	},
	{
		ID: "inut_205_6", ClubID: "club_thelight_pool", ClubName: "TheLight Pool Arena", BoxName: "TheLight Pool Arena",
		Branding: "billiards", VPNIP: "77.88.205.6", LANIP: "192.168.1.106", VirtualIP: "192.168.1.254",
		SoCModel: "Allwinner H313", Arch: "aarch64", RAMTotalMB: 1945, RAMAvailableMB: 969, ZRAMTotalMB: 3891, ZRAMAlgo: "lz4",
		SDTotalGB: 118, SDUsedPct: 57, SDFSType: "f2fs", SDMountOpts: "rw,noatime,errors=continue",
		CamerasCount: 10, Vendor: "Dahua", Resolution: "1920x1080", Width: 1920, Height: 1080, FPS: 30, BitrateKbps: 1536, Codec: "H.265",
		ShinobiGroupKey: "thelightpoolarena", StoreDomain: "store-thelight.video.io.vn", CStoreDomain: "c-store-thelight.video.io.vn",
		PM2CameraMB: 200, HardeningScore: 8, Notes: "10 camera, 20/20 FFmpeg",
	},
	{
		ID: "inut_205_10", ClubID: "club_the_king_billiard", ClubName: "CLB Billiard The King", BoxName: "CLB Billiard The King",
		Branding: "billiards", VPNIP: "77.88.205.10", LANIP: "192.168.1.110", VirtualIP: "192.168.1.254",
		SoCModel: "Allwinner H313", Arch: "aarch64", RAMTotalMB: 1945, RAMAvailableMB: 1024, ZRAMTotalMB: 1945, ZRAMAlgo: "lz4",
		SDTotalGB: 58, SDUsedPct: 68, SDFSType: "f2fs", SDMountOpts: "rw,noatime,errors=continue",
		CamerasCount: 8, Vendor: "Dahua", Resolution: "1920x1080", Width: 1920, Height: 1080, FPS: 30, BitrateKbps: 1536, Codec: "H.265",
		ShinobiGroupKey: "xinchao", StoreDomain: "store-theking.video.io.vn", CStoreDomain: "c-store-theking.video.io.vn",
		PM2CameraMB: 185, HardeningScore: 8, Notes: "8 camera Dahua, 16/16 FFmpeg",
	},
	{
		ID: "inut_205_77", ClubID: "club_zone_plus_center", ClubName: "ZONE+ Billiards Center", BoxName: "ZONE+ Billiards Center",
		Branding: "billiards", VPNIP: "77.88.205.77", LANIP: "192.168.1.177", VirtualIP: "192.168.1.254",
		SoCModel: "Allwinner H313", Arch: "aarch64", RAMTotalMB: 1945, RAMAvailableMB: 1170, ZRAMTotalMB: 1945, ZRAMAlgo: "lz4",
		SDTotalGB: 118, SDUsedPct: 59, SDFSType: "f2fs", SDMountOpts: "rw,noatime,errors=continue",
		CamerasCount: 12, Vendor: "Dahua", Resolution: "1920x1080", Width: 1920, Height: 1080, FPS: 30, BitrateKbps: 1536, Codec: "H.265",
		ShinobiGroupKey: "toktok3", StoreDomain: "store-zonepluscenter.video.io.vn", CStoreDomain: "c-store-zonepluscenter.video.io.vn",
		PM2CameraMB: 220, HardeningScore: 8, Notes: "12 camera Dahua, 24/24 FFmpeg",
	},
	{
		ID: "inut_204_148", ClubID: "club_ruby_billiard", ClubName: "Ruby Billiard Club", BoxName: "Ruby Billiard Club",
		Branding: "billiards", VPNIP: "77.88.204.148", LANIP: "192.168.1.148", VirtualIP: "192.168.1.254",
		SoCModel: "Allwinner H313", Arch: "aarch64", RAMTotalMB: 1945, RAMAvailableMB: 1120, ZRAMTotalMB: 1945, ZRAMAlgo: "lz4",
		SDTotalGB: 58, SDUsedPct: 53, SDFSType: "f2fs", SDMountOpts: "rw,noatime,errors=continue",
		CamerasCount: 7, Vendor: "Hikvision", Resolution: "1920x1080", Width: 1920, Height: 1080, FPS: 30, BitrateKbps: 1536, Codec: "H.265",
		ShinobiGroupKey: "na", StoreDomain: "store-ruby.video.io.vn", CStoreDomain: "c-store-ruby.video.io.vn",
		PM2CameraMB: 180, HardeningScore: 8, Notes: "7 camera Hikvision, 14/14 FFmpeg",
	},
	{
		ID: "inut_207_12", ClubID: "club_poseidon_billiards", ClubName: "Poseidon Billiards", BoxName: "Poseidon Billiards",
		Branding: "billiards", VPNIP: "77.88.207.12", LANIP: "192.168.1.212", VirtualIP: "192.168.1.254",
		SoCModel: "Rockchip RK322x", Arch: "armv7l", RAMTotalMB: 2048, RAMAvailableMB: 1024, ZRAMTotalMB: 2048, ZRAMAlgo: "lz4",
		SDTotalGB: 118, SDUsedPct: 64, SDFSType: "f2fs", SDMountOpts: "rw,noatime,errors=continue",
		CamerasCount: 17, Vendor: "Dahua", Resolution: "1920x1080", Width: 1920, Height: 1080, FPS: 30, BitrateKbps: 1536, Codec: "H.265",
		ShinobiGroupKey: "hk", StoreDomain: "store-poseidon.video.io.vn", CStoreDomain: "c-store-poseidon.video.io.vn",
		PM2CameraMB: 260, HardeningScore: 8, Notes: "17 camera Dahua, 34/34 FFmpeg",
	},
	{
		ID: "inut_205_122", ClubID: "club_magic_arena_dn", ClubName: "Magic Arena Đà Nẵng", BoxName: "Magic Arena Đà Nẵng - Box 3 (NVR)",
		Branding: "billiards", VPNIP: "77.88.205.122", LANIP: "192.168.1.122", VirtualIP: "192.168.1.122",
		SoCModel: "ARMv8 Cortex-A53", Arch: "aarch64", RAMTotalMB: 1945, RAMAvailableMB: 940, ZRAMTotalMB: 2867, ZRAMAlgo: "lz4",
		SDTotalGB: 118, SDUsedPct: 61, SDFSType: "f2fs", SDMountOpts: "rw,noatime,errors=continue,users",
		CamerasCount: 16, Vendor: "Dahua", Resolution: "2560x1440", Width: 2560, Height: 1440, FPS: 30, BitrateKbps: 1536, Codec: "H.265",
		NVRHost: "192.168.1.250", ShinobiGroupKey: "magic3", StoreDomain: "store-magicdn3.video.io.vn", CStoreDomain: "c-store-magicdn3.video.io.vn",
		PM2CameraMB: 250, HardeningScore: 8, Notes: "Magic Arena Đà Nẵng NVR Box 3",
	},
	{
		ID: "inut_205_123", ClubID: "club_magic_arena_dn", ClubName: "Magic Arena Đà Nẵng", BoxName: "Magic Arena Đà Nẵng - Box 2",
		Branding: "billiards", VPNIP: "77.88.205.123", LANIP: "192.168.1.123", VirtualIP: "192.168.1.123",
		SoCModel: "ARMv8 Cortex-A53", Arch: "aarch64", RAMTotalMB: 1945, RAMAvailableMB: 980, ZRAMTotalMB: 1945, ZRAMAlgo: "lz4",
		SDTotalGB: 58, SDUsedPct: 65, SDFSType: "f2fs", SDMountOpts: "rw,noatime,errors=continue,users",
		CamerasCount: 12, Vendor: "Dahua", Resolution: "2560x1440", Width: 2560, Height: 1440, FPS: 30, BitrateKbps: 1536, Codec: "H.265",
		NVRHost: "192.168.1.250", ShinobiGroupKey: "magic2", StoreDomain: "store-magicdn2.video.io.vn", CStoreDomain: "c-store-magicdn2.video.io.vn",
		PM2CameraMB: 220, HardeningScore: 8, Notes: "Magic Arena Đà Nẵng Box 2, 24/24 FFmpeg",
	},
	{
		ID: "inut_205_124", ClubID: "club_magic_arena_dn", ClubName: "Magic Arena Đà Nẵng", BoxName: "Magic Arena Đà Nẵng - Box 1",
		Branding: "billiards", VPNIP: "77.88.205.124", LANIP: "192.168.1.124", VirtualIP: "192.168.1.124",
		SoCModel: "ARMv8 Cortex-A53", Arch: "aarch64", RAMTotalMB: 1945, RAMAvailableMB: 910, ZRAMTotalMB: 1945, ZRAMAlgo: "lz4",
		SDTotalGB: 128, SDUsedPct: 55, SDFSType: "f2fs", SDMountOpts: "rw,noatime,errors=continue,users",
		CamerasCount: 16, Vendor: "Dahua", Resolution: "2560x1440", Width: 2560, Height: 1440, FPS: 30, BitrateKbps: 1536, Codec: "H.265",
		NVRHost: "192.168.1.250", ShinobiGroupKey: "magic1", StoreDomain: "store-magicdn1.video.io.vn", CStoreDomain: "c-store-magicdn1.video.io.vn",
		PM2CameraMB: 250, HardeningScore: 8, Notes: "Magic Arena Đà Nẵng Box 1, 32/32 FFmpeg",
	},
	{
		ID: "inut_202_213", ClubID: "club_nui_thanh_billiards", ClubName: "Núi Thành Billiards Center", BoxName: "Núi Thành Billiards Center",
		Branding: "billiards", VPNIP: "77.88.202.213", LANIP: "192.168.1.254", VirtualIP: "192.168.1.254",
		SoCModel: "Cortex-A7 sunxi", Arch: "armv7l", RAMTotalMB: 1024, RAMAvailableMB: 520, ZRAMTotalMB: 1024, ZRAMAlgo: "lz4",
		SDTotalGB: 30, SDUsedPct: 68, SDFSType: "f2fs", SDMountOpts: "rw,noatime,errors=continue",
		CamerasCount: 4, Vendor: "Dahua", Resolution: "2880x1620", Width: 2880, Height: 1620, FPS: 30, BitrateKbps: 1536, Codec: "H.265",
		ShinobiGroupKey: "nuithanh", StoreDomain: "store-nuithanh.video.io.vn", CStoreDomain: "c-store-nuithanh.video.io.vn",
		PM2CameraMB: 160, HardeningScore: 8, Notes: "4 camera Dahua 3K, netwatch sysrq neutralized",
	},
	{
		ID: "inut_204_222", ClubID: "club_hieu_billiards", ClubName: "Hiếu Billiards Arena", BoxName: "Hiếu Billiards Arena",
		Branding: "billiards", VPNIP: "77.88.204.222", LANIP: "192.168.1.222", VirtualIP: "192.168.1.254",
		SoCModel: "Amlogic S912", Arch: "aarch64", RAMTotalMB: 1843, RAMAvailableMB: 910, ZRAMTotalMB: 1843, ZRAMAlgo: "lz4",
		SDTotalGB: 58, SDUsedPct: 64, SDFSType: "f2fs", SDMountOpts: "rw,noatime,errors=continue",
		CamerasCount: 6, Vendor: "Dahua", Resolution: "2880x1620", Width: 2880, Height: 1620, FPS: 30, BitrateKbps: 1536, Codec: "H.265",
		ShinobiGroupKey: "job", StoreDomain: "store-hieubilliards.video.io.vn", CStoreDomain: "c-store-hieubilliards.video.io.vn",
		PM2CameraMB: 180, HardeningScore: 8, Notes: "6 camera Dahua 3K, 12/12 FFmpeg",
	},
	{
		ID: "inut_204_161", ClubID: "club_ntv_billiards", ClubName: "NTV Billiards", BoxName: "NTV Billiards",
		Branding: "billiards", VPNIP: "77.88.204.161", LANIP: "192.168.1.161", VirtualIP: "192.168.1.254",
		SoCModel: "Amlogic S912", Arch: "aarch64", RAMTotalMB: 1843, RAMAvailableMB: 690, ZRAMTotalMB: 1843, ZRAMAlgo: "lz4",
		SDTotalGB: 117, SDUsedPct: 66, SDFSType: "f2fs", SDMountOpts: "rw,noatime,errors=continue",
		CamerasCount: 12, Vendor: "Dahua", Resolution: "2880x1620", Width: 2880, Height: 1620, FPS: 30, BitrateKbps: 1536, Codec: "H.265",
		ShinobiGroupKey: "ntv", StoreDomain: "store-ntv.video.io.vn", CStoreDomain: "c-store-ntv.video.io.vn",
		PM2CameraMB: 220, HardeningScore: 8, Notes: "12 camera Dahua 3K, eMMC swapfile removed",
	},
	{
		ID: "inut_202_226", ClubID: "club_queen_billiards", ClubName: "Queen Billiards Club", BoxName: "Queen Billiards Club",
		Branding: "billiards", VPNIP: "77.88.202.226", LANIP: "192.168.1.254", VirtualIP: "192.168.1.254",
		SoCModel: "Cortex-A7 sunxi", Arch: "armv7l", RAMTotalMB: 1024, RAMAvailableMB: 650, ZRAMTotalMB: 1024, ZRAMAlgo: "lz4",
		SDTotalGB: 59, SDUsedPct: 55, SDFSType: "f2fs", SDMountOpts: "rw,noatime,errors=continue",
		CamerasCount: 5, Vendor: "Dahua", Resolution: "1920x1080", Width: 1920, Height: 1080, FPS: 30, BitrateKbps: 1536, Codec: "H.265",
		ShinobiGroupKey: "giaphong", StoreDomain: "store-queen.video.io.vn", CStoreDomain: "c-store-queen.video.io.vn",
		PM2CameraMB: 160, HardeningScore: 8, Notes: "MariaDB innodb_use_native_aio=0 fix applied",
	},
	{
		ID: "inut_205_120", ClubID: "club_vien_dong_pickleball", ClubName: "Viễn Đông Pickleball", BoxName: "Viễn Đông Pickleball",
		Branding: "pickleball", VPNIP: "77.88.205.120", LANIP: "192.168.20.204", VirtualIP: "192.168.20.252",
		SoCModel: "Allwinner H616", Arch: "aarch64", RAMTotalMB: 1945, RAMAvailableMB: 820, ZRAMTotalMB: 1945, ZRAMAlgo: "lz4",
		SDTotalGB: 59, SDUsedPct: 67, SDFSType: "f2fs", SDMountOpts: "rw,noatime,errors=continue",
		CamerasCount: 4, Vendor: "Dahua", Resolution: "2880x1620", Width: 2880, Height: 1620, FPS: 30, BitrateKbps: 1536, Codec: "H.265",
		ShinobiGroupKey: "tanminh", StoreDomain: "store-viendong.video.io.vn", CStoreDomain: "c-store-viendong.video.io.vn",
		PM2CameraMB: 180, HardeningScore: 8, Notes: "4 sân Pickleball 3K, 8/8 FFmpeg",
	},
	{
		ID: "inut_205_55", ClubID: "club_tuyet_linh_billiards", ClubName: "Tuyết Linh Billiards Club", BoxName: "Tuyết Linh Billiards Club",
		Branding: "billiards", VPNIP: "77.88.205.55", LANIP: "192.168.1.155", VirtualIP: "192.168.1.254",
		SoCModel: "Allwinner H616", Arch: "aarch64", RAMTotalMB: 1945, RAMAvailableMB: 960, ZRAMTotalMB: 1945, ZRAMAlgo: "lz4",
		SDTotalGB: 60, SDUsedPct: 63, SDFSType: "f2fs", SDMountOpts: "rw,noatime,errors=continue",
		CamerasCount: 11, Vendor: "Dahua", Resolution: "1920x1080", Width: 1920, Height: 1080, FPS: 30, BitrateKbps: 1536, Codec: "H.265",
		ShinobiGroupKey: "tuyetlinh", StoreDomain: "store-tuyetlinh.video.io.vn", CStoreDomain: "c-store-tuyetlinh.video.io.vn",
		PM2CameraMB: 210, HardeningScore: 8, Notes: "11 camera Dahua, ZRAM 100% unconstrained",
	},
	{
		ID: "inut_204_53", ClubID: "club_city_billiards_tn", ClubName: "City Billiards Club TN - CS3", BoxName: "City Billiards Club TN - CS3",
		Branding: "billiards", VPNIP: "77.88.204.53", LANIP: "192.168.1.150", VirtualIP: "192.168.1.254",
		SoCModel: "Rockchip RK3528", Arch: "aarch64", RAMTotalMB: 3993, RAMAvailableMB: 2710, ZRAMTotalMB: 3891, ZRAMAlgo: "lz4",
		SDTotalGB: 239, SDUsedPct: 50, SDFSType: "f2fs", SDMountOpts: "rw,noatime,errors=continue",
		CamerasCount: 16, Vendor: "Dahua", Resolution: "2880x1620", Width: 2880, Height: 1620, FPS: 30, BitrateKbps: 1536, Codec: "H.265",
		ShinobiGroupKey: "4953", StoreDomain: "store-citytn3.video.io.vn", CStoreDomain: "c-store-citytn3.video.io.vn",
		PM2CameraMB: 260, HardeningScore: 8, Notes: "16 camera Dahua 3K, 239GB F2FS",
	},
	{
		ID: "inut_205_60", ClubID: "club_bata_billiards", ClubName: "Bata Billiards Club", BoxName: "Bata Billiards Club",
		Branding: "billiards", VPNIP: "77.88.205.60", LANIP: "192.168.1.160", VirtualIP: "192.168.1.254",
		SoCModel: "Allwinner H616", Arch: "aarch64", RAMTotalMB: 1945, RAMAvailableMB: 850, ZRAMTotalMB: 1945, ZRAMAlgo: "lz4",
		SDTotalGB: 58, SDUsedPct: 42, SDFSType: "f2fs", SDMountOpts: "rw,noatime,errors=continue",
		CamerasCount: 11, Vendor: "Hikvision", Resolution: "1920x1080", Width: 1920, Height: 1080, FPS: 30, BitrateKbps: 1536, Codec: "H.265",
		ShinobiGroupKey: "bata", StoreDomain: "store-bata.video.io.vn", CStoreDomain: "c-store-bata.video.io.vn",
		PM2CameraMB: 210, HardeningScore: 8, Notes: "11 camera Hikvision 1080p H.265",
	},
	{
		ID: "inut_204_38", ClubID: "club_top_billiards_tn", ClubName: "Top Billiards Thái Nguyên", BoxName: "Top Billiards Thái Nguyên - Box 1",
		Branding: "billiards", VPNIP: "77.88.204.38", LANIP: "192.168.1.133", VirtualIP: "192.168.1.133",
		SoCModel: "Rockchip RK3528", Arch: "aarch64", RAMTotalMB: 3993, RAMAvailableMB: 2620, ZRAMTotalMB: 3891, ZRAMAlgo: "lz4",
		SDTotalGB: 115, SDUsedPct: 58, SDFSType: "f2fs", SDMountOpts: "rw,noatime,errors=continue,users",
		CamerasCount: 13, Vendor: "Dahua", Resolution: "1920x1080", Width: 1920, Height: 1080, FPS: 30, BitrateKbps: 1536, Codec: "H.265",
		NVRHost: "192.168.1.73", ShinobiGroupKey: "ux0pAUigSn", StoreDomain: "store-toptn1.video.io.vn", CStoreDomain: "c-store-toptn1.video.io.vn",
		PM2CameraMB: 230, HardeningScore: 8, Notes: "Bàn 01..13 qua NVR 192.168.1.73",
	},
	{
		ID: "inut_204_83", ClubID: "club_top_billiards_tn", ClubName: "Top Billiards Thái Nguyên", BoxName: "Top Billiards Thái Nguyên - Box 2",
		Branding: "billiards", VPNIP: "77.88.204.83", LANIP: "192.168.1.239", VirtualIP: "192.168.1.239",
		SoCModel: "Rockchip RK3528", Arch: "aarch64", RAMTotalMB: 3993, RAMAvailableMB: 2650, ZRAMTotalMB: 3891, ZRAMAlgo: "lz4",
		SDTotalGB: 58, SDUsedPct: 62, SDFSType: "f2fs", SDMountOpts: "rw,noatime,errors=continue,users",
		CamerasCount: 13, Vendor: "Dahua", Resolution: "1920x1080", Width: 1920, Height: 1080, FPS: 30, BitrateKbps: 1536, Codec: "H.265",
		NVRHost: "192.168.1.73", ShinobiGroupKey: "YZT7lO2CzM", StoreDomain: "store-toptn2.video.io.vn", CStoreDomain: "c-store-toptn2.video.io.vn",
		PM2CameraMB: 230, HardeningScore: 8, Notes: "Bàn 14..26 qua NVR 192.168.1.73",
	},
	{
		ID: "inut_204_165", ClubID: "club_nguyen_hue_pickleball", ClubName: "Nguyễn Huệ Pickleball", BoxName: "Nguyễn Huệ Pickleball",
		Branding: "pickleball", VPNIP: "77.88.204.165", LANIP: "192.168.1.10", VirtualIP: "192.168.1.254",
		SoCModel: "Amlogic S912", Arch: "aarch64", RAMTotalMB: 1843, RAMAvailableMB: 990, ZRAMTotalMB: 1843, ZRAMAlgo: "lz4",
		SDTotalGB: 58, SDUsedPct: 54, SDFSType: "f2fs", SDMountOpts: "rw,noatime,errors=continue",
		CamerasCount: 4, Vendor: "Dahua", Resolution: "2688x1664", Width: 2688, Height: 1664, FPS: 30, BitrateKbps: 1536, Codec: "H.265",
		ShinobiGroupKey: "KT8zqe2yGq", StoreDomain: "store-nguyenhue.video.io.vn", CStoreDomain: "c-store-nguyenhue.video.io.vn",
		PM2CameraMB: 180, HardeningScore: 8, Notes: "4 sân Pickleball 4MP 2688x1664",
	},
	{
		ID: "inut_207_18", ClubID: "club_tri_nguyen_cafe", ClubName: "Câu Lạc Bộ Trí Nguyễn Café", BoxName: "Câu Lạc Bộ Trí Nguyễn Café",
		Branding: "billiards", VPNIP: "77.88.207.18", LANIP: "192.168.1.5", VirtualIP: "192.168.1.252",
		SoCModel: "Cortex-A7 sunxi", Arch: "armv7l", RAMTotalMB: 1945, RAMAvailableMB: 1120, ZRAMTotalMB: 1945, ZRAMAlgo: "lz4",
		SDTotalGB: 58, SDUsedPct: 60, SDFSType: "f2fs", SDMountOpts: "rw,noatime,errors=continue",
		CamerasCount: 10, Vendor: "Dahua", Resolution: "2880x1620", Width: 2880, Height: 1620, FPS: 30, BitrateKbps: 1536, Codec: "H.265",
		ShinobiGroupKey: "tringuyen", StoreDomain: "store-tringuyen.video.io.vn", CStoreDomain: "c-store-tringuyen.video.io.vn",
		PM2CameraMB: 210, HardeningScore: 8, Notes: "10 camera Dahua 3K, 20/20 FFmpeg",
	},
	{
		ID: "inut_205_57", ClubID: "club_zone_plus_tn", ClubName: "Zone+ Thái Nguyên", BoxName: "Zone+ Thái Nguyên",
		Branding: "billiards", VPNIP: "77.88.205.57", LANIP: "192.168.1.240", VirtualIP: "192.168.1.240",
		SoCModel: "Allwinner H616", Arch: "aarch64", RAMTotalMB: 1945, RAMAvailableMB: 920, ZRAMTotalMB: 1945, ZRAMAlgo: "lz4",
		SDTotalGB: 128, SDUsedPct: 48, SDFSType: "f2fs", SDMountOpts: "rw,noatime,errors=continue",
		CamerasCount: 12, Vendor: "Dahua", Resolution: "2304x1296", Width: 2304, Height: 1296, FPS: 30, BitrateKbps: 1536, Codec: "H.265",
		ShinobiGroupKey: "moi", StoreDomain: "store-zoneplus.video.io.vn", CStoreDomain: "c-store-zoneplus.video.io.vn",
		PM2CameraMB: 220, HardeningScore: 8, Notes: "12 camera Dahua 2K, 128GB F2FS",
	},
	{
		ID: "inut_205_31", ClubID: "club_khet_billiards", ClubName: "Khét Billiards Club", BoxName: "Khét Billiards Club",
		Branding: "billiards", VPNIP: "77.88.205.31", LANIP: "192.168.1.254", VirtualIP: "192.168.1.254",
		SoCModel: "ARMv8 Cortex-A53", Arch: "aarch64", RAMTotalMB: 1945, RAMAvailableMB: 980, ZRAMTotalMB: 1945, ZRAMAlgo: "lz4",
		SDTotalGB: 58, SDUsedPct: 57, SDFSType: "f2fs", SDMountOpts: "rw,noatime,errors=continue",
		CamerasCount: 6, Vendor: "Dahua", Resolution: "1920x1080", Width: 1920, Height: 1080, FPS: 30, BitrateKbps: 1536, Codec: "H.265",
		NVRHost: "192.168.1.212", DoubleStreamIDs: []int{1},
		ShinobiGroupKey: "khet", StoreDomain: "store-khet.video.io.vn", CStoreDomain: "c-store-khet.video.io.vn",
		PM2CameraMB: 170, HardeningScore: 8, Notes: "6 camera qua NVR 192.168.1.212",
	},
	{
		ID: "inut_205_24", ClubID: "club_zone9_billiards", ClubName: "Zone 9 Billiards - 2", BoxName: "Zone 9 Billiards - 2",
		Branding: "billiards", VPNIP: "77.88.205.24", LANIP: "192.168.1.224", VirtualIP: "192.168.1.254",
		SoCModel: "Allwinner H616", Arch: "aarch64", RAMTotalMB: 1945, RAMAvailableMB: 950, ZRAMTotalMB: 1945, ZRAMAlgo: "lz4",
		SDTotalGB: 119, SDUsedPct: 68, SDFSType: "f2fs", SDMountOpts: "rw,noatime,errors=continue,users",
		CamerasCount: 11, Vendor: "Dahua", Resolution: "1920x1080", Width: 1920, Height: 1080, FPS: 30, BitrateKbps: 1536, Codec: "H.265",
		ShinobiGroupKey: "kao", StoreDomain: "store-zone9.video.io.vn", CStoreDomain: "c-store-zone9.video.io.vn",
		PM2CameraMB: 210, HardeningScore: 8, Notes: "11 camera, 101GB video preserved with errors=continue",
	},
	{
		ID: "inut_204_220", ClubID: "club_khanh_ha_billiards", ClubName: "Khánh Hà Billiards", BoxName: "Khánh Hà Billiards",
		Branding: "billiards", VPNIP: "77.88.204.220", LANIP: "192.168.1.220", VirtualIP: "192.168.1.254",
		SoCModel: "Rockchip ARM64", Arch: "aarch64", RAMTotalMB: 1843, RAMAvailableMB: 910, ZRAMTotalMB: 1843, ZRAMAlgo: "lz4",
		SDTotalGB: 115, SDUsedPct: 64, SDFSType: "ext4", SDMountOpts: "rw,noatime,errors=continue,users",
		CamerasCount: 10, Vendor: "Dahua", Resolution: "1920x1080", Width: 1920, Height: 1080, FPS: 30, BitrateKbps: 1536, Codec: "H.265",
		ShinobiGroupKey: "khanhha2", StoreDomain: "store-khanhha2.video.io.vn", CStoreDomain: "c-store-khanhha2.video.io.vn",
		PM2CameraMB: 210, HardeningScore: 8, Notes: "10 camera, 115GB ext4 errors=continue",
	},
	{
		ID: "inut_207_13", ClubID: "club_top_one_hp", ClubName: "TOP ONE / Axis Hải Phòng", BoxName: "TOP ONE / Axis Hải Phòng",
		Branding: "billiards", VPNIP: "77.88.207.13", LANIP: "192.168.1.213", VirtualIP: "192.168.1.254",
		SoCModel: "Rockchip Cortex-A55", Arch: "aarch64", RAMTotalMB: 3276, RAMAvailableMB: 2048, ZRAMTotalMB: 2457, ZRAMAlgo: "zstd",
		SDTotalGB: 117, SDUsedPct: 39, SDFSType: "f2fs", SDMountOpts: "rw,noatime,errors=continue,users",
		CamerasCount: 16, Vendor: "Dahua", Resolution: "1920x1080", Width: 1920, Height: 1080, FPS: 30, BitrateKbps: 1536, Codec: "H.265",
		ShinobiGroupKey: "hk", StoreDomain: "store-toponehp.video.io.vn", CStoreDomain: "c-store-toponehp.video.io.vn",
		PM2CameraMB: 250, HardeningScore: 8, Notes: "16 camera, Storage Guard trimmed 97% -> 39%",
	},
	{
		ID: "inut_205_73", ClubID: "club_pika_sun_pickleball", ClubName: "Pika Sun Pickleball", BoxName: "Pika Sun Pickleball",
		Branding: "pickleball", VPNIP: "77.88.205.73", LANIP: "192.168.1.173", VirtualIP: "192.168.1.254",
		SoCModel: "Allwinner H616", Arch: "aarch64", RAMTotalMB: 1945, RAMAvailableMB: 1331, ZRAMTotalMB: 1945, ZRAMAlgo: "lz4",
		SDTotalGB: 58, SDUsedPct: 52, SDFSType: "f2fs", SDMountOpts: "rw,noatime,errors=continue,users",
		CamerasCount: 6, Vendor: "Dahua", Resolution: "1920x1080", Width: 1920, Height: 1080, FPS: 30, BitrateKbps: 1536, Codec: "H.265",
		ShinobiGroupKey: "arenaTuoIL", StoreDomain: "store-pika.video.io.vn", CStoreDomain: "c-store-pika.video.io.vn",
		PM2CameraMB: 170, HardeningScore: 8, Notes: "Fixed FRPC subdomain conflict store-lionking-2 -> store-pika",
	},
	{
		ID: "inut_207_44", ClubID: "club_miapool_arena", ClubName: "MIAPool Arena", BoxName: "MIAPool Arena",
		Branding: "billiards", VPNIP: "77.88.207.44", LANIP: "192.168.1.44", VirtualIP: "192.168.1.254",
		SoCModel: "Rockchip ARMv7l", Arch: "armv7l", RAMTotalMB: 1945, RAMAvailableMB: 1126, ZRAMTotalMB: 1945, ZRAMAlgo: "lz4",
		SDTotalGB: 58, SDUsedPct: 58, SDFSType: "f2fs", SDMountOpts: "rw,noatime,errors=continue,users",
		CamerasCount: 13, Vendor: "Dahua", Resolution: "2880x1620", Width: 2880, Height: 1620, FPS: 30, BitrateKbps: 1536, Codec: "H.265",
		NVRHost: "192.168.1.88", DoubleStreamIDs: []int{7, 8},
		ShinobiGroupKey: "mia", StoreDomain: "store-miapool.video.io.vn", CStoreDomain: "c-store-miapool.video.io.vn",
		PM2CameraMB: 230, HardeningScore: 8, Notes: "13 camera 30fps 1536kbps CBR, RX ~10 Mbps",
	},
	{
		ID: "inut_204_167", ClubID: "club_onepiece_billiards", ClubName: "OnePiece Billiards", BoxName: "OnePiece",
		Branding: "billiards", VPNIP: "77.88.204.167", LANIP: "192.168.1.167", VirtualIP: "192.168.1.254",
		SoCModel: "Rockchip ARM64", Arch: "aarch64", RAMTotalMB: 1843, RAMAvailableMB: 972, ZRAMTotalMB: 1843, ZRAMAlgo: "zstd",
		SDTotalGB: 58, SDUsedPct: 68, SDFSType: "f2fs", SDMountOpts: "rw,noatime,errors=continue,users",
		CamerasCount: 6, Vendor: "Dahua", Resolution: "2304x1296", Width: 2304, Height: 1296, FPS: 30, BitrateKbps: 1536, Codec: "H.265",
		ShinobiGroupKey: "3ejN83HASs", StoreDomain: "store-onepiece.video.io.vn", CStoreDomain: "c-store-onepiece.video.io.vn",
		PM2CameraMB: 280, HardeningScore: 8, Notes: "6 camera Dahua 2K iso5 fragmented MP4",
	},
	{
		ID: "inut_204_155", ClubID: "club_bao_anh_2", ClubName: "Bảo Anh Billiards Club", BoxName: "Bảo Anh Billiards Club",
		Branding: "billiards", VPNIP: "77.88.204.155", LANIP: "192.168.1.155", VirtualIP: "192.168.1.254",
		SoCModel: "Rockchip ARM64", Arch: "aarch64", RAMTotalMB: 1794, RAMAvailableMB: 824, ZRAMTotalMB: 897, ZRAMAlgo: "zstd",
		SDTotalGB: 58, SDUsedPct: 70, SDFSType: "f2fs", SDMountOpts: "rw,noatime,users",
		CamerasCount: 8, Vendor: "Dahua", Resolution: "1920x1080", Width: 1920, Height: 1080, FPS: 30, BitrateKbps: 1536, Codec: "H.265",
		ShinobiGroupKey: "baoanh2", StoreDomain: "store-baoanh2.video.io.vn", CStoreDomain: "c-store-baoanh2.video.io.vn",
		PM2CameraMB: 200, HardeningScore: 8, Notes: "8 camera Dahua, fixed media-automount@.service Type=oneshot, 16/16 FFmpeg",
	},
	{
		ID: "inut_204_48", ClubID: "club_job_billiards", ClubName: "Job Billiards", BoxName: "Job Billiards",
		Branding: "billiards", VPNIP: "77.88.204.48", LANIP: "192.168.1.48", VirtualIP: "192.168.1.254",
		SoCModel: "Rockchip RK322x", Arch: "armv7l", RAMTotalMB: 1945, RAMAvailableMB: 1331, ZRAMTotalMB: 1945, ZRAMAlgo: "lz4",
		SDTotalGB: 58, SDUsedPct: 72, SDFSType: "ext4", SDMountOpts: "rw,noatime,errors=continue,users",
		CamerasCount: 13, Vendor: "Hikvision", Resolution: "1920x1080", Width: 1920, Height: 1080, FPS: 30, BitrateKbps: 1536, Codec: "H.265",
		ShinobiGroupKey: "job", StoreDomain: "store-job.video.io.vn", CStoreDomain: "c-store-job.video.io.vn",
		PM2CameraMB: 250, HardeningScore: 8, Notes: "13 camera Hikvision, ext4 SD card, fixed videos.js file.destroy() FD leak, 26/26 FFmpeg",
	},
	{
		ID: "inut_205_50", ClubID: "club_skyline_pool", ClubName: "Skyline Pool Club", BoxName: "Skyline Pool Club - Box 1",
		Branding: "billiards", VPNIP: "77.88.205.50", LANIP: "192.168.1.254", VirtualIP: "192.168.1.254",
		SoCModel: "Allwinner H616", Arch: "aarch64", RAMTotalMB: 1945, RAMAvailableMB: 814, ZRAMTotalMB: 1945, ZRAMAlgo: "lz4",
		SDTotalGB: 119, SDUsedPct: 69, SDFSType: "f2fs", SDMountOpts: "rw,noatime,errors=continue,users",
		CamerasCount: 9, Vendor: "Dahua", Resolution: "2560x1440", Width: 2560, Height: 1440, FPS: 30, BitrateKbps: 1536, Codec: "H.265",
		NVRHost: "192.168.1.246",
		ShinobiGroupKey: "sun", StoreDomain: "store-skyline.video.io.vn", CStoreDomain: "c-store-skyline.video.io.vn",
		PM2CameraMB: 220, HardeningScore: 8, Notes: "9 camera Dahua, NVR 192.168.1.246 (DHI-NVR1108HS-S3/H-VN) rebooted, 18/18 FFmpeg",
	},
	{
		ID: "inut_205_54", ClubID: "club_skyline_pool", ClubName: "Skyline Pool Club", BoxName: "Skyline Pool Club - Box 2",
		Branding: "billiards", VPNIP: "77.88.205.54", LANIP: "192.168.1.254", VirtualIP: "192.168.1.254",
		SoCModel: "Allwinner H616", Arch: "aarch64", RAMTotalMB: 1945, RAMAvailableMB: 1126, ZRAMTotalMB: 1945, ZRAMAlgo: "lz4",
		SDTotalGB: 58, SDUsedPct: 76, SDFSType: "f2fs", SDMountOpts: "rw,noatime,errors=continue,users",
		CamerasCount: 9, Vendor: "Dahua", Resolution: "2560x1440", Width: 2560, Height: 1440, FPS: 30, BitrateKbps: 1536, Codec: "H.265",
		NVRHost: "192.168.1.200",
		ShinobiGroupKey: "ultrapool2", StoreDomain: "store-skyline2.video.io.vn", CStoreDomain: "c-store-skyline2.video.io.vn",
		PM2CameraMB: 220, HardeningScore: 8, Notes: "9 camera Dahua, NVR 192.168.1.200 (DH-NVR2108HS-4KS2) rebooted, 18/18 FFmpeg",
	},
	{
		ID: "inut_204_135", ClubID: "club_htp_pickleball", ClubName: "HTP Pickleball", BoxName: "HTP Pickleball",
		Branding: "pickleball", VPNIP: "77.88.204.135", LANIP: "192.168.1.253", VirtualIP: "192.168.1.253",
		SoCModel: "Allwinner H616", Arch: "aarch64", RAMTotalMB: 1945, RAMAvailableMB: 620, ZRAMTotalMB: 1945, ZRAMAlgo: "lz4",
		SDTotalGB: 58, SDUsedPct: 68, SDFSType: "f2fs", SDMountOpts: "rw,noatime,errors=continue,users",
		CamerasCount: 6, Vendor: "Hikvision", Resolution: "2560x1440", Width: 2560, Height: 1440, FPS: 20, BitrateKbps: 1536, Codec: "H.265",
		ShinobiGroupKey: "b389", StoreDomain: "store-htppickleball.video.io.vn", CStoreDomain: "c-store-htppickleball.video.io.vn",
		PM2CameraMB: 220, HardeningScore: 8, Notes: "6 camera Hikvision 2K, enabled AAC audio + 2.5h (range=2.5) clip download range, 12/12 FFmpeg",
	},
	{
		ID: "inut_205_75", ClubID: "club_49_billiards_ninh_hoa", ClubName: "49 Billiards Carom Club Ninh Hòa", BoxName: "49 Billiards Carom Club Ninh Hòa",
		Branding: "billiards", VPNIP: "77.88.205.75", LANIP: "192.168.1.254", VirtualIP: "192.168.1.254",
		SoCModel: "Allwinner H616", Arch: "aarch64", RAMTotalMB: 1945, RAMAvailableMB: 846, ZRAMTotalMB: 1945, ZRAMAlgo: "lz4",
		SDTotalGB: 118, SDUsedPct: 59, SDFSType: "exfat", SDMountOpts: "rw,noatime,errors=continue,users",
		CamerasCount: 10, Vendor: "Dahua", Resolution: "1920x1080", Width: 1920, Height: 1080, FPS: 25, BitrateKbps: 1536, Codec: "H.265",
		ShinobiGroupKey: "49", StoreDomain: "store-49-2.video.io.vn", CStoreDomain: "c-store-49-2.video.io.vn",
		PM2CameraMB: 230, HardeningScore: 8, Notes: "10 camera Dahua 1080p, 118GB exFAT SD card, 20/20 FFmpeg",
	},
	{
		ID: "inut_204_2", ClubID: "club_dragon_billiards_bn", ClubName: "Dragon Billiards Club", BoxName: "Dragon Billiards Club",
		Branding: "billiards", VPNIP: "77.88.204.2", LANIP: "192.168.1.254", VirtualIP: "192.168.1.254",
		SoCModel: "Rockchip RK3318", Arch: "aarch64", RAMTotalMB: 3974, RAMAvailableMB: 2834, ZRAMTotalMB: 2457, ZRAMAlgo: "zstd",
		SDTotalGB: 238, SDUsedPct: 84, SDFSType: "btrfs", SDMountOpts: "rw,noatime,ssd,space_cache=v2",
		CamerasCount: 11, Vendor: "Xiongmai/Dahua", Resolution: "1920x1080", Width: 1920, Height: 1080, FPS: 25, BitrateKbps: 1536, Codec: "H.264",
		NVRHost: "192.168.1.93",
		ShinobiGroupKey: "Fgbghldvgk", StoreDomain: "store-dragon-bn.video.io.vn", CStoreDomain: "c-store-dragon-bn.video.io.vn",
		PM2CameraMB: 280, HardeningScore: 8, Notes: "11 NVR channels on 192.168.1.93:34567, 239GB btrfs SD card, fixed offline Ch1 snapshot worker leak, 22 FFmpeg",
	},
	{
		ID: "inut_205_52", ClubID: "club_genz_hn7", ClubName: "Gen Z Billiards 375 Vũ Tông Phan", BoxName: "Gen Z Billiards 375 Vũ Tông Phan",
		Branding: "billiards", VPNIP: "77.88.205.52", LANIP: "192.168.1.254", VirtualIP: "192.168.1.254",
		SoCModel: "Allwinner H616", Arch: "aarch64", RAMTotalMB: 1945, RAMAvailableMB: 700, ZRAMTotalMB: 1945, ZRAMAlgo: "lz4",
		SDTotalGB: 59, SDUsedPct: 68, SDFSType: "f2fs", SDMountOpts: "rw,noatime,nodiscard,errors=continue,users",
		CamerasCount: 12, Vendor: "Dahua", Resolution: "1920x1080", Width: 1920, Height: 1080, FPS: 25, BitrateKbps: 1536, Codec: "H.265",
		NVRHost: "192.168.1.99",
		ShinobiGroupKey: "hn7", StoreDomain: "store-genz-hn-7.video.io.vn", CStoreDomain: "c-store-genz-hn-7.video.io.vn",
		PM2CameraMB: 260, HardeningScore: 8, Notes: "12 camera Dahua + NVR 192.168.1.99, revived errored camera.js, 24/24 FFmpeg",
	},
	{
		ID: "inut_205_59", ClubID: "club_tuan_king", ClubName: "Tuấn King Billiards Club", BoxName: "Tuấn King Billiards Club",
		Branding: "billiards", VPNIP: "77.88.205.59", LANIP: "192.168.1.254", VirtualIP: "192.168.1.254",
		SoCModel: "Allwinner H616", Arch: "aarch64", RAMTotalMB: 1945, RAMAvailableMB: 847, ZRAMTotalMB: 1945, ZRAMAlgo: "lz4",
		SDTotalGB: 58, SDUsedPct: 3, SDFSType: "f2fs", SDMountOpts: "rw,noatime,nodiscard,errors=continue,users",
		CamerasCount: 10, Vendor: "Dahua", Resolution: "1920x1080", Width: 1920, Height: 1080, FPS: 25, BitrateKbps: 1536, Codec: "H.265",
		ShinobiGroupKey: "tuancreep", StoreDomain: "store-tuanking.video.io.vn", CStoreDomain: "c-store-tuanking.video.io.vn",
		PM2CameraMB: 230, HardeningScore: 8, Notes: "10 camera Dahua, revived errored camera.js, 20/20 FFmpeg",
	},
	{
		ID: "inut_205_110", ClubID: "club_99_cs9_2", ClubName: "99 CS9 Billiards - 2", BoxName: "99 CS9 Billiards - 2",
		Branding: "billiards", VPNIP: "77.88.205.110", LANIP: "192.168.11.254", VirtualIP: "192.168.11.254",
		SoCModel: "Allwinner H616", Arch: "aarch64", RAMTotalMB: 1945, RAMAvailableMB: 445, ZRAMTotalMB: 1945, ZRAMAlgo: "lz4",
		SDTotalGB: 117, SDUsedPct: 69, SDFSType: "f2fs", SDMountOpts: "rw,noatime,nodiscard,errors=continue,users",
		CamerasCount: 15, Vendor: "Hikvision", Resolution: "1920x1080", Width: 1920, Height: 1080, FPS: 25, BitrateKbps: 1536, Codec: "H.265",
		ShinobiGroupKey: "9992", StoreDomain: "store-99-cs9-2.video.io.vn", CStoreDomain: "c-store-99-cs9-2.video.io.vn",
		PM2CameraMB: 260, HardeningScore: 8, Notes: "15 camera Hikvision, fixed c-store-99-cs96-2 typo -> c-store-99-cs9-2, revived errored camera.js, 30/30 FFmpeg",
	},
	{
		ID: "inut_204_133", ClubID: "club_zone_plus_1", ClubName: "Zone+ Thái Nguyên 1", BoxName: "Zone+ Thái Nguyên 1",
		Branding: "billiards", VPNIP: "77.88.204.133", LANIP: "192.168.1.254", VirtualIP: "192.168.1.254",
		SoCModel: "Allwinner H616", Arch: "aarch64", RAMTotalMB: 1945, RAMAvailableMB: 1024, ZRAMTotalMB: 1945, ZRAMAlgo: "lz4",
		SDTotalGB: 58, SDUsedPct: 41, SDFSType: "f2fs", SDMountOpts: "rw,noatime,nodiscard,errors=continue,users",
		CamerasCount: 10, Vendor: "Dahua", Resolution: "1920x1080", Width: 1920, Height: 1080, FPS: 25, BitrateKbps: 1536, Codec: "H.265",
		ShinobiGroupKey: "zoneplus1", StoreDomain: "store-zoneplus1.video.io.vn", CStoreDomain: "c-store-zoneplus1.video.io.vn",
		PM2CameraMB: 230, HardeningScore: 8, Notes: "10 camera, HLS live 20/20 FFmpeg active; SD card hardware dead on mmc0 (needs physical SD card replacement)",
	},
}

func randomHex(nBytes int, seedFallback string) string {
	buf := make([]byte, nBytes)
	if _, err := rand.Read(buf); err == nil {
		return hex.EncodeToString(buf)
	}
	sum := sha256.Sum256([]byte(seedFallback))
	if nBytes <= len(sum) {
		return hex.EncodeToString(sum[:nBytes])
	}
	sum2 := sha256.Sum256([]byte(seedFallback + ":2"))
	combined := append(sum[:], sum2[:]...)
	return hex.EncodeToString(combined[:nBytes])
}

// DefaultFleetCatalog builds the complete 35-box, 29-club catalog derived from FLEET_WATCHLIST.md.
func DefaultFleetCatalog() ([]*Club, []*Box) {
	return buildCatalogFromSpecs(builtInFleetSpecs)
}

func buildCatalogFromSpecs(specs []seedBoxSpec) ([]*Club, []*Box) {
	clubMap := make(map[string]*Club)
	var clubOrder []string
	var boxes []*Box
	now := time.Now().UTC()

	for idx, sp := range specs {
		club, exists := clubMap[sp.ClubID]
		if !exists {
			storeURL := "https://" + sp.StoreDomain
			cstoreURL := "https://" + sp.CStoreDomain
			portalToken := randomHex(16, "portal:"+sp.ClubID)
			dlHMACKey := randomHex(32, "hmac:"+sp.ClubID)
			club = &Club{
				ID:          sp.ClubID,
				Name:        sp.ClubName,
				Branding:    sp.Branding,
				PortalToken: portalToken,
				DLHMACKey:   dlHMACKey,
				StoreURL:    storeURL,
				CStoreURL:   cstoreURL,
				PortalURL:   "/portal/" + portalToken + "/",
				BoxIDs:      []string{},
				CreatedAt:   now,
			}
			clubMap[sp.ClubID] = club
			clubOrder = append(clubOrder, sp.ClubID)
		}

		box := materializeBox(sp, club, idx, now)
		boxes = append(boxes, box)
		club.BoxIDs = append(club.BoxIDs, box.ID)
		club.BoxesCount = len(club.BoxIDs)
		if box.Online {
			club.OnlineBoxesCount++
		}
		club.CamerasCount += box.CamerasCount
	}

	clubs := make([]*Club, 0, len(clubOrder))
	for _, cid := range clubOrder {
		clubs = append(clubs, clubMap[cid])
	}
	return clubs, boxes
}

func materializeBox(sp seedBoxSpec, club *Club, idx int, now time.Time) *Box {
	ramTotalMB := sp.RAMTotalMB
	if ramTotalMB <= 0 {
		ramTotalMB = 1945
	}
	ramAvailMB := sp.RAMAvailableMB
	if ramAvailMB <= 0 || ramAvailMB >= ramTotalMB {
		ramAvailMB = ramTotalMB / 2
	}
	ramUsedMB := ramTotalMB - ramAvailMB
	if ramUsedMB < 0 {
		ramUsedMB = ramTotalMB / 2
	}
	ramPct := 45
	if ramTotalMB > 0 {
		ramPct = (ramUsedMB * 100) / ramTotalMB
	}
	zramTotalMB := sp.ZRAMTotalMB
	if zramTotalMB <= 0 {
		zramTotalMB = 1945
	}
	zramUsedMB := 180 + (idx%7)*35
	if zramUsedMB > zramTotalMB {
		zramUsedMB = zramTotalMB / 4
	}
	swapTotalMB := zramTotalMB
	swapUsedMB := zramUsedMB
	swapFreeMB := swapTotalMB - swapUsedMB
	zramPct := 0
	if swapTotalMB > 0 {
		zramPct = (swapUsedMB * 100) / swapTotalMB
	}
	cpuCores := 4
	if strings.Contains(sp.SoCModel, "S912") {
		cpuCores = 8
	}
	cpuPct := float64(18+(idx%22)) + float64(idx%5)*0.2
	loadAvg1m := float64(int((0.85+float64(idx%9)*0.18)*100)) / 100.0
	heapAllocMB := float64(int((11.2+float64(idx%6)*0.7)*10)) / 10.0
	heapSysMB := float64(int((heapAllocMB*1.85)*10)) / 10.0
	kspcamRSSMB := float64(int((24.0+float64(idx%7)*1.4)*10)) / 10.0
	cameraRSSMB := float64(int((115.0+float64(sp.CamerasCount)*3.2)*10)) / 10.0
	goroutines := 24 + sp.CamerasCount*2
	uptimeDays := 3 + (idx % 14)
	uptimeHours := 2 + (idx % 18)
	uptimeSec := int64(uptimeDays*86400 + uptimeHours*3600 + 900)
	uptimeHuman := fmt.Sprintf("%dd %dh", uptimeDays, uptimeHours)
	sdUsedGB := (sp.SDTotalGB * sp.SDUsedPct) / 100
	ffmpegExpected := sp.CamerasCount * 2
	ffmpegCount := ffmpegExpected
	rttBase := 18 + (idx%17)*2
	rxMbps := float64(sp.CamerasCount) * float64(sp.BitrateKbps) / 1000.0 * 0.65
	doubleSet := make(map[int]bool)
	for _, d := range sp.DoubleStreamIDs {
		doubleSet[d] = true
	}

	cameras := make([]*Camera, 0, sp.CamerasCount)
	monitors := make([]*ShinobiMonitor, 0, sp.CamerasCount)
	tabs := make(map[string]string)

	unitLabel := "Bàn"
	if sp.Branding == "pickleball" {
		unitLabel = "Sân"
	}

	for c := 1; c <= sp.CamerasCount; c++ {
		camHost := fmt.Sprintf("192.168.1.%d", 100+c)
		if strings.HasPrefix(sp.LANIP, "172.10.") {
			camHost = fmt.Sprintf("172.10.17.%d", 100+c)
		} else if strings.HasPrefix(sp.LANIP, "192.168.20.") {
			camHost = fmt.Sprintf("192.168.20.%d", 200+c)
		}
		camID := fmt.Sprintf("%s:%s:37777", sp.ID, camHost)
		mid := fmt.Sprintf("camera%02d", c)
		tableName := fmt.Sprintf("%s %02d", unitLabel, c)
		isDouble := doubleSet[c]
		codec := sp.Codec
		bitrate := sp.BitrateKbps
		rcMode := "CBR"
		streamSource := "direct"
		if sp.NVRHost != "" {
			streamSource = "nvr"
		}
		if isDouble {
			streamSource = "direct+nvr"
			bitrate = 2048
		}
		cam := &Camera{
			ID:              camID,
			BoxID:           sp.ID,
			BoxName:         sp.BoxName,
			ClubID:          club.ID,
			ClubName:        club.Name,
			TableNo:         c,
			Name:            mid,
			TableName:       tableName,
			Vendor:          sp.Vendor,
			Host:            camHost,
			Port:            37777,
			RTSPPort:        554,
			NVRHost:         sp.NVRHost,
			NVRChannel:      c,
			Online:          true,
			Recording:       true,
			Codec:           codec,
			SmartCodec:      true,
			Resolution:      sp.Resolution,
			Width:           sp.Width,
			Height:          sp.Height,
			FPS:             sp.FPS,
			BitrateKbps:     bitrate,
			RCMode:          rcMode,
			GOP:             50,
			AudioCodec:      "AAC",
			DoubleStreaming: isDouble,
			StreamSource:    streamSource,
			LastProbedAt:    now,
		}
		cameras = append(cameras, cam)

		mon := &ShinobiMonitor{
			MID:                mid,
			BoxID:              sp.ID,
			Name:               tableName,
			Host:               camHost,
			Mode:               "record",
			Status:             "Watching",
			Codec:              "copy",
			ACodec:             "copy",
			Width:              sp.Width,
			Height:             sp.Height,
			CustRecord:         "-tag:v hvc1 -segment_format_options movflags=frag_keyframe+empty_moov+default_base_moof",
			InputMapChoicesOK:  true,
			DoSnapshotDisabled: true,
			FragMP4OK:          true,
		}
		monitors = append(monitors, mon)

		tabKey := fmt.Sprintf("C%02d", c)
		tabs[tabKey] = fmt.Sprintf("[%s]\nname=%s\nhost=%s\nchannel=%d\nrtsp=rtsp://%s:554/cam/realmonitor?channel=%d&subtype=0\n",
			tabKey, tableName, camHost, c, camHost, c)
	}
	for c := sp.CamerasCount + 1; c <= 20; c++ {
		tabKey := fmt.Sprintf("C%02d", c)
		tableName := fmt.Sprintf("%s %02d", unitLabel, c)
		camHost := fmt.Sprintf("192.168.1.%d", 100+c)
		tabs[tabKey] = fmt.Sprintf("[%s]\nname=%s\nhost=%s\nchannel=%d\nenabled=0\n",
			tabKey, tableName, camHost, c)
	}

	storeSlug := strings.TrimSuffix(sp.StoreDomain, ".video.io.vn")
	cstoreSlug := strings.TrimSuffix(sp.CStoreDomain, ".video.io.vn")
	frpcIni := fmt.Sprintf("[common]\nserver_addr = video.io.vn\nserver_port = 7000\n\n[%s]\ntype = http\nlocal_port = 2023\nsubdomain = %s\n\n[%s]\ntype = http\nlocal_port = 8080\nsubdomain = %s\n",
		storeSlug, storeSlug, cstoreSlug, cstoreSlug)

	ggcode := strings.ToUpper(sp.ShinobiGroupKey)
	if len(ggcode) > 8 {
		ggcode = ggcode[:8]
	}

	redbidaKeys := DefaultRedBidaKeys(sp.ClubName, sp.Branding, sp.VirtualIP, sp.ShinobiGroupKey, ggcode, frpcIni)

	box := &Box{
		ID:                       sp.ID,
		ClubID:                   club.ID,
		ClubName:                 club.Name,
		Name:                     sp.BoxName,
		VPNIP:                    sp.VPNIP,
		LANIP:                    sp.LANIP,
		VirtualIP:                sp.VirtualIP,
		SoCModel:                 sp.SoCModel,
		Arch:                     sp.Arch,
		CPUPct:                   cpuPct,
		LoadAvg1m:                loadAvg1m,
		CPUCores:                 cpuCores,
		TempC:                    48.5 + float64(idx%9),
		RAMTotalMB:               ramTotalMB,
		RAMUsedMB:                ramUsedMB,
		RAMAvailableMB:           ramAvailMB,
		RAMUsedPct:               ramPct,
		RAMPct:                   ramPct,
		HeapAllocMB:              heapAllocMB,
		HeapSysMB:                heapSysMB,
		KspcamRSSMB:              kspcamRSSMB,
		CameraRSSMB:              cameraRSSMB,
		Goroutines:               goroutines,
		SwapTotalMB:              swapTotalMB,
		SwapUsedMB:               swapUsedMB,
		SwapFreeMB:               swapFreeMB,
		ZRAMTotalMB:              zramTotalMB,
		ZRAMUsedMB:               zramUsedMB,
		ZRAMPct:                  zramPct,
		ZRAMAlgo:                 sp.ZRAMAlgo,
		ZRAMMemLimit:             0,
		Swappiness:               80,
		SDTotalGB:                sp.SDTotalGB,
		SDUsedGB:                 sdUsedGB,
		SDUsedPct:                sp.SDUsedPct,
		SDFSType:                 sp.SDFSType,
		SDMountOpts:              sp.SDMountOpts,
		SDDevice:                 "/dev/mmcblk0p1",
		CleanFstab:               true,
		UdevAutomount:            true,
		PM2CameraLimitMB:         sp.PM2CameraMB,
		PM2InutLimitMB:           280,
		PM2OtaLimitMB:            120,
		PM2LogsDevNull:           true,
		SysrqEnabled:             0,
		SysrqHooksRemoved:        true,
		CameraWatchdogActive:     true,
		StorageGuardActive:       true,
		ShinobiDoSnapshot:        false,
		ShinobiWorkerFix:         true,
		ShinobiFragMP4:           true,
		ShinobiGroupKey:          sp.ShinobiGroupKey,
		ShinobiAPIKey:            "api_" + sp.ShinobiGroupKey,
		ShinobiUserSizeMB:        100000,
		ShinobiMonitorsTotal:     sp.CamerasCount,
		ShinobiMonitorsRecording: sp.CamerasCount,
		CorruptedMP4Count:        0,
		CamerasCount:             sp.CamerasCount,
		FFmpegCount:              ffmpegCount,
		FFmpegExpected:           ffmpegExpected,
		RXMbps:                   float64(int(rxMbps*10)) / 10.0,
		StoreDomain:              sp.StoreDomain,
		CStoreDomain:             sp.CStoreDomain,
		StoreHTTPCode:            200,
		CStoreHTTPCode:           200,
		Online:                   true,
		TunnelActive:             false,
		Transport:                "vpn",
		RTTMs:                    rttBase,
		RTTHistory:               []int{rttBase + 2, rttBase - 1, rttBase + 1, rttBase, rttBase - 2, rttBase},
		HardeningScore:           8,
		HardeningMax:             8,
		RedBidaScore:             17,
		RedBidaMax:               17,
		Branding:                 sp.Branding,
		PortalToken:              club.PortalToken,
		GGCode:                   ggcode,
		FRPCConfig:               frpcIni,
		RedBidaKeys:              redbidaKeys,
		RedBidaTabs:              tabs,
		Cameras:                  cameras,
		Monitors:                 monitors,
		LastSeen:                 now,
		UptimeSec:                uptimeSec,
		UptimeHuman:              uptimeHuman,
		TelemetrySource:          "seed",
		Notes:                    sp.Notes,
	}

	if sp.HardeningScore < 8 && sp.HardeningScore > 0 {
		if sp.HardeningScore <= 7 {
			box.ShinobiDoSnapshot = true
			box.CorruptedMP4Count = 3
		}
		if sp.HardeningScore <= 6 {
			box.CleanFstab = false
		}
	}
	report := Build8StepHardening(box)
	box.HardeningScore = report.Score
	return box
}

var (
	reNodeHeader  = regexp.MustCompile(`(?m)^-\s*\[[xX ]\]\s*\*\*Node\s+\x60(inut_[0-9_]+)\x60\*\*\s*\(([^)]+)\)`)
	reMultiHeader = regexp.MustCompile(`(?m)^-\s*\[[xX ]\]\s*\*\*Các node\s+([^*]+)\*\*\s*\(([^)]+)\)`)
	reBoxIDToken  = regexp.MustCompile(`inut_[0-9]+_[0-9]+`)
	reIPv4Token   = regexp.MustCompile(`77\.88\.[0-9]+\.[0-9]+`)
	reGroupToken  = regexp.MustCompile(`group\s+\x60([^\x60]+)\x60`)
	reCamCount    = regexp.MustCompile(`(?i)(?:Số lượng camera| camera)[:\s]*([0-9]{1,2})\s*camera`)
)

// ParseWatchlist parses FLEET_WATCHLIST.md content into Clubs and Boxes.
// If markdown matches the repository FLEET_WATCHLIST.md or is empty, it merges with the full built-in catalog.
func ParseWatchlist(markdown string) ([]*Club, []*Box) {
	trimmed := strings.TrimSpace(markdown)
	if trimmed == "" {
		return DefaultFleetCatalog()
	}

	builtInByID := make(map[string]seedBoxSpec, len(builtInFleetSpecs))
	for _, sp := range builtInFleetSpecs {
		builtInByID[sp.ID] = sp
	}

	type parsedItem struct {
		id    string
		title string
		block string
	}
	var items []parsedItem
	seenID := make(map[string]bool)

	lines := strings.Split(markdown, "\n")
	var curID, curTitle string
	var curLines []string
	flush := func() {
		if curID != "" && !seenID[curID] {
			seenID[curID] = true
			items = append(items, parsedItem{
				id:    curID,
				title: curTitle,
				block: strings.Join(curLines, "\n"),
			})
		}
		curID = ""
		curTitle = ""
		curLines = nil
	}

	for _, line := range lines {
		if m := reNodeHeader.FindStringSubmatch(line); m != nil {
			flush()
			curID = m[1]
			curTitle = m[2]
			curLines = []string{line}
			continue
		}
		if m := reMultiHeader.FindStringSubmatch(line); m != nil {
			flush()
			groupTitle := strings.TrimSpace(m[1])
			ids := reBoxIDToken.FindAllString(m[2], -1)
			for idx, id := range ids {
				if !seenID[id] {
					seenID[id] = true
					items = append(items, parsedItem{
						id:    id,
						title: fmt.Sprintf("%s - Box %d", groupTitle, idx+1),
						block: line,
					})
				}
			}
			continue
		}
		if curID != "" {
			curLines = append(curLines, line)
		}
	}
	flush()

	if len(items) == 0 {
		return DefaultFleetCatalog()
	}

	var specs []seedBoxSpec
	specSeen := make(map[string]bool)

	for _, it := range items {
		if sp, ok := builtInByID[it.id]; ok {
			specs = append(specs, sp)
			specSeen[it.id] = true
			continue
		}
		// Dynamic fallback for custom test markdown nodes not in builtInByID
		sp := parseDynamicNodeSpec(it.id, it.title, it.block)
		specs = append(specs, sp)
		specSeen[it.id] = true
	}

	// If this is the full FLEET_WATCHLIST.md (or contains >15 nodes), ensure every built-in node is included
	if len(specs) >= 15 || strings.Contains(markdown, "FLEET WATCHLIST & HARDENING INVENTORY") {
		for _, sp := range builtInFleetSpecs {
			if !specSeen[sp.ID] {
				specs = append(specs, sp)
				specSeen[sp.ID] = true
			}
		}
	}

	return buildCatalogFromSpecs(specs)
}

func parseDynamicNodeSpec(boxID, headerParen, block string) seedBoxSpec {
	vpnIP := "77.88.204.100"
	if m := reIPv4Token.FindString(headerParen); m != "" {
		vpnIP = m
	} else {
		parts := strings.Split(boxID, "_")
		if len(parts) == 3 {
			vpnIP = fmt.Sprintf("77.88.%s.%s", parts[1], parts[2])
		}
	}

	groupKey := strings.ReplaceAll(boxID, "_", "")
	if m := reGroupToken.FindStringSubmatch(headerParen); len(m) > 1 {
		groupKey = m[1]
	}

	rawName := headerParen
	if idx := strings.Index(rawName, "—"); idx > 0 {
		rawName = strings.TrimSpace(rawName[:idx])
	} else if idx := strings.Index(rawName, ","); idx > 0 {
		rawName = strings.TrimSpace(rawName[:idx])
	}
	if rawName == "" {
		rawName = boxID
	}

	clubName := normalizeClubName(rawName)
	clubID := "club_" + slugify(clubName)
	branding := "billiards"
	if strings.Contains(strings.ToLower(clubName), "pickleball") || strings.Contains(strings.ToLower(block), "pickleball") {
		branding = "pickleball"
	}

	camsCount := 8
	if m := reCamCount.FindStringSubmatch(block); len(m) > 1 {
		if n, err := strconv.Atoi(m[1]); err == nil && n > 0 && n <= 32 {
			camsCount = n
		}
	}

	slug := slugify(rawName)
	return seedBoxSpec{
		ID:              boxID,
		ClubID:          clubID,
		ClubName:        clubName,
		BoxName:         rawName,
		Branding:        branding,
		VPNIP:           vpnIP,
		LANIP:           "192.168.1.10",
		VirtualIP:       "192.168.1.254",
		SoCModel:        "Allwinner H616",
		Arch:            "aarch64",
		RAMTotalMB:      1945,
		RAMAvailableMB:  1024,
		ZRAMTotalMB:     1945,
		ZRAMAlgo:        "lz4",
		SDTotalGB:       64,
		SDUsedPct:       52,
		SDFSType:        "f2fs",
		SDMountOpts:     "rw,noatime,errors=continue,users",
		CamerasCount:    camsCount,
		Vendor:          "Dahua",
		Resolution:      "1920x1080",
		Width:           1920,
		Height:          1080,
		FPS:             30,
		BitrateKbps:     1536,
		Codec:           "H.265",
		ShinobiGroupKey: groupKey,
		StoreDomain:     "store-" + slug + ".video.io.vn",
		CStoreDomain:    "c-store-" + slug + ".video.io.vn",
		PM2CameraMB:     220,
		HardeningScore:  8,
	}
}

func normalizeClubName(boxTitle string) string {
	reSuffix := regexp.MustCompile(`(?i)\s*-\s*(?:Box\s*\d+|\d+)\s*$`)
	cleaned := strings.TrimSpace(reSuffix.ReplaceAllString(boxTitle, ""))
	if cleaned == "" {
		return boxTitle
	}
	return cleaned
}

func slugify(s string) string {
	lower := strings.ToLower(s)
	replacer := strings.NewReplacer(
		"đ", "d", "ă", "a", "â", "a", "á", "a", "à", "a", "ả", "a", "ã", "a", "ạ", "a",
		"ắ", "a", "ằ", "a", "ẳ", "a", "ẵ", "a", "ặ", "a", "ấ", "a", "ầ", "a", "ẩ", "a", "ẫ", "a", "ậ", "a",
		"ê", "e", "é", "e", "è", "e", "ẻ", "e", "ẽ", "e", "ẹ", "e", "ế", "e", "ề", "e", "ể", "e", "ễ", "e", "ệ", "e",
		"í", "i", "ì", "i", "ỉ", "i", "ĩ", "i", "ị", "i",
		"ô", "o", "ơ", "o", "ó", "o", "ò", "o", "ỏ", "o", "õ", "o", "ọ", "o",
		"ố", "o", "ồ", "o", "ổ", "o", "ỗ", "o", "ộ", "o", "ớ", "o", "ờ", "o", "ở", "o", "ỡ", "o", "ợ", "o",
		"ư", "u", "ú", "u", "ù", "u", "ủ", "u", "ũ", "u", "ụ", "u", "ứ", "u", "ừ", "u", "ử", "u", "ữ", "u", "ự", "u",
		"ý", "y", "ỳ", "y", "ỷ", "y", "ỹ", "y", "ỵ", "y",
	)
	lower = replacer.Replace(lower)
	var b strings.Builder
	lastUnderscore := false
	for _, r := range lower {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
			lastUnderscore = false
		} else if !lastUnderscore {
			b.WriteByte('_')
			lastUnderscore = true
		}
	}
	res := strings.Trim(b.String(), "_")
	if res == "" {
		return "club"
	}
	return res
}

// LoadOrSeedWatchlist reads watchlistPath if readable; otherwise falls back to DefaultFleetCatalog.
func LoadOrSeedWatchlist(watchlistPath string) ([]*Club, []*Box) {
	if watchlistPath != "" {
		if data, err := os.ReadFile(watchlistPath); err == nil && len(data) > 0 {
			return ParseWatchlist(string(data))
		}
	}
	if data, err := os.ReadFile("FLEET_WATCHLIST.md"); err == nil && len(data) > 0 {
		return ParseWatchlist(string(data))
	}
	return DefaultFleetCatalog()
}
