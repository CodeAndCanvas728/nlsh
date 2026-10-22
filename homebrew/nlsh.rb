class Nlsh < Formula
  desc "Natural-language shell: describe what you want, it runs the commands"
  homepage "https://github.com/CodeAndCanvas728/nlsh"
  license "MIT"
  # Bump the version, the checksums and the tag together at every release.
  # The binaries are published by the GitHub workflow, reached through the
  # push mirror from the development forge. The archives are reproducible, so
  # rebuilding the same source gives the same checksums.
  version "0.3.1"

  on_macos do
    on_arm do
      url "https://github.com/CodeAndCanvas728/nlsh/releases/download/v#{version}/nlsh-darwin-arm64.tar.gz"
      sha256 "feae64022f49289b8c58c3257af0c55f9b197b77c06cf4f60d99b35998db8451"
    end

    on_intel do
      url "https://github.com/CodeAndCanvas728/nlsh/releases/download/v#{version}/nlsh-darwin-amd64.tar.gz"
      sha256 "1d9f6ffe2bba685e76b6677188a19c1e4a37a06ef323befcab3ef511bf7ceb3b"
    end
  end

  on_linux do
    on_arm do
      url "https://github.com/CodeAndCanvas728/nlsh/releases/download/v#{version}/nlsh-linux-arm64.tar.gz"
      sha256 "9efcc9c707555e73fb64095baf8182b5b45fa7a7866af94c78eca74f61531bbf"
    end

    on_intel do
      url "https://github.com/CodeAndCanvas728/nlsh/releases/download/v#{version}/nlsh-linux-amd64.tar.gz"
      sha256 "03c7fc5f8c50f8b949745632efb71f7d4a93f49dc7e565cf7bc92a59c78915ce"
    end
  end

  def install
    bin.install "nlsh"
  end

  test do
    # The binary reports the same version the formula pins, so a stale
    # artefact fails the test rather than passing it.
    assert_equal version.to_s, shell_output("#{bin}/nlsh --version").split.last
  end
end
