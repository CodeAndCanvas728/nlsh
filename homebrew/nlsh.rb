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
      sha256 "e90decd5dfdb5cf46e7b9e9d5b43dc2de7d70444fc495eca280817a24bb01c56"
    end

    on_intel do
      url "https://github.com/CodeAndCanvas728/nlsh/releases/download/v#{version}/nlsh-darwin-amd64.tar.gz"
      sha256 "19c9b10e2a70043038c8104bafbf9f4535fbf1a4229f89a5a22e2492c1ecf879"
    end
  end

  on_linux do
    on_arm do
      url "https://github.com/CodeAndCanvas728/nlsh/releases/download/v#{version}/nlsh-linux-arm64.tar.gz"
      sha256 "229ff1f09e14b00900985195a6130c669ad7548243c193deedfd6ea5633aa77c"
    end

    on_intel do
      url "https://github.com/CodeAndCanvas728/nlsh/releases/download/v#{version}/nlsh-linux-amd64.tar.gz"
      sha256 "f573b16707c1325ef3ff019f97b9095452b352d3be988b9a20eccd09382b4120"
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
