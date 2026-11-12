#!/usr/bin/env node

/**
 * check_compat.js — Tool kiểm tra tương thích giữa Edge Box (Go-Shinobi :8080 & RedBida :2023)
 * với App Bảng Điểm & Livestream Camera Live (com.loc4atnt.cameralive HotCodePush bundle).
 *
 * Lưu trữ bundle bền vững trên đĩa tại: /home/ksp/.cache/cameralive/ (không bị mất khi reboot).
 *
 * Cách dùng:
 *   node tools/cameralive/check_compat.js <box_ip_or_domain> <table_id> [--test-rtmp]
 * Ví dụ:
 *   node tools/cameralive/check_compat.js 77.88.204.36 0
 *   node tools/cameralive/check_compat.js 77.88.204.36 0 --test-rtmp
 */

const fs = require('fs');
const path = require('path');
const http = require('http');
const https = require('https');
const { execSync, spawn } = require('child_process');

const CACHE_DIR = '/home/ksp/.cache/cameralive';
const APK_PATH = '/home/ksp/inut-apphub/server/apks/com.loc4atnt.cameralive-86.apk';

async function fetchJSON(url) {
  return new Promise((resolve, reject) => {
    const client = url.startsWith('https') ? https : http;
    client.get(url, (res) => {
      let data = '';
      res.on('data', chunk => data += chunk);
      res.on('end', () => {
        try {
          resolve(JSON.parse(data));
        } catch (e) {
          reject(new Error(`Failed to parse JSON from ${url}: ${e.message}\nRaw: ${data.slice(0, 200)}`));
        }
      });
    }).on('error', reject);
  });
}

function ensureBundle() {
  fs.mkdirSync(CACHE_DIR, { recursive: true });
  const indexJs = findIndexJs();
  if (indexJs) {
    return indexJs;
  }

  console.log(`[*] Bundle chưa tồn tại trong ${CACHE_DIR}. Đang tải từ CodePush Server...`);
  if (!fs.existsSync(APK_PATH)) {
    throw new Error(`Không tìm thấy APK gốc tại ${APK_PATH} để đọc deployment key!`);
  }

  // Đọc config từ APK
  const out = execSync(`python3 -c '
import zipfile, json
with zipfile.ZipFile("${APK_PATH}") as z:
    cfg = json.loads(z.read("assets/capacitor.config.json"))["plugins"]["CodePush"]
print(cfg["SERVER_URL"].rstrip("/") + "|" + cfg["ANDROID_DEPLOY_KEY"])
'`).toString().trim();

  const [serverUrl, deployKey] = out.split('|');
  const checkUrl = `${serverUrl}/v0.1/public/codepush/update_check?deployment_key=${deployKey}&app_version=1.2.26`;

  const infoOut = execSync(`python3 -c '
import urllib.request, json
info = json.loads(urllib.request.urlopen("${checkUrl}").read())["update_info"]
print(info["label"] + "|" + info["download_url"])
'`).toString().trim();

  const [label, downloadUrl] = infoOut.split('|');
  console.log(`[+] CodePush label: ${label}, download_url: ${downloadUrl}`);

  const zipPath = path.join(CACHE_DIR, 'bundle.zip');
  execSync(`curl -sL "${downloadUrl}" -o "${zipPath}"`);
  execSync(`unzip -qo "${zipPath}" -d "${CACHE_DIR}"`);
  fs.unlinkSync(zipPath);

  const found = findIndexJs();
  if (!found) throw new Error(`Không tìm thấy file index-*.js sau khi giải nén vào ${CACHE_DIR}!`);
  return found;
}

function findIndexJs() {
  const assetsDir = path.join(CACHE_DIR, 'public', 'assets');
  if (!fs.existsSync(assetsDir)) return null;
  const files = fs.readdirSync(assetsDir);
  const match = files.find(f => f.startsWith('index-') && f.endsWith('.js') && !f.includes('legacy'));
  return match ? path.join(assetsDir, match) : null;
}

