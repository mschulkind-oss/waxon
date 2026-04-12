#!/usr/bin/env node

// Downloads the correct waxon binary for the current platform from GitHub releases.

const https = require("https");
const fs = require("fs");
const path = require("path");
const { execSync } = require("child_process");
const os = require("os");

const VERSION = require("../package.json").version;
const REPO = "mschulkind-oss/waxon";

function getPlatform() {
  const platform = os.platform();
  const arch = os.arch();

  const platformMap = {
    darwin: "darwin",
    linux: "linux",
    win32: "windows",
  };

  const archMap = {
    x64: "amd64",
    arm64: "arm64",
  };

  const goos = platformMap[platform];
  const goarch = archMap[arch];

  if (!goos || !goarch) {
    throw new Error(`Unsupported platform: ${platform}-${arch}`);
  }

  return { goos, goarch, platform };
}

function getDownloadUrl(version, goos, goarch) {
  const ext = goos === "windows" ? "zip" : "tar.gz";
  return `https://github.com/${REPO}/releases/download/v${version}/waxon-${version}-${goos}-${goarch}.${ext}`;
}

function download(url, dest) {
  return new Promise((resolve, reject) => {
    const follow = (url) => {
      https
        .get(url, (res) => {
          if (res.statusCode >= 300 && res.statusCode < 400 && res.headers.location) {
            follow(res.headers.location);
            return;
          }
          if (res.statusCode !== 200) {
            reject(new Error(`Download failed: HTTP ${res.statusCode} for ${url}`));
            return;
          }
          const file = fs.createWriteStream(dest);
          res.pipe(file);
          file.on("finish", () => file.close(resolve));
          file.on("error", reject);
        })
        .on("error", reject);
    };
    follow(url);
  });
}

async function main() {
  const { goos, goarch, platform } = getPlatform();
  const url = getDownloadUrl(VERSION, goos, goarch);
  const binDir = path.join(__dirname, "..", "bin");
  const binName = platform === "win32" ? "waxon.exe" : "waxon";
  const binPath = path.join(binDir, binName);

  // Skip if binary already exists (e.g., pre-packed)
  if (fs.existsSync(binPath)) {
    return;
  }

  fs.mkdirSync(binDir, { recursive: true });

  const tmpDir = fs.mkdtempSync(path.join(os.tmpdir(), "waxon-"));
  const ext = goos === "windows" ? "zip" : "tar.gz";
  const archive = path.join(tmpDir, `waxon.${ext}`);

  console.log(`Downloading waxon v${VERSION} for ${goos}/${goarch}...`);

  try {
    await download(url, archive);

    if (ext === "tar.gz") {
      execSync(`tar -xzf "${archive}" -C "${tmpDir}"`);
    } else {
      // For Windows, use PowerShell to extract
      execSync(
        `powershell -command "Expand-Archive -Path '${archive}' -DestinationPath '${tmpDir}'"`,
      );
    }

    const extracted = path.join(tmpDir, binName);
    fs.copyFileSync(extracted, binPath);
    fs.chmodSync(binPath, 0o755);

    console.log(`Installed waxon v${VERSION} to ${binPath}`);
  } finally {
    fs.rmSync(tmpDir, { recursive: true, force: true });
  }
}

main().catch((err) => {
  console.error(`Failed to install waxon: ${err.message}`);
  console.error("You can download it manually from:");
  console.error(`  https://github.com/${REPO}/releases`);
  process.exit(1);
});
