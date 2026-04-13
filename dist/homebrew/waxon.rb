# Homebrew formula for waxon
# This file is a template — the actual formula lives in the mschulkind-oss/homebrew-tap repo.
# It gets updated automatically by the release workflow.

class Waxon < Formula
  desc "A slide deck toolkit built for the mind meld between human and agent"
  homepage "https://github.com/mschulkind-oss/waxon"
  license "Apache-2.0"
  version "0.0.0" # Updated by CI

  on_macos do
    if Hardware::CPU.arm?
      url "https://github.com/mschulkind-oss/waxon/releases/download/v#{version}/waxon-#{version}-darwin-arm64.tar.gz"
      sha256 "PLACEHOLDER"
    else
      url "https://github.com/mschulkind-oss/waxon/releases/download/v#{version}/waxon-#{version}-darwin-amd64.tar.gz"
      sha256 "PLACEHOLDER"
    end
  end

  on_linux do
    if Hardware::CPU.arm?
      url "https://github.com/mschulkind-oss/waxon/releases/download/v#{version}/waxon-#{version}-linux-arm64.tar.gz"
      sha256 "PLACEHOLDER"
    else
      url "https://github.com/mschulkind-oss/waxon/releases/download/v#{version}/waxon-#{version}-linux-amd64.tar.gz"
      sha256 "PLACEHOLDER"
    end
  end

  def install
    bin.install "waxon"
  end

  test do
    assert_match version.to_s, shell_output("#{bin}/waxon --version")
  end
end