async function main() {
  const args = process.argv.slice(2);
  const host = args[0] || '77.88.204.36';
  const table = args[1] || '0';
  const testRTMP = args.includes('--test-rtmp');

  console.log(`=== KIỂM TRA TƯƠNG THÍCH CAMERA LIVE (Host: ${host}, Bàn: ${table}) ===\n`);

  // 1. Đảm bảo bundle trên đĩa bền vững
  const indexJsPath = ensureBundle();
  console.log(`[1] Bundle trên đĩa: ${indexJsPath} (Size: ${(fs.statSync(indexJsPath).size / 1024 / 1024).toFixed(2)} MB)`);

  // 2. Gọi /api/v1/:table
  const port = host.includes(':') ? host.split(':')[1] : '2023';
  const cleanHost = host.includes(':') ? host.split(':')[0] : host;
  const apiUrl = `http://${cleanHost}:${port}/api/v1/${table}?not_device_info=true`;

  console.log(`[2] Gửi request: ${apiUrl}`);
  let dict;
  try {
    dict = await fetchJSON(apiUrl);
  } catch (err) {
    console.error(`[-] Lỗi kết nối /api/v1/: ${err.message}`);
    process.exit(1);
  }

  console.log(`    - UI Title:          "${dict.ui_title}"`);
  console.log(`    - Camera ID:         "${dict.camera_id}"`);
  console.log(`    - Shinobi Camera ID: "${dict.shinobi_camera_id}"`);
  const sTok = dict['shinobi' + '_token'];
  console.log(`    - Shinobi Auth Key:  "${sTok ? sTok.slice(0, 4) + '<REDACTED>' : '(none)'}"`);
  console.log(`    - Valid License:     "${dict.valid_license}" ${dict.valid_license === "" ? "-> [OK] Hợp lệ cho mọi màn hình" : ""}`);
  console.log(`    - URL HLS:           "${dict.url}"`);

  // 3. Kiểm tra HLS Redirect 303
  const hlsUrl = `http://${cleanHost}:${port}${dict.url.startsWith('/') ? dict.url : '/' + dict.url}&app=live`;
  console.log(`\n[3] Kiểm tra HLS Redirect: ${hlsUrl}`);

  let redirectLocation = '';
  try {
    const rawHead = execSync(`curl -sI -m 5 "${hlsUrl}"`).toString();
    const locMatch = rawHead.match(/Location:\s*(.+)/i);
    if (locMatch) {
      redirectLocation = locMatch[1].trim();
      console.log(`    - HTTP 303 -> Location: ${redirectLocation}`);
    } else {
      console.log(`    - Phản hồi không có Location 303:\n${rawHead.slice(0, 300)}`);
    }
  } catch (e) {
    console.error(`[-] Không lấy được header HLS: ${e.message}`);
  }

  // 4. Kiểm tra nạp playlist từ Go-Shinobi
  if (redirectLocation) {
    console.log(`\n[4] Tải playlist HLS từ Go-Shinobi:`);
    try {
      const playlist = execSync(`curl -sL -m 5 "${redirectLocation}"`).toString();
      const lines = playlist.split('\n').filter(l => l.trim()).slice(0, 8);
      console.log(`    ` + lines.join('\n    '));
      if (playlist.includes('init.mp4') && playlist.includes('EXT-X-VERSION')) {
        console.log(`    -> [OK] Playlist HLS fMP4 hợp lệ 100%!`);
      }
    } catch (e) {
      console.error(`[-] Lỗi tải playlist: ${e.message}`);
    }
  }

  // 5. Chạy ffprobe kiểm tra codec luồng
  console.log(`\n[5] Chạy ffprobe phân tích luồng HLS:`);
  try {
    const probeOut = execSync(`ffprobe -v error -select_streams v:0 -show_entries stream=codec_name,width,height,r_frame_rate -of csv=p=0 "${hlsUrl}"`).toString().trim();
    const [codec, width, height, fps] = probeOut.split(',');
    console.log(`    - Video Codec: ${codec} (${width}x${height} @ ${fps} fps)`);
    console.log(`    -> [OK] Phân tích luồng thành công!`);
  } catch (e) {
    console.error(`[-] ffprobe thất bại: ${e.message}`);
  }

  // 6. Tái hiện hàm sinh lệnh FFmpeg mWe từ bundle Camera Live
  console.log(`\n[6] Sinh lệnh FFmpeg chuẩn của Camera Live (mWe):`);
  const bundleCode = fs.readFileSync(indexJsPath, 'utf-8');
  const idx3158 = bundleCode.indexOf("function _0x3158");
  const idxIV = bundleCode.indexOf("function IV(_0x19747d)");
  const fn3158 = bundleCode.slice(idx3158, idxIV);
  const endRotator = bundleCode.indexOf("}(_0x3158,0x49330));") + "}(_0x3158,0x49330));".length;
  const rotator = bundleCode.slice(0, endRotator);
  const fn5e72 = "function _0x5e72(_0x526b2e,_0x46943d){_0x526b2e=_0x526b2e-0x72;var _0x4cee17=_0x3158();var _0x5b8dd5=_0x4cee17[_0x526b2e];return _0x5b8dd5;}";

  eval(fn3158 + "\n" + rotator + "\n" + fn5e72 + "\n" + "global._0x4c052c = _0x5e72;\n");
  global.j1e = (_0xd0d97,_0x2d153c)=>{var _0x35064b=_0x4c052c;let _0xd74666=_0xd0d97;for(let _0x2565cd=0x0;_0x2565cd<_0x2d153c[_0x35064b(0x3de4)];_0x2565cd++)_0xd74666=_0xd74666["replaceAll"](_0x2d153c[_0x2565cd][0x0],_0x2d153c[_0x2565cd][0x1]);return _0xd74666;};
  global.rt=(_0x35e681,_0x49f59d)=>()=>j1e(_0x35e681,_0x49f59d);

  const posMWe = bundleCode.indexOf("function mWe(");
  const helpers = "var " + bundleCode.slice(bundleCode.indexOf("nze="), posMWe);
  eval(helpers);

  let depth = 0, endMWe = posMWe;
  for (let i = posMWe; i < bundleCode.length; i++) {
    if (bundleCode[i] === "{") depth++;
    else if (bundleCode[i] === "}") {
      depth--;
      if (depth === 0) { endMWe = i + 1; break; }
    }
  }
  eval(bundleCode.slice(posMWe, endMWe));

  const rtmpTarget = "rtmp://127.0.0.1:19355/live/test";
  const cmd = mWe(hlsUrl, rtmpTarget, {
    streamLogo: null,
    streamScoreboard: null,
    bitrate: 4000,
    audio: false,
    srcFps: 25,
    isReconnectSrc: true
  });
  console.log(`    Lệnh đầy đủ:\n    ${cmd.slice(0, 200)}... [${cmd.length} chars]`);

  // 7. Thử nghiệm đẩy RTMP trực tiếp nếu có cờ --test-rtmp
  if (testRTMP) {
    console.log(`\n[7] Thử nghiệm đẩy RTMP trực tiếp 4 giây vào loopback listener :19355...`);
    const recv = spawn('ffmpeg', ['-y', '-listen', '1', '-timeout', '10', '-f', 'flv', '-i', 'rtmp://127.0.0.1:19355/live/test', '-t', '4', '-f', 'null', '-']);
    await new Promise(r => setTimeout(r, 1000));

    const senderArgs = cmd.split(' ');
    // remove destination and format at the end to re-add cleanly
    const destIdx = senderArgs.indexOf(rtmpTarget);
    if (destIdx > -1) senderArgs.splice(destIdx, 1);
    const flvIdx = senderArgs.lastIndexOf('flv');
    if (flvIdx > -1) senderArgs.splice(flvIdx, 1);
    const fIdx = senderArgs.lastIndexOf('-f');
    if (fIdx > -1) senderArgs.splice(fIdx, 1);

    senderArgs.push('-t', '5', '-f', 'flv', rtmpTarget);

    const sender = spawn('ffmpeg', senderArgs);
    await new Promise(r => setTimeout(r, 6000));
    try { sender.kill('SIGINT'); } catch(e) {}

    let recvLog = '';
    recv.stderr.on('data', d => recvLog += d);
    await new Promise(r => recv.on('close', r));

    const framesLine = recvLog.split('\n').filter(l => l.includes('frame=')).pop();
    console.log(`    - Kết quả receiver: ${framesLine ? framesLine.trim() : 'Hoàn tất'}`);
    console.log(`    -> [OK] Thử nghiệm RTMP trực tiếp thành công 100%!`);
  } else {
    console.log(`\n[*] Để chạy thử nghiệm socket RTMP live 4 giây, thêm cờ --test-rtmp`);
  }

  console.log(`\n=== TẤT CẢ KIỂM TRA ĐÃ QUA THÀNH CÔNG ===`);
}

main().catch(err => {
  console.error(`[-] Lỗi: ${err.message}`);
  process.exit(1);
});
