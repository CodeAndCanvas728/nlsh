class Nlsh < Formula
  desc "Natural-language shell: describe what you want, it runs the commands"
  homepage "https://github.com/CodeAndCanvas728/nlsh"
  license "MIT"
  # Bump the version, the checksums and the tag together at every release.
  # The binaries are published by the GitHub workflow, reached through the
  # push mirror from the development forge. The archives are reproducible, so
  # rebuilding the same source gives the same checksums.
  version "0.3.0"

  on_macos do
    on_arm do
      url "https://github.com/CodeAndCanvas728/nlsh/releases/download/v#{version}/nlsh-darwin-arm64.tar.gz"
      sha256 "afd219226a232e9f190fc5baf20f6a41728e242e5af34e770aa46729e93a0228"
    end

    on_intel do
      url "https://github.com/CodeAndCanvas728/nlsh/releases/download/v#{version}/nlsh-darwin-amd64.tar.gz"
      sha256 "4e7b0698995b9c0a990e75c0d8136440a4a1b97ce427c6a96dfc68dc82d871d9"
    end
  end

  on_linux do
    on_arm do
      url "https://github.com/CodeAndCanvas728/nlsh/releases/download/v#{version}/nlsh-linux-arm64.tar.gz"
      sha256 "d9f2e907171b76e0e8e6c5307626257d8c32451907c6b4cfd7607ad1e8852509"
    end

    on_intel do
      url "https://github.com/CodeAndCanvas728/nlsh/releases/download/v#{version}/nlsh-linux-amd64.tar.gz"
      sha256 "2be182d3c37559865d156dd08d81668d641b058ffc36a6558899650872dc638c"
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
