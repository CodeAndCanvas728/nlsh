class Nlsh < Formula
  desc "Natural-language shell: describe what you want, it runs the commands"
  homepage "https://github.com/jonah/nlsh"
  url "https://github.com/jonah/nlsh/releases/download/v#{version}/nlsh-darwin-arm64.tar.gz"
  version "0.3.0"
  license "MIT"

  def install
    bin.install "nlsh"
  end

  test do
    assert_match version.to_s, shell_output("#{bin}/nlsh --version")
  end
end
